package engine

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFeedbackConsentPersistenceAndTombstones(t *testing.T) {
	file := filepath.Join(t.TempDir(), "memory.jsonl")
	m, err := NewMemory(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.SubmitFeedback("w", "alice", "wrong_intent", "req-1", "", false); err == nil {
		t.Fatal("nonconsensual feedback saved")
	}
	if _, err = m.SubmitFeedback("w", "alice", "imagined_label", "req-2", "", true); err == nil {
		t.Fatal("unknown category saved")
	}
	if _, err = m.SubmitFeedback("w", "alice", "uncertain", "req-3", strings.Repeat("a", 513), true); err == nil {
		t.Fatal("oversized note accepted")
	}
	id, err := m.SubmitFeedback("w", "alice", "insufficient_evidence", "local-id-123", "This claim lacked evidence", true)
	if err != nil {
		t.Fatal(err)
	}
	other, err := m.SubmitFeedback("w", "bob", "rejected_suggestion", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if m.FeedbackSummary("w", "alice")["insufficient_evidence"] != 1 || m.FeedbackSummary("w", "alice")["rejected_suggestion"] != 0 {
		t.Fatal("scope leaked")
	}
	if err := m.DeleteFeedback("w", "bob", id); err == nil {
		t.Fatal("cross-user removal allowed")
	}
	if err := m.DeleteFeedback("w", "alice", other); err == nil {
		t.Fatal("cross-user deletion allowed")
	}
	if err := m.DeleteFeedback("w", "alice", id); err != nil {
		t.Fatal(err)
	}
	if m.FeedbackSummary("w", "alice")["insufficient_evidence"] != 0 {
		t.Fatal("still active")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m, err = NewMemory(file)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if len(m.FeedbackSummary("w", "alice")) != 0 || m.FeedbackSummary("w", "bob")["rejected_suggestion"] != 1 {
		t.Fatalf("recovery mismatch: %#v %#v", m.FeedbackSummary("w", "alice"), m.FeedbackSummary("w", "bob"))
	}
	next, err := m.SubmitFeedback("w", "alice", "misheard", "", "", true)
	if err != nil || next <= other {
		t.Fatalf("reused id: %d, %v", next, err)
	}
	if got := m.SimilarExamples("w", "alice", "This claim lacked evidence", "", 5); len(got) != 0 {
		t.Fatalf("feedback was auto-learned as approved example: %+v", got)
	}
}

func TestFeedbackConcurrentScopedCounts(t *testing.T) {
	m, err := NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	var wg sync.WaitGroup
	const workers = 12
	const each = 60
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				_, err := m.SubmitFeedback("w", "shared", "uncertain", "", "", true)
				if err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if got := m.FeedbackSummary("w", "shared")["uncertain"]; got != workers*each {
		t.Fatalf("%d != %d", got, workers*each)
	}
}
