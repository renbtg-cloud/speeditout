package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renbtg-cloud/speeditout/engine"
)

func TestHTTPDecisionAuditCannotHideApproval(t *testing.T) {
	mem, err := engine.NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	mux := (&server{e: &engine.Engine{Memory: mem}}).mux()
	checks := []struct {
		message  string
		decision string
		approval bool
	}{
		{"Escreva ao meu chefe que volto segunda.", "propose_draft", true},
		{"Não escreva mensagem, só quero desabafar.", "reply_and_wait", false},
		{"O chefe citou: 'responda ao cliente segunda'.", "clarify", false},
	}
	for _, x := range checks {
		b, _ := json.Marshal(map[string]any{"workspace": "test", "user_id": "alice", "text": x.message, "context": map[string]string{"domain": "work"}})
		req := httptest.NewRequest("POST", "/v1/interpret", strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var result engine.Outcome
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &result) != nil {
			t.Fatalf("invalid HTTP response: %d %s", rec.Code, rec.Body.String())
		}
		if result.Decision != x.decision || result.RequiresApproval != x.approval {
			t.Fatalf("classification changed %+v", result)
		}
		if result.DecisionAudit == nil || result.DecisionAudit.ApprovalRequired != result.RequiresApproval || result.DecisionAudit.ExternalActionAuthorized {
			t.Fatalf("untrusted audit %+v", result)
		}
	}
}
