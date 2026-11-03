package engine

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Preference struct {
	Workspace, UserID, Recipient, Key, Value string
	UpdatedAt                                int64
}
type Example struct {
	ID           int64  `json:"id"`
	Workspace    string `json:"workspace"`
	UserID       string `json:"user_id"`
	Recipient    string `json:"recipient"`
	RawText      string `json:"raw_text"`
	ApprovedText string `json:"approved_text"`
	Note         string `json:"note"`
	CreatedAt    int64  `json:"created_at"`
}

// UserFeedback is a consented, user-authored signal, not a proven model error.
// Neither free-text notes nor per-user feedback are returned in public metrics.
type UserFeedback struct {
	ID        int64  `json:"id"`
	Workspace string `json:"workspace"`
	UserID    string `json:"user_id"`
	Category  string `json:"category"`
	Reference string `json:"reference,omitempty"`
	Note      string `json:"note,omitempty"`
	CreatedAt int64  `json:"created_at"`
}
type FeedbackDelete struct {
	Workspace string `json:"workspace"`
	UserID    string `json:"user_id"`
	ID        int64  `json:"id"`
}
type Similar struct {
	ID                            int64
	Score                         float64
	Recipient, ApprovedText, Note string
}
type record struct {
	Kind           string          `json:"kind"`
	Preference     *Preference     `json:"preference,omitempty"`
	Example        *Example        `json:"example,omitempty"`
	Feedback       *UserFeedback   `json:"feedback,omitempty"`
	FeedbackDelete *FeedbackDelete `json:"feedback_delete,omitempty"`
	NextID         *int64          `json:"next_id,omitempty"`
}

// Memory is concurrency-safe. Writes are consent-gated, serialized, and fsynced.
// JSONL is a research format, NOT an encrypted/production database.
type Memory struct {
	mu       sync.RWMutex
	prefs    map[string]string
	examples []Example
	feedback map[int64]UserFeedback
	next     int64
	f        *os.File
	path     string
	lockPath string
}

