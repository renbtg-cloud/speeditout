package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renbtg-cloud/speeditout/engine"
)

func TestStrictRequestJSON(t *testing.T) {
	checks := []struct {
		name, payload string
		valid         bool
	}{
		{"normal", `{"consent":true,"user_id":"u","context":{"domain":"work"}}`, true},
		{"duplicate root consent", `{"consent":false,"consent":true}`, false},
		{"duplicate context", `{"context":{"domain":"work","domain":"home"}}`, false},
		{"escaped duplicate", `{"consent":false,"co\u006esent":true}`, false},
		{"nested duplicate", `{"foo":[{"a":1,"a":2}]}`, false},
		{"additional value", `{}[]`, false},
		{"truncated", `{"user_id":"abc"`, false},
		{"valid Unicode", `{"text":"não entendi 💬"}`, true},
		{"leading whitespace", " \n {\"x\":1} \t", true},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRequestJSON([]byte(tc.payload))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
	if validateRequestJSON([]byte{0xff, 0xfe}) == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	if validateRequestJSON([]byte(strings.Repeat("[", 18)+"0"+strings.Repeat("]", 18))) == nil {
		t.Fatal("deep JSON accepted")
	}
}

func TestRejectDuplicateConsentOnHTTP(t *testing.T) {
	mem, err := engine.NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	srv := (&server{e: &engine.Engine{Memory: mem}}).mux()
	cases := []struct {
		path, text string
		want       int
	}{
		{"/v1/preferences", `{"workspace":"w","user_id":"u","key":"tone","value":"direct","consent":false,"consent":true}`, 400},
		{"/v1/corrections", `{"workspace":"w","user_id":"u","consent":true,"consent":false}`, 400},
		{"/v1/interpret", `{"workspace":"w","user_id":"u","context":{"domain":"work","domain":"private"},"text":"oi"}`, 400},
		{"/v1/interpret", `{"workspace":"w","user_id":"u","text":"oi"}`, 200},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("POST", tc.path, bytes.NewBufferString(tc.text))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		rw := httptest.NewRecorder()
		srv.ServeHTTP(rw, req)
		if rw.Code != tc.want {
			t.Errorf("%s: got HTTP %d want %d body=%s", tc.path, rw.Code, tc.want, rw.Body.String())
		}
	}
	req := httptest.NewRequest("POST", "/v1/interpret", strings.NewReader(`{"workspace":"w","user_id":"u","text":"oi"}`))
	req.Header.Set("Content-Type", "application/jsonp")
	rw := httptest.NewRecorder()
	srv.ServeHTTP(rw, req)
	if rw.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("content type accepted: %d", rw.Code)
	}
	var o map[string]json.RawMessage
	if err := json.Unmarshal(rw.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
}
