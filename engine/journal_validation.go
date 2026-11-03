package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// decodeJournalRecord is strict because compaction must never silently drop
// unknown future journal records that the current binary cannot understand.
// A corrupt/unknown record halts startup; it is not silently skipped.
func decodeJournalRecord(line []byte) (record, error) {
	var r record
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return r, err
	}
	var trailing json.RawMessage
	if err := d.Decode(&trailing); err != io.EOF {
		return r, errors.New("more than one JSON value in journal entry")
	}
	switch r.Kind {
	case "counter":
		if r.NextID == nil || *r.NextID < 1 || r.Preference != nil || r.Example != nil || r.Feedback != nil || r.FeedbackDelete != nil {
			return r, errors.New("invalid counter entry")
		}
	case "pref":
		if r.Preference == nil || r.Example != nil || r.Feedback != nil || r.FeedbackDelete != nil || r.NextID != nil {
			return r, errors.New("invalid preference entry")
		}
	case "example":
		if r.Example == nil || r.Preference != nil || r.Feedback != nil || r.FeedbackDelete != nil || r.NextID != nil {
			return r, errors.New("invalid example entry")
		}
	case "correction":
		if r.Example == nil || r.Feedback != nil || r.FeedbackDelete != nil || r.NextID != nil {
			return r, errors.New("invalid correction entry")
		}
	case "feedback":
		if r.Feedback == nil || r.Example != nil || r.Preference != nil || r.FeedbackDelete != nil || r.NextID != nil {
			return r, errors.New("invalid feedback entry")
		}
	case "feedback_delete":
		if r.FeedbackDelete == nil || r.Example != nil || r.Preference != nil || r.Feedback != nil || r.NextID != nil {
			return r, errors.New("invalid feedback deletion entry")
		}
	default:
		return r, fmt.Errorf("unknown journal entry kind %q", r.Kind)
	}
	return r, nil
}
