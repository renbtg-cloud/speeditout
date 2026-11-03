package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCompactJournalPurgesLogicalDeletionsPreservesState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.SetPreference("w", "u", "tone", "direct", "boss", true); err != nil {
		t.Fatal(err)
	}
	exampleID, err := m.AddExample("w", "u", "começar amanhã", "volto amanhã", "boss", "approved", true)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := m.SubmitFeedback("w", "u", "rejected_suggestion", "ref-deleted", "PRIVATE_NOTE_TO_REMOVE", true)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := m.SubmitFeedback("w", "u", "uncertain", "ref-retained", "active note", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.DeleteFeedback("w", "u", removed); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(before, []byte("PRIVATE_NOTE_TO_REMOVE")) {
		t.Fatal("test setup wrong")
	}
	result, err := m.CompactJournal()
	if err != nil {
		t.Fatal(err)
	}
	if result.Preferences != 1 || result.Examples != 1 || result.ActiveFeedback != 1 {
		t.Fatalf("count mismatch: %+v", result)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte("PRIVATE_NOTE_TO_REMOVE")) || bytes.Contains(after, []byte("ref-deleted")) {
		t.Fatal("tombstoned plaintext in active journal")
	}
	if !bytes.Contains(after, []byte("active note")) || result.AfterBytes >= result.BeforeBytes {
		t.Fatalf("unexpected compaction: %+v", result)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	m, err = NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if got := m.Preference("w", "u", "tone", "boss"); got != "direct" {
		t.Fatalf("preference lost: %q", got)
	}
	if got := m.SimilarExamples("w", "u", "começar amanhã", "boss", 10); len(got) != 1 || got[0].ID != exampleID {
		t.Fatalf("example lost %+v", got)
	}
	if m.FeedbackSummary("w", "u")["rejected_suggestion"] != 0 || m.FeedbackSummary("w", "u")["uncertain"] != 1 {
		t.Fatal("feedback scope wrong")
	}
	next, err := m.SubmitFeedback("w", "u", "misheard", "", "", true)
	if err != nil || next <= retained {
		t.Fatalf("sequence reused next=%d last=%d err=%v", next, retained, err)
	}
}

func TestCompactJournalConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if _, err := m.SubmitFeedback("w", "u", "uncertain", "", "", true); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for i := 0; i < 4; i++ {
		if _, err := m.CompactJournal(); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	if count := m.FeedbackSummary("w", "u")["uncertain"]; count != 400 {
		t.Fatalf("lost concurrent records: %d", count)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	m, err = NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if count := m.FeedbackSummary("w", "u")["uncertain"]; count != 400 {
		t.Fatalf("restart lost records: %d", count)
	}
}

func TestCompactJournalPreconditions(t *testing.T) {
	m, _ := NewMemory("")
	defer m.Close()
	if _, err := m.CompactJournal(); err == nil {
		t.Fatal("in-memory compaction should reject")
	}
}
