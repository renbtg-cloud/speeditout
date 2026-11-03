package main

import "testing"

func TestResearchListenerRejectsExposure(t *testing.T) {
	for _, addr := range []string{
		"0.0.0.0:18761", "[::]:18761", "192.168.1.10:18761",
		"example.com:18761", ":18761", "localhost", "127.0.0.1",
		"not-local:18761", "[fe80::1]:18761", "[::ffff:192.168.0.1]:18761",
	} {
		if err := researchListener(addr); err == nil {
			t.Errorf("unsafe listener accepted: %s", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:18761", "localhost:18761", "[::1]:18761", "[::ffff:127.0.0.1]:18761"} {
		if err := researchListener(addr); err != nil {
			t.Errorf("safe listener rejected %s: %v", addr, err)
		}
	}
}
