package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type Case struct {
	ID               string            `json:"id"`
	Text             string            `json:"text"`
	ExpectedIntent   string            `json:"expected_intent"`
	ExpectedDecision string            `json:"expected_decision"`
	Context          map[string]string `json:"context"`
	Required         []string          `json:"required"`
	Forbidden        []string          `json:"forbidden"`
}

func TestCorpus(t *testing.T) {
	data, err := os.ReadFile("../cases/adversarial_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	m, _ := NewMemory("")
	e := Engine{Memory: m}
	for _, c := range cases {
		got := e.Interpret(Request{UserID: "test", Text: c.Text, Context: c.Context})
		if got.Decision != c.ExpectedDecision || got.Intent != c.ExpectedIntent {
			t.Errorf("%s: got %s/%s want %s/%s", c.ID, got.Decision, got.Intent, c.ExpectedDecision, c.ExpectedIntent)
		}
		for _, r := range c.Required {
			if !strings.Contains(got.Reply, r) {
				t.Errorf("%s missing %q", c.ID, r)
			}
		}
		for _, r := range c.Forbidden {
			if strings.Contains(strings.ToLower(got.Reply), strings.ToLower(r)) {
				t.Errorf("%s forbidden %q", c.ID, r)
			}
		}
	}
}
func TestConsentAndIsolation(t *testing.T) {
	m, _ := NewMemory("")
	defer m.Close()
	if m.SetPreference("a", "u", "tone", "casual", "", false) == nil {
		t.Fatal("no consent accepted")
	}
	if err := m.SetPreference("a", "u", "tone", "casual", "", true); err != nil {
		t.Fatal(err)
	}
	if m.Preference("b", "u", "tone", "") != "" || m.Preference("a", "v", "tone", "") != "" {
		t.Fatal("scope leak")
	}
	if err := m.SetPreference("a", "u", "tone", "direct", "boss", true); err != nil {
		t.Fatal(err)
	}
	if m.Preference("a", "u", "tone", "boss") != "direct" {
		t.Fatal("recipient override")
	}
}
func TestConcurrentReadsAndWrites(t *testing.T) {
	m, _ := NewMemory("")
	e := Engine{Memory: m}
	var wg sync.WaitGroup
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 1000; n++ {
				res := e.Interpret(Request{UserID: "u", Workspace: "w", Text: "Levei ghosting, acho que mereci"})
				if res.Intent != "ghosting_self_blame" {
					t.Errorf("unexpected %s", res.Intent)
					return
				}
				if i%4 == 0 && n%50 == 0 {
					if err := m.SetPreference("w", "u", "tone", "casual", "", true); err != nil {
						t.Error(err)
					}
				}
			}
		}(g)
	}
	wg.Wait()
}
func TestPersistentMemory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "memory.jsonl")
	m, err := NewMemory(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetPreference("w", "u", "tone", "casual", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.AddExample("w", "u", "chefe espera segunda", "proposta", "boss", "", true); err != nil {
		t.Fatal(err)
	}
	m.Close()
	m2, err := NewMemory(p)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()
	if m2.Preference("w", "u", "tone", "") != "casual" || len(m2.SimilarExamples("w", "u", "chefe espera segunda", "boss", 3)) != 1 {
		t.Fatal("persistence mismatch")
	}
}
func TestApprovedCorrection(t *testing.T) {
	m, _ := NewMemory("")
	e := Engine{Memory: m}
	u := Request{UserID: "u", Text: "chefe fica pra segunda", Context: map[string]string{"domain": "work", "recipient": "boss"}}
	before := e.Interpret(u).Reply
	if !strings.HasPrefix(before, "Recebi sua mensagem. ") {
		t.Fatal("bad default")
	}
	if _, err := m.LearnGreetingCorrection("personal", "u", u.Text, before, strings.TrimPrefix(before, "Recebi sua mensagem. "), "boss", true, true); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(e.Interpret(u).Reply, "Recebi sua mensagem. ") {
		t.Fatal("correction not applied")
	}
	u.Context["recipient"] = "colleague"
	if !strings.HasPrefix(e.Interpret(u).Reply, "Recebi sua mensagem. ") {
		t.Fatal("scope leak")
	}
}
func BenchmarkInterpret(b *testing.B) {
	m, _ := NewMemory("")
	e := Engine{Memory: m}
	r := Request{UserID: "bench", Text: "Fala pro meu chefe tomar no cu, ele espera até segunda!", Context: map[string]string{"domain": "work", "recipient": "boss"}}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e.Interpret(r)
		}
	})
}

func TestDurableAtomicCorrectionRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "atomic-correction.jsonl")
	m, err := NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.LearnGreetingCorrection("w", "u", "chefe pode esperar segunda", "Recebi sua mensagem. Vou retomar esse assunto na segunda-feira.", "Vou retomar esse assunto na segunda-feira.", "boss", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	m, err = NewMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if m.Preference("w", "u", "greeting", "boss") != "omit" || len(m.SimilarExamples("w", "u", "chefe pode esperar segunda", "boss", 3)) != 1 {
		t.Fatal("failed to replay correction as a journal record")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Split(strings.TrimSpace(string(b)), "\n")) != 1 {
		t.Fatal("expected correction in exactly one serialized record")
	}
}
