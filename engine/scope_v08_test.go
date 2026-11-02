package engine

import (
	"strings"
	"testing"
)

// Direct-draft preflight (v0.7+) must continue honoring individual preferences.
func TestV08ScopedDraftUsesConsentedPreferences(t *testing.T) {
	m, err := NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	eng := Engine{Memory: m}
	request := Request{Workspace: "corp", UserID: "person", Text: "Escreva ao chefe que volto a isso na segunda.", Context: map[string]string{"domain": "work", "recipient": "boss"}}
	before := eng.Interpret(request)
	if before.Decision != "propose_draft" || before.Intent != "defer_work" || !before.RequiresApproval || !strings.HasPrefix(before.Reply, "Recebi sua mensagem. ") {
		t.Fatalf("unexpected default draft: %+v", before)
	}
	if err := m.SetPreference("corp", "person", "greeting", "omit", "boss", true); err != nil {
		t.Fatal(err)
	}
	after := eng.Interpret(request)
	if strings.HasPrefix(after.Reply, "Recebi sua mensagem.") {
		t.Fatalf("personal preference ignored: %+v", after)
	}
	request.Context["recipient"] = "colleague"
	other := eng.Interpret(request)
	if !strings.HasPrefix(other.Reply, "Recebi sua mensagem.") {
		t.Fatalf("recipient preference leaked: %+v", other)
	}
	request.Context["recipient"] = "boss"
	if err := m.SetPreference("corp", "person", "avoid_phrase", "segunda-feira", "boss", true); err != nil {
		t.Fatal(err)
	}
	blocked := eng.Interpret(request)
	if blocked.Decision != "clarify" || blocked.Intent != "defer_work" {
		t.Fatalf("forbidden phrase not caught: %+v", blocked)
	}
}
