package engine

import (
	"math/rand"
	"strings"
	"testing"
)

// Reproducible pseudo-random input mutations: this checks policy invariants,
// NOT semantic correctness. It does not claim to sample real human dialogue.
func TestPolicyInvariantsAcrossTenThousandSyntheticMutations(t *testing.T) {
	e := &Engine{Memory: mustMemory(t)}
	rng := rand.New(rand.NewSource(20261102))
	beginnings := []string{"", "meu chefe falou:", "a professora citou:", "agora", "eu não quero", "por favor", "no treinamento", "o gerente disse"}
	contents := []string{"escreva ao chefe que volto segunda", "não envie mensagem", "só quero desabafar", "me deixou no vácuo", "não gostei do jogo", "o carro rateia na subida", "volto amanhã", "ainda não sei", "o que você acha?"}
	punctuation := []string{"", ".", "?", "!", ":", ";", " '"}
	samples := 10000
	if testing.Short() {
		samples = 1000
	}
	for i := 0; i < samples; i++ {
		n := 1 + rng.Intn(4)
		words := make([]string, 0, n+1)
		words = append(words, beginnings[rng.Intn(len(beginnings))])
		for j := 0; j < n; j++ {
			words = append(words, contents[rng.Intn(len(contents))]+punctuation[rng.Intn(len(punctuation))])
		}
		utterance := strings.Join(words, " ")
		ctx := map[string]string{"domain": "work"}
		if i%4 == 0 {
			ctx = map[string]string{"situation": "offered_game"}
		}
		got := e.Interpret(Request{Text: utterance, Context: ctx})
		if got.DecisionAudit == nil || got.DecisionAudit.ExternalActionAuthorized ||
			(got.Decision == "propose_draft") != got.RequiresApproval || got.ExternalCalls != 0 {
			t.Fatalf("policy broken at seed-case %d: %+v", i, got)
		}
	}
}
