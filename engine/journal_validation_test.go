package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnknownOrAmbiguousJournalRecordCannotBeCompactedAway(t *testing.T) {
	bad := []string{
		`{"kind":"future","opaque":"secret"}`,
		`{"kind":"counter","next_id":2,"future_key":42}`,
		`{"kind":"feedback","feedback":null}`,
		`{"kind":"feedback_delete","feedback_delete":{"workspace":"w","user_id":"u","id":3},"example":{"id":2}}`,
		`{"kind":"pref"}`,
		`{"kind":"counter","next_id":0}`,
		`{"kind":"counter","next_id":1}{"kind":"counter","next_id":2}`,
	}
	for _, entry := range bad {
		t.Run(entry, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "journal.jsonl")
			if err := os.WriteFile(path, []byte(entry+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewMemory(path); err == nil {
				t.Fatalf("accepted unknown journal data: %s", entry)
			}
			if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
				t.Fatalf("leaked lock on invalid journal %v", err)
			}
		})
	}
}
func TestJournalValidationAcceptsKnownLegacyCorrection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	data := strings.Join([]string{
		`{"kind":"correction","example":{"id":1,"workspace":"w","user_id":"u","recipient":"boss","raw_text":"call","approved_text":"call next week","note":"x","created_at":0},"preference":{"Workspace":"w","UserID":"u","Recipient":"boss","Key":"greeting","Value":"omit","UpdatedAt":0}}`,
		`{"kind":"counter","next_id":9}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if m.Preference("w", "u", "greeting", "boss") != "omit" {
		t.Fatal("legacy correction lost")
	}
	if _, err := m.CompactJournal(); err != nil {
		t.Fatal(err)
	}
	id, err := m.SubmitFeedback("w", "u", "helpful", "", "", true)
	if err != nil || id != 9 {
		t.Fatalf("counter not preserved id=%d err=%v", id, err)
	}
}
