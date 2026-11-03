package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalExclusiveOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	first, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewMemory(path); err == nil {
		t.Fatal("second writer accepted")
	}
	if _, err := first.SubmitFeedback("lab", "u", "uncertain", "", "", true); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.FeedbackSummary("lab", "u")["uncertain"] != 1 {
		t.Fatal("journal not recovered")
	}
}

func TestStaleJournalLockRequiresOperatorAction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	if err := os.WriteFile(path+".lock", []byte("pid=999999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMemory(path); err == nil || !strings.Contains(err.Error(), "stale lock") {
		t.Fatalf("stale owner silently stolen: %v", err)
	}
	if err := os.Remove(path + ".lock"); err != nil {
		t.Fatal(err)
	}
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptJournalReleasesOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	if err := os.WriteFile(path, []byte("not JSON\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMemory(path); err == nil {
		t.Fatal("corruption accepted")
	}
	if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("lock leaked: %v", err)
	}
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
}

func TestCompactJournalOwnerCannotBeStolen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for i := 0; i < 20; i++ {
		if _, err = m.SubmitFeedback("w", "u", "uncertain", "", "", true); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err = m.CompactJournal(); err != nil {
			t.Fatal(err)
		}
		if _, err = NewMemory(path); err == nil {
			t.Fatal("exclusive ownership lost after compact")
		}
	}
	if got := m.FeedbackSummary("w", "u")["uncertain"]; got != 20 {
		t.Fatalf("lost entries %d", got)
	}
}
