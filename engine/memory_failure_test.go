package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCompactionReplaceFailureRecoversWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	id, err := m.SubmitFeedback("w", "u", "uncertain", "", "survivor", true)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.compactJournalWithReplace(func(string, string) error { return errors.New("injected rename failure") })
	if err == nil {
		t.Fatal("injected rename error not surfaced")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("original file mutated on failed replacement")
	}
	next, err := m.SubmitFeedback("w", "u", "helpful", "", "still writable", true)
	if err != nil || next <= id {
		t.Fatalf("cannot append after recovered writer: %v %d", err, next)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if opened.FeedbackSummary("w", "u")["helpful"] != 1 {
		t.Fatal("lost post-failure record")
	}
}
func TestJournalDisconnectionDoesNotSilentlySucceed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	// Force the same structural state as a failed reopen, with all operations
	// serialized by the journal mutex. No external OS failure injection needed.
	m.mu.Lock()
	m.f.Close()
	m.f = nil
	m.mu.Unlock()
	_, err = m.SubmitFeedback("w", "u", "uncertain", "", "unsafe", true)
	if err == nil {
		t.Fatal("append pretended success after disconnected storage")
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("owner marker survived close: %v", err)
	}
}
