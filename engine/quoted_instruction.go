package engine

import (
	"strings"
	"unicode/utf8"
)

// hasQuotedImperative identifies a quoted imperative; it does not decide who
// uttered it, whether the quotation is true, or what the user wants done.
func hasQuotedImperative(text string) bool {
	if len(text) > 8192 || !utf8.ValidString(text) {
		return false
	}
	var quote rune
	var b strings.Builder
	for _, r := range text {
		if quote == 0 {
			if r == '\'' || r == '"' || r == '“' || r == '‘' {
				quote = r
				b.Reset()
			}
			continue
		}
		if (quote == '\'' && (r == '\'' || r == '’')) || (quote == '"' && r == '"') || (quote == '“' && r == '”') || (quote == '‘' && r == '’') {
			q := strings.TrimSpace(norm(b.String()))
			if ownDirectCue.MatchString(q) {
				return true
			}
			quote = 0
		} else {
			b.WriteRune(r)
		}
	}
	return false
}

// reportedQuoteGuard is a deliberately narrow evidence check: an imperative
// inside literal quotation marks with a reported-speech cue OUTSIDE them is
// not independently authorized. Fail conservative: ask, do not make a draft.
// It never changes an explicit user refusal and never blocks an independently
// expressed directive outside the quote.
func reportedQuoteGuard(req Request) (Outcome, bool) {
	t := norm(req.Text)
	if req.Context["domain"] != "work" && !any(t, "chefe", "gerente", "cliente", "email", "e-mail", "diretor", "supervisor", "equipe") {
		return Outcome{}, false
	}
	if contains(draftNegRE, t) || any(t, "nao quero mensagem", "nao quero mandar", "nao envie", "nao responda", "nao prepare") {
		return Outcome{}, false
	}
	if !hasQuotedImperative(req.Text) {
		return Outcome{}, false
	}
	outside := norm(stripQuotedText(req.Text))
	if strings.Contains(outside, "sua opiniao") && strings.Contains(outside, "?") {
		return Outcome{}, false
	}
	if strings.Contains(outside, "quem pediu fui eu: escreva") || strings.Contains(outside, "quem pediu fui eu: redija") {
		return Outcome{}, false
	}
	if !reportCue.MatchString(outside) {
		return Outcome{}, false
	}
	// A clear user command in any unquoted clause defeats this protection.
	// Later Go gates still require human approval before any draft can be sent.
	for _, cl := range splitSemantic(outside) {
		if cl.Source != "third_party" && cl.Mode != "hypothetical" && cl.Mode != "question" && ownDirectCue.MatchString(strings.TrimSpace(cl.Text)) {
			return Outcome{}, false
		}
	}
	o := out("clarify", "unknown", "Isso parece uma ordem citada, não um pedido seu. Você quer apenas conversar sobre ela ou preparar um rascunho?", "low", "quoted_order_provenance", "no_unquoted_directive")
	tr := newTrace(req)
	tr.Resolution = "blocked"
	tr.Hypotheses = []SemanticHypothesis{{Intent: "authority_reported", Decision: "clarify", Blockers: []string{"no independent user directive in unquoted text"}}}
	o.Semantic = &tr
	return o, true
}
