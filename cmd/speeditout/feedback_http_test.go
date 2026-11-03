package main

import "testing"

func TestHTTPFeedbackConsentAndIsolation(t *testing.T) {
	s, _ := testServer(t)
	resp, _ := post(t, s.URL+"/v1/feedback", `{"workspace":"w","user_id":"a","category":"misheard","consent":false}`)
	if resp.StatusCode != 403 {
		t.Fatalf("nonconsent HTTP status %d", resp.StatusCode)
	}
	resp, result := post(t, s.URL+"/v1/feedback", `{"workspace":"w","user_id":"a","category":"misheard","reference":"opaque-1","note":"voice capture seemed incomplete","consent":true}`)
	if resp.StatusCode != 200 || result["automatically_learned"] != false {
		t.Fatalf("bad save: %d %+v", resp.StatusCode, result)
	}
	id := result["feedback_id"].(float64)
	resp, summary := post(t, s.URL+"/v1/feedback/summary", `{"workspace":"w","user_id":"a"}`)
	if resp.StatusCode != 200 || summary["counts"].(map[string]any)["misheard"] != float64(1) {
		t.Fatalf("bad counts %+v", summary)
	}
	resp, summary = post(t, s.URL+"/v1/feedback/summary", `{"workspace":"w","user_id":"other"}`)
	if resp.StatusCode != 200 || len(summary["counts"].(map[string]any)) != 0 {
		t.Fatalf("cross-user summary %+v", summary)
	}
	resp, _ = post(t, s.URL+"/v1/feedback/delete", `{"workspace":"w","user_id":"a","id":1,"consent":false}`)
	if resp.StatusCode != 403 {
		t.Fatalf("unconsented delete %d", resp.StatusCode)
	}
	resp, _ = post(t, s.URL+"/v1/feedback/delete", `{"workspace":"w","user_id":"other","id":1,"consent":true}`)
	if resp.StatusCode != 404 {
		t.Fatalf("cross-user delete %d", resp.StatusCode)
	}
	if id != 1 {
		t.Fatalf("unexpected ID %f", id)
	}
	resp, ack := post(t, s.URL+"/v1/feedback/delete", `{"workspace":"w","user_id":"a","id":1,"consent":true}`)
	if resp.StatusCode != 200 || ack["journal_tombstone_only"] != true {
		t.Fatalf("delete was not truthful: %+v", ack)
	}
	resp, summary = post(t, s.URL+"/v1/feedback/summary", `{"workspace":"w","user_id":"a"}`)
	if resp.StatusCode != 200 || len(summary["counts"].(map[string]any)) != 0 {
		t.Fatalf("tombstone didn't apply: %+v", summary)
	}
}
