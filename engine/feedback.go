package engine

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Feedback categories distinguish disagreement, uncertainty and missing
// evidence. They are intentionally not silently mapped to a model-training
// label or added as an approved wording example.
func validFeedbackCategory(category string) bool {
	switch category {
	case "misheard", "wrong_intent", "rejected_suggestion", "insufficient_evidence", "uncertain", "helpful":
		return true
	}
	return false
}

func (m *Memory) SubmitFeedback(workspace, userID, category, reference, note string, consent bool) (int64, error) {
	if !consent {
		return 0, errors.New("explicit consent required")
	}
	if strings.TrimSpace(workspace) == "" || strings.TrimSpace(userID) == "" {
		return 0, errors.New("workspace and user required")
	}
	if len(workspace) > 128 || len(userID) > 128 || !validFeedbackCategory(category) ||
		len(reference) > 128 || utf8.RuneCountInString(note) > 512 || len(note) > 2048 {
		return 0, errors.New("invalid feedback fields")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	feedback := UserFeedback{ID: m.next, Workspace: workspace, UserID: userID, Category: category,
		Reference: reference, Note: note, CreatedAt: time.Now().Unix()}
	if err := m.appendRecord(record{Kind: "feedback", Feedback: &feedback}); err != nil {
		return 0, fmt.Errorf("persist feedback: %w", err)
	}
	m.feedback[feedback.ID] = feedback
	m.next++
	return feedback.ID, nil
}

// FeedbackSummary intentionally returns only aggregate categories for the
// caller-provided scope. Research API stays loopback-only and unauthenticated.
func (m *Memory) FeedbackSummary(workspace, userID string) map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	counts := map[string]int{}
	for _, x := range m.feedback {
		if x.Workspace == workspace && x.UserID == userID {
			counts[x.Category]++
		}
	}
	return counts
}

// DeleteFeedback appends a tombstone before changing in-memory state. It
// takes effect on subsequent restarts, without rewriting unrelated data.
// A log retains old bytes; for secure erasure the journal file must be
// compacted or deleted, which is NOT implemented in this research version.
func (m *Memory) DeleteFeedback(workspace, userID string, id int64) error {
	if id <= 0 || workspace == "" || userID == "" {
		return errors.New("invalid delete request")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.feedback[id]
	if !ok || x.Workspace != workspace || x.UserID != userID {
		return errors.New("feedback not found in scope")
	}
	item := FeedbackDelete{Workspace: workspace, UserID: userID, ID: id}
	if err := m.appendRecord(record{Kind: "feedback_delete", FeedbackDelete: &item}); err != nil {
		return err
	}
	delete(m.feedback, id)
	return nil
}
