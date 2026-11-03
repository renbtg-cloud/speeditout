package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// JournalCompaction is a filesystem operation, not secure erasure.
// Deleted entries disappear from the ACTIVE file after atomic replacement;
// filesystem snapshots, backups, wear leveling and old disk sectors are not
// under this application's control.
type JournalCompaction struct {
	BeforeBytes    int64 `json:"before_bytes"`
	AfterBytes     int64 `json:"after_bytes"`
	Preferences    int   `json:"preferences"`
	Examples       int   `json:"examples"`
	ActiveFeedback int   `json:"active_feedback"`
}

// JournalStats reports counts without rewriting. The byte count includes
// tombstoned entries because they remain in the un-compacted active journal.
func (m *Memory) JournalStats() (JournalCompaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.path == "" || m.f == nil {
		return JournalCompaction{}, errors.New("file-backed journal required")
	}
	stat, err := m.f.Stat()
	if err != nil {
		return JournalCompaction{}, err
	}
	return JournalCompaction{BeforeBytes: stat.Size(), Preferences: len(m.prefs), Examples: len(m.examples), ActiveFeedback: len(m.feedback)}, nil
}

func (m *Memory) CompactJournal() (JournalCompaction, error) {
	return m.compactJournalWithReplace(os.Rename)
}

// Accepting the replacement function internally allows deterministic
// filesystem-failure tests without modifying process-global state.
func (m *Memory) compactJournalWithReplace(replace func(string, string) error) (JournalCompaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result JournalCompaction
	if m.path == "" || m.f == nil {
		return result, errors.New("compaction requires a file-backed memory journal")
	}
	info, err := m.f.Stat()
	if err != nil {
		return result, err
	}
	result.BeforeBytes = info.Size()
	result.Preferences = len(m.prefs)
	result.Examples = len(m.examples)
	result.ActiveFeedback = len(m.feedback)
	temp, err := os.CreateTemp(filepath.Dir(m.path), ".speeditout-compaction-*.tmp")
	if err != nil {
		return result, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err = temp.Chmod(0600); err != nil {
		temp.Close()
		return result, err
	}
	enc := json.NewEncoder(temp)
	write := func(r record) error { return enc.Encode(r) }
	nextID := m.next
	if err = write(record{Kind: "counter", NextID: &nextID}); err != nil {
		temp.Close()
		return result, err
	}
	keys := make([]string, 0, len(m.prefs))
	for key := range m.prefs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		if len(parts) != 4 {
			temp.Close()
			return result, fmt.Errorf("invalid preference key")
		}
		pref := Preference{Workspace: parts[0], UserID: parts[1], Recipient: parts[2], Key: parts[3], Value: m.prefs[key]}
		if err = write(record{Kind: "pref", Preference: &pref}); err != nil {
			temp.Close()
			return result, err
		}
	}
	examples := append([]Example(nil), m.examples...)
	sort.Slice(examples, func(i, j int) bool { return examples[i].ID < examples[j].ID })
	for _, example := range examples {
		ex := example
		if err = write(record{Kind: "example", Example: &ex}); err != nil {
			temp.Close()
			return result, err
		}
	}
	ids := make([]int64, 0, len(m.feedback))
	for id := range m.feedback {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		feedback := m.feedback[id]
		if err = write(record{Kind: "feedback", Feedback: &feedback}); err != nil {
			temp.Close()
			return result, err
		}
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return result, err
	}
	stat, err := temp.Stat()
	if err != nil {
		temp.Close()
		return result, err
	}
	result.AfterBytes = stat.Size()
	if err = temp.Close(); err != nil {
		return result, err
	}
	if err = m.f.Sync(); err != nil {
		return result, err
	}
	// Windows does not allow replacing an open target. Keep the mutex held
	// so no writers can append during close/rename/reopen.
	if err = m.f.Close(); err != nil {
		return result, err
	}
	m.f = nil
	if err = replace(tempName, m.path); err != nil {
		reopened, reopenErr := os.OpenFile(m.path, os.O_RDWR|os.O_APPEND, 0600)
		if reopenErr == nil {
			m.f = reopened
		}
		return JournalCompaction{}, fmt.Errorf("atomic journal replacement failed: %w (reopen: %v)", err, reopenErr)
	}
	m.f, err = os.OpenFile(m.path, os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return result, fmt.Errorf("reopen compacted journal: %w", err)
	}
	// Directory durability varies across platforms; syncing here is best
	// effort, with the replacement itself already completed atomically.
	if dir, e := os.Open(filepath.Dir(m.path)); e == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return result, nil
}