func prefKey(w, u, r, k string) string { return w + "\x00" + u + "\x00" + r + "\x00" + k }
func NewMemory(path string) (*Memory, error) {
	m := &Memory{prefs: map[string]string{}, feedback: map[int64]UserFeedback{}, next: 1}
	if path == "" {
		return m, nil
	}
	// Refuse multiple processes opening the same journal for writes. This
	// advisory lock is intentionally conservative: a crash leaves a stale
	// .lock that an operator must inspect and remove after confirming no
	// prior process still owns the journal. Never auto-steal stale locks.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	lockPath := absPath + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("journal appears in use (or stale lock): %w", err)
	}
	if _, err = fmt.Fprintf(lock, "pid=%d\n", os.Getpid()); err != nil {
		lock.Close()
		os.Remove(lockPath)
		return nil, err
	}
	if err = lock.Close(); err != nil {
		os.Remove(lockPath)
		return nil, err
	}
	f, err := os.OpenFile(absPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		os.Remove(lockPath)
		return nil, err
	}
	m.f = f
	m.path = absPath
	m.lockPath = lockPath
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 8<<20)
	for scan.Scan() {
		rec, err := decodeJournalRecord(scan.Bytes())
		if err != nil {
			f.Close()
			os.Remove(lockPath)
			return nil, fmt.Errorf("memory log invalid: %w", err)
		}
		if (rec.Kind == "pref" || rec.Kind == "correction") && rec.Preference != nil {
			p := rec.Preference
			m.prefs[prefKey(p.Workspace, p.UserID, p.Recipient, p.Key)] = p.Value
		}
		if rec.Kind == "counter" && rec.NextID != nil {
			if *rec.NextID < 1 {
				f.Close()
				return nil, fmt.Errorf("memory log invalid: counter")
			}
			if *rec.NextID > m.next {
				m.next = *rec.NextID
			}
		}
		if rec.Kind == "feedback" && rec.Feedback != nil {
			x := *rec.Feedback
			if x.ID <= 0 || x.Workspace == "" || x.UserID == "" || !validFeedbackCategory(x.Category) {
				f.Close()
				return nil, fmt.Errorf("memory log invalid: feedback entry")
			}
			m.feedback[x.ID] = x
			if x.ID >= m.next {
				m.next = x.ID + 1
			}
		}
		if rec.Kind == "feedback_delete" && rec.FeedbackDelete != nil {
			d := rec.FeedbackDelete
			if previous, exists := m.feedback[d.ID]; exists && previous.Workspace == d.Workspace && previous.UserID == d.UserID {
				delete(m.feedback, d.ID)
			}
		}
		if (rec.Kind == "example" || rec.Kind == "correction") && rec.Example != nil {
			m.examples = append(m.examples, *rec.Example)
			if rec.Example.ID >= m.next {
				m.next = rec.Example.ID + 1
			}
		}
	}
	if err := scan.Err(); err != nil {
		f.Close()
		os.Remove(lockPath)
		return nil, err
	}
	return m, nil
}
func (m *Memory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var err error
	if m.f != nil {
		err = m.f.Close()
		m.f = nil
	}
	if m.lockPath != "" {
		if e := os.Remove(m.lockPath); e != nil && err == nil {
			err = e
		}
		m.lockPath = ""
	}
	return err
}
func (m *Memory) appendRecord(rec record) error {
	if m.f == nil {
		if m.path != "" {
			return errors.New("file-backed journal is not writable")
		}
		return nil
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err = m.f.Write(b); err != nil {
		return err
	}
	return m.f.Sync()
}
func (m *Memory) SetPreference(w, u, k, v, r string, consent bool) error {
	if !consent {
		return errors.New("explicit consent required")
	}
	if w == "" || u == "" {
		return errors.New("workspace and user required")
	}
	switch k {
	case "tone":
		if v != "neutral" && v != "casual" && v != "profane" && v != "direct" {
			return errors.New("invalid tone")
		}
	case "greeting":
		if v != "include" && v != "omit" {
			return errors.New("invalid greeting")
		}
	case "avoid_phrase":
	default:
		return errors.New("unsupported preference")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p := Preference{w, u, r, k, v, time.Now().Unix()}
	if err := m.appendRecord(record{Kind: "pref", Preference: &p}); err != nil {
		return err
	}
	m.prefs[prefKey(w, u, r, k)] = v
	return nil
}
func (m *Memory) Preference(w, u, k, r string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if r != "" {
		if v, ok := m.prefs[prefKey(w, u, r, k)]; ok {
			return v
		}
	}
	return m.prefs[prefKey(w, u, "", k)]
}
func (m *Memory) AddExample(w, u, raw, approved, r, note string, consent bool) (int64, error) {
	if !consent {
		return 0, errors.New("explicit consent required")
	}
	if w == "" || u == "" || strings.TrimSpace(raw) == "" || strings.TrimSpace(approved) == "" {
		return 0, errors.New("missing required fields")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ex := Example{m.next, w, u, r, raw, approved, note, time.Now().Unix()}
	if err := m.appendRecord(record{Kind: "example", Example: &ex}); err != nil {
		return 0, err
	}
	m.examples = append(m.examples, ex)
	m.next++
	return ex.ID, nil
}

// LearnGreetingCorrection validates before writing, then commits both the approved
// example and generalized recipient preference in one serialized journal entry.
func (m *Memory) LearnGreetingCorrection(w, u, utterance, before, after, r string, consent, generalize bool) (int64, error) {
	if !consent {
		return 0, errors.New("explicit consent required")
	}
	if w == "" || u == "" || strings.TrimSpace(utterance) == "" || strings.TrimSpace(after) == "" {
		return 0, errors.New("missing required fields")
	}
	prefix := "Recebi sua mensagem. "
	if generalize && (!strings.HasPrefix(before, prefix) || strings.TrimSpace(after) != strings.TrimSpace(strings.TrimPrefix(before, prefix))) {
		return 0, errors.New("unsupported generalization")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ex := Example{m.next, w, u, r, utterance, after, "Explicitly approved revision", time.Now().Unix()}
	rec := record{Kind: "example", Example: &ex}
	if generalize {
		p := Preference{w, u, r, "greeting", "omit", time.Now().Unix()}
		rec.Kind = "correction"
		rec.Preference = &p
	}
	if err := m.appendRecord(rec); err != nil {
		return 0, err
	}
	m.examples = append(m.examples, ex)
	m.next++
	if rec.Preference != nil {
		m.prefs[prefKey(w, u, r, "greeting")] = rec.Preference.Value
	}
	return ex.ID, nil
}
func words(s string) map[string]bool {
	skip := map[string]bool{"a": true, "o": true, "e": true, "de": true, "da": true, "do": true, "que": true, "em": true, "no": true, "na": true, "um": true, "uma": true, "pra": true, "para": true, "por": true, "eu": true, "ele": true, "ela": true, "voce": true, "vc": true, "me": true, "minha": true, "meu": true, "com": true, "foi": true, "ta": true, "to": true, "vai": true, "nao": true, "so": true, "isso": true, "sobre": true, "como": true, "quando": true, "muito": true}
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(norm(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(w) > 2 && !skip[w] {
			out[w] = true
		}
	}
	return out
}

// charGrams is an in-house, language-independent sparse lexical vector. It is
// NOT a neural semantic embedding: useful for spelling/morphological variation,
// but it cannot reliably understand new meanings or contradictory statements.
func charGrams(text string) map[string]bool {
	out := make(map[string]bool)
	for word := range words(text) {
		rr := []rune("^" + word + "$")
		for i := 0; i+3 <= len(rr); i++ {
			out[string(rr[i:i+3])] = true
		}
	}
	return out
}
func cosineSets(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	common := 0
	for k := range a {
		if b[k] {
			common++
		}
	}
	return float64(common) / math.Sqrt(float64(len(a)*len(b)))
}

func (m *Memory) SimilarExamples(w, u, query, r string, max int) []Similar {
	qt := words(query)
	qgrams := charGrams(query)
	if len(qt) == 0 {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Similar{}
	seen := 0
	for i := len(m.examples) - 1; i >= 0 && seen < 200; i-- {
		ex := m.examples[i]
		if ex.Workspace != w || ex.UserID != u || (ex.Recipient != "" && ex.Recipient != r) {
			continue
		}
		seen++
		et := words(ex.RawText)
		score := 0.70*cosineSets(qt, et) + 0.30*cosineSets(qgrams, charGrams(ex.RawText))
		if score >= 0.35 {
			out = append(out, Similar{ex.ID, score, ex.Recipient, ex.ApprovedText, ex.Note})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Recipient == r && out[j].Recipient != r
	})
	if max < 0 {
		max = 0
	}
	if len(out) > max {
		return out[:max]
	}
	return out
}
