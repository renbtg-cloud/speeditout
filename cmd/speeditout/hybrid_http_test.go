package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/renbtg-cloud/speeditout/engine"
)

func TestHybridHTTPFlowAndIndependentFallback(t *testing.T) {
	var calls atomic.Int64
	provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/api/chat" {
			http.Error(w, "wrong path", 404)
			return
		}
		fmt.Fprint(w, `{"message":{"content":"{\"intent\":\"ghosting_self_blame\"}"}}`)
	}))
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	provider.Listener = ln
	provider.Start()
	defer provider.Close()
	model, err := engine.NewLocalModel(provider.URL+"/api/chat", "synthetic-local", 2, 150*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	mem, err := engine.NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer((&server{e: &engine.Engine{Memory: mem, Model: model}}).mux())
	defer srv.Close()
	data := []byte(`{"workspace":"w","user_id":"u","text":"Fulana me deixou falando sozinho, culpa foi minha."}`)
	resp, err := http.Post(srv.URL+"/v1/interpret", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var got engine.Outcome
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || got.Intent != "ghosting_self_blame" || got.ExternalCalls != 0 || calls.Load() != 1 {
		t.Fatalf("hybrid http integration failed: status=%d out=%+v", resp.StatusCode, got)
	}
	provider.Close() // provider now unavailable; core remains functional
	resp, err = http.Post(srv.URL+"/v1/interpret", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got = engine.Outcome{}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || got.Intent != "unknown" || got.Decision != "clarify" {
		t.Fatalf("dependent on model: status=%d out=%+v", resp.StatusCode, got)
	}
}
