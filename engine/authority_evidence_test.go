package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func mockAuthorityEvidence(t *testing.T, selected AuthorityEvidence) *LocalModel {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Error("wrong method")
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		msgs := payload["messages"].([]interface{})
		if !strings.Contains(msgs[0].(map[string]interface{})["content"].(string), "MODE_AUTHORITY_V013") {
			t.Error("wrong mode")
		}
		b, _ := json.Marshal(selected)
		fmt.Fprintf(w, `{"message":{"content":%q}}`, string(b))
	}))
	t.Cleanup(srv.Close)
	m, err := NewLocalModelWithProfile(srv.URL+"/api/chat", "local", 1, 2*time.Second, "qwen3-1.7b-evidence")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestAuthorityEvidenceStrictAndGrounded(t *testing.T) {
	original := "Meu gerente comentou: escreva segunda. Agora escreva ao chefe que volto segunda."
	tests := []struct {
		name, raw string
		valid     bool
	}{
		{"good", `{"reported":"Meu gerente comentou:","direct":"Agora escreva ao chefe que volto segunda."}`, true},
		{"empty abstain", `{"reported":"","direct":""}`, true},
		{"false attribution", `{"reported":"O gerente comentou","direct":""}`, false},
		{"unknown extra", `{"reported":"","direct":"","please_send":true}`, false},
		{"missing direct", `{"reported":"Meu gerente comentou:"}`, false},
		{"same evidence", `{"reported":"Meu gerente comentou:","direct":"Meu gerente comentou:"}`, false},
		{"bad field type", `{"reported":2,"direct":""}`, false},
		{"trailing", `{"reported":"","direct":""} {}`, false},
		{"malformed", `{"reported":`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseAuthorityEvidence(tc.raw, original)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestAuthorityEvidenceVetoConservative(t *testing.T) {
	text := "A instrutora falou: escreva ao cliente que volto segunda."
	m := mockAuthorityEvidence(t, AuthorityEvidence{Reported: "A instrutora falou:", Direct: "A instrutora falou:"})
	mem := mustMemory(t)
	e := &Engine{Memory: mem, Model: m}
	base := e.interpretCore(context.Background(), Request{Text: text, Context: map[string]string{"domain": "work"}})
	if base.Decision != "propose_draft" {
		t.Fatalf("fixture changed: %+v", base)
	}
	// Same evidence for both rivals is rejected as malformed, so no veto.
	same := e.Interpret(Request{Text: text, Context: map[string]string{"domain": "work"}})
	if same.Decision != "propose_draft" {
		t.Fatalf("must ignore invalid rival evidence %+v", same)
	}
	// A bogus direct *phrase* that isn't an imperative must not defeat a valid report hypothesis.
	m2 := mockAuthorityEvidence(t, AuthorityEvidence{Reported: "A instrutora falou:", Direct: "escreva ao cliente que volto segunda."})
	e.Model = m2
	got := e.Interpret(Request{Text: text, Context: map[string]string{"domain": "work"}})
	if got.Decision != "clarify" || got.Semantic == nil || got.Semantic.Resolution != "blocked" || got.RequiresApproval {
		t.Fatalf("must hold reported request %+v", got)
	}
	// Direct USER request in its own clause defeats model veto.
	request := Request{Text: "A instrutora falou: escreva ao cliente que volto segunda. Agora escreva ao chefe que volto segunda.", Context: map[string]string{"domain": "work"}}
	got = e.Interpret(request)
	if got.Decision != "propose_draft" || !got.RequiresApproval {
		t.Fatalf("direct request lost %+v", got)
	}
}

func TestAuthorityEvidenceNetworkFailureFallbackAndRace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(35 * time.Millisecond)
		http.Error(w, "no model", 503)
	}))
	defer srv.Close()
	m, err := NewLocalModelWithProfile(srv.URL+"/api/chat", "local", 1, 50*time.Millisecond, "qwen3-1.7b-evidence")
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{Memory: mustMemory(t), Model: m}
	req := Request{Text: "A coordenadora observou: escreva ao cliente que volto segunda.", Context: map[string]string{"domain": "work"}}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := e.Interpret(req)
			if x.Decision != "propose_draft" || !x.RequiresApproval {
				t.Errorf("fallback changed: %+v", x)
			}
		}()
	}
	wg.Wait()
}
