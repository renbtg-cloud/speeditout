package engine

import (
	"path/filepath"
	"testing"
)

func TestJournalStatsBeforeAfterCompaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	first, err := m.SubmitFeedback("w", "u", "helpful", "", "private sample", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.DeleteFeedback("w", "u", first); err != nil {
		t.Fatal(err)
	}
	x, err := m.JournalStats()
	if err != nil {
		t.Fatal(err)
	}
	if x.ActiveFeedback != 0 || x.BeforeBytes <= 0 {
		t.Fatalf("bad stats: %+v", x)
	}
	y, err := m.CompactJournal()
	if err != nil {
		t.Fatal(err)
	}
	if y.AfterBytes >= x.BeforeBytes {
		t.Fatalf("not compacted: %+v", y)
	}
	z, err := m.JournalStats()
	if err != nil {
		t.Fatal(err)
	}
	if z.BeforeBytes != y.AfterBytes {
		t.Fatalf("mismatch: %+v %+v", z, y)
	}
}
