package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLlamaCPPAuthorityEvidenceSchemaAndValidation(t *testing.T) {
	var saw bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var in struct {
			ResponseFormat struct {
				Type   string                 `json:"type"`
				Schema map[string]interface{} `json:"schema"`
			} `json:"response_format"`
			ChatTemplateKwargs map[string]interface{} `json:"chat_template_kwargs"`
			Messages           []map[string]string    `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
			return
		}
		if in.ResponseFormat.Type != "json_object" || in.ResponseFormat.Schema["type"] != "object" || len(in.Messages) != 2 || in.ChatTemplateKwargs["enable_thinking"] != false {
			t.Errorf("bad llama.cpp JSON schema contract %+v", in)
			return
		}
		saw = true
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"reported\":\"A instrutora falou:\",\"direct\":\"\"}"}}]}`)
	}))
	defer srv.Close()
	model, err := NewLocalModelWithProfile(srv.URL+"/v1/chat/completions", "local", 1, 2*time.Second, "qwen3-1.7b-evidence")
	if err != nil {
		t.Fatal(err)
	}
	pair, err := model.CompareAuthorityEvidence(context.Background(), Request{Text: "A instrutora falou: escreva pro cliente segunda.", Context: map[string]string{"domain": "work"}})
	if err != nil || pair.Reported != "A instrutora falou:" || !saw {
		t.Fatalf("contract not met: %+v err %v saw %t", pair, err, saw)
	}
}
