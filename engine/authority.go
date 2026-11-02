package engine

import (
	"regexp"
	"strings"
)

// Authority preflight is deliberately narrower than intent classification.
// It catches cases where someone else's imperative, a quotation, or a classroom
// example contains an apparent command. Such text is NOT user authorization.
// A later, separate user directive is considered independently. No messages
// are ever sent by this engine, including when the preflight does not apply.
var (
	reportedStartRE     = regexp.MustCompile(`^(?:(?:(?:meu|minha|o|a|um|uma)\s+)?(?:chefe|gerente|diretor|diretora|supervisor|supervisora|professor|professora|colega|filho|filha|amigo|amiga)|ele|ela)\s+(?:me\s+)?(?:disse|diz|falou|pediu|solicitou|ordenou|mandou|sugeriu|citou|repetiu|comentou|contou|escreveu)\b`)
	reportedVariationRE = regexp.MustCompile(`^(?:quando (?:ele|ela) (?:disse|falou|pediu)|(?:citaram|mencionaram) (?:a frase|o exemplo))\b`)
	hypotheticalStartRE = regexp.MustCompile(`^(?:suponha que|imagine que|isso e (?:um )?exemplo|no exemplo|num exemplo|na aula|no treinamento|talvez eu precise|se (?:o|a|meu|minha|ele|ela)\b)`)
	tentativeQuestionRE = regexp.MustCompile(`^(?:eu deveria|eu queria saber se devo|seria melhor|devo)\s+(?:escrever|responder|mandar|avisar|falar)\b`)
	reportedActionRE    = regexp.MustCompile(`\b(?:escreva|escrever|redija|redigir|prepare|preparar|responda|responder|mande|mandar|envie|enviar|avise|avisar|diga|fale|falar|retomar|retorno|volto|voltaria)\b`)
	userDecisionRE      = regexp.MustCompile(`^(?:fica (?:para|pra) |eu (?:vou|quero|decidi)|vou |preciso )`)
	userDirectiveRE     = regexp.MustCompile(`\b(?:escreva|redija|prepare|responda|mande|avise|diga|fale|fala pro|diz pro)\b`)
)

func authorityPreflight(req Request) (Outcome, bool) {
	t := norm(strings.TrimSpace(req.Text))
	if t == "" {
		return Outcome{}, false
	}
	domain := req.Context["domain"]
	if domain != "work" && !any(t, "chefe", "gerente", "diretor", "diretora", "supervisor", "email", "e-mail", "cliente") {
		return Outcome{}, false
	}

	// A question about whether to act is not an instruction to act.
	if strings.Contains(t, "?") && tentativeQuestionRE.MatchString(t) {
		trace := newTrace(req)
		h := SemanticHypothesis{Intent: "question_about_work", Decision: "clarify", Evidence: []string{"tentative_question"}}
		return authorityAccept(out("clarify", h.Intent, "Quer conversar sobre isso ou preparar uma mensagem?", "low"), trace, h)
	}

	// Explicit user-level refusal outranks any reported instructions. Let the
	// existing handling provide its established non-drafting response.
	if contains(draftNegRE, t) || any(t, "nao quero mandar", "nao quero responder", "nao quero enviar", "nao quero mensagem", "nao prepare", "nao redija", "nao responda", "nao envie") {
		return Outcome{}, false
	}

	chunks := splitSemantic(req.Text)
	sawReport := false
	ownCommand := false
	userQuestion := false
	for _, chunk := range chunks {
		s := strings.TrimSpace(chunk.Text)
		outsideQuote := strings.TrimSpace(quoteRE.ReplaceAllString(s, " "))
		reported := reportedStartRE.MatchString(outsideQuote) || reportedVariationRE.MatchString(outsideQuote)
		hypothetical := hypotheticalStartRE.MatchString(outsideQuote)
		quotedOnly := outsideQuote == "" && strings.TrimSpace(s) != ""
		if reported || hypothetical || quotedOnly {
			if (reported && reportedActionRE.MatchString(s)) || (hypothetical && reportedActionRE.MatchString(s)) || quotedOnly {
				sawReport = true
			}
			continue
		}
		// An imperative outside attributed/quoted/example material, in its
		// OWN clause, can be considered for drafting by later safety gates.
		if (userDirectiveRE.MatchString(outsideQuote) && !strings.Contains(outsideQuote, "?")) || userDecisionRE.MatchString(outsideQuote) {
			ownCommand = true
		}
		if strings.Contains(outsideQuote, "sua opiniao") && strings.Contains(outsideQuote, "?") {
			userQuestion = true
		}
	}
	if !sawReport || ownCommand || userQuestion {
		return Outcome{}, false
	}

	trace := newTrace(req)
	return authorityBlock(out("clarify", "unknown", "Isso parece uma instrução citada ou hipotética. Você quer conversar sobre ela ou me pedir um rascunho?", "low"), trace, "quoted_or_reported_instruction")
}

func authorityAccept(o Outcome, trace SemanticTrace, h SemanticHypothesis) (Outcome, bool) {
	trace.Resolution = "accepted"
	trace.Hypotheses = append(trace.Hypotheses, h)
	o.Semantic = &trace
	o.Evidence = append(o.Evidence, h.Evidence...)
	o.Reasons = append(o.Reasons, "authority_scope_v011")
	return o, true
}

func authorityBlock(o Outcome, trace SemanticTrace, blocker string) (Outcome, bool) {
	trace.Resolution = "blocked"
	trace.Hypotheses = append(trace.Hypotheses, SemanticHypothesis{Intent: o.Intent, Decision: o.Decision, Blockers: []string{blocker}})
	o.Semantic = &trace
	o.Reasons = append(o.Reasons, "authority_scope_v011", blocker)
	return o, true
}
