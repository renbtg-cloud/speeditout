package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/renbtg-cloud/speeditout/engine"
)

func testServer(t *testing.T) (*httptest.Server, *engine.Memory) {
	t.Helper()
	m, err := engine.NewMemory(filepath.Join(t.TempDir(), "memory.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer((&server{e: &engine.Engine{Memory: m}}).mux())
	t.Cleanup(func() { s.Close(); m.Close() })
	return s, m
}
func post(t *testing.T, url string, body string) (*http.Response, map[string]any) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	return resp, got
}
func TestEndToEndHTTPAndConsent(t *testing.T) {
	s, _ := testServer(t)
	target := s.URL + "/v1/interpret"
	request := `{"user_id":"u","workspace":"w","text":"chefe fica pra segunda","context":{"domain":"work","recipient":"boss"}}`
	resp, got := post(t, target, request)
	if resp.StatusCode != 200 || got["intent"] != "defer_work" {
		t.Fatalf("bad interpretation %d %v", resp.StatusCode, got)
	}
	before := got["reply"].(string)
	if !strings.HasPrefix(before, "Recebi sua mensagem. ") {
		t.Fatal("baseline changed")
	}
	resp, _ = post(t, s.URL+"/v1/preferences", `{"user_id":"u","workspace":"w","key":"tone","value":"profane","consent":false}`)
	if resp.StatusCode != 403 {
		t.Fatalf("consent bypass: %d", resp.StatusCode)
	}
	correction := map[string]any{"user_id": "u", "workspace": "w", "recipient": "boss", "utterance": "chefe fica pra segunda", "before": before, "after": strings.TrimPrefix(before, "Recebi sua mensagem. "), "consent": true, "generalize": true}
	payload, _ := json.Marshal(correction)
	resp, got = post(t, s.URL+"/v1/corrections", string(payload))
	if resp.StatusCode != 200 || got["saved"] != true {
		t.Fatalf("correction not saved %d %v", resp.StatusCode, got)
	}
	resp, got = post(t, target, request)
	if resp.StatusCode != 200 || strings.HasPrefix(got["reply"].(string), "Recebi sua mensagem. ") {
		t.Fatalf("learning not applied %v", got)
	}
	other := strings.Replace(request, `"boss"`, `"colleague"`, 1)
	_, got = post(t, target, other)
	if !strings.HasPrefix(got["reply"].(string), "Recebi sua mensagem. ") {
		t.Fatalf("recipient leaked %v", got)
	}
}
func TestHTTPInputValidation(t *testing.T) {
	s, _ := testServer(t)
	url := s.URL + "/v1/interpret"
	tests := []struct {
		body   string
		status int
	}{
		{`{bad`, 400},
		{`{"user_id":"x","workspace":"w","text":"hi","unknown":4}`, 400},
		{`{"user_id":"x","workspace":"w","text":"hi"} {}`, 400},
		{`{"user_id":"","workspace":"w","text":"hi"}`, 400},
		{`{"user_id":"x","workspace":"w","text":"hi"}`, 200},
	}
	for _, tc := range tests {
		r, _ := post(t, url, tc.body)
		if r.StatusCode != tc.status {
			t.Errorf("body %q got %d expected %d", tc.body, r.StatusCode, tc.status)
		}
	}
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(`{"user_id":"x","workspace":"w","text":"hi"}`))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 415 {
		t.Errorf("content type bypass %d", resp.StatusCode)
	}
}
func TestConcurrentHTTP(t *testing.T) {
	s, _ := testServer(t)
	payload := `{"user_id":"load","workspace":"bench","text":"Levei um ghosting. Acho que mereci.","context":{}}`
	var wg sync.WaitGroup
	for j := 0; j < 32; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				resp, err := http.Post(s.URL+"/v1/interpret", "application/json", strings.NewReader(payload))
				if err != nil {
					t.Error(err)
					return
				}
				var result engine.Outcome
				err = json.NewDecoder(resp.Body).Decode(&result)
				resp.Body.Close()
				if err != nil || result.Intent != "ghosting_self_blame" {
					t.Errorf("failure %v %v", err, result.Intent)
					return
				}
			}
		}()
	}
	wg.Wait()
}
