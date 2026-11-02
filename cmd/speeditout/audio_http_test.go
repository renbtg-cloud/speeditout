package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAudioAssessHTTPEndpoint(t *testing.T) {
	srv := &server{}
	handler := srv.mux()
	tests := []struct {
		name   string
		method string
		body   string
		status int
		expect string
	}{
		{"uncertain", "POST", `{"transcript":"Fala pro chefe esperar", "transcription_status":"uncertain", "platform":"android", "input_route":"speakerphone", "attempts":2}`, 200, `"level":"confirm"`},
		{"bad audio", "POST", `{"signal":{"rms_dbfs":-70,"clipped_fraction":0,"duration_seconds":3},"transcription_status":"ready","platform":"ios","input_route":"handset"}`, 200, `"level":"retry"`},
		{"invalid property", "POST", `{"bogus":1}`, 400, "invalid JSON"},
		{"invalid ASR status", "POST", `{"transcription_status":"excellent","platform":"web","input_route":"unknown"}`, 400, "invalid transcription_status"},
		{"get not allowed", "GET", "", 405, "Method Not Allowed"},
		{"too big", "POST", `{"transcript":"` + strings.Repeat("a", 8193) + `","transcription_status":"ready","platform":"web","input_route":"unknown"}`, 400, "transcript too long"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/v1/audio/assess", bytes.NewBufferString(tc.body))
			if tc.method == "POST" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.expect) {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.status == 200 {
				var res map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
					t.Fatal(err)
				}
				if res["auto_send_allowed"] != false {
					t.Fatalf("unsafe: %v", res)
				}
			}
		})
	}
}
