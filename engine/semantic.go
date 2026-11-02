package engine

// v0.9: Explicit, auditable evidence representation ahead of legacy lexical rules.
// The grammar is intentionally bounded. It is NOT a general Portuguese parser.
// Most importantly, an observation of another person's words is not permission
// to act, and an LLM's confidence never overrides the user's constraints.
import (
	"regexp"
	"strings"
)

type SemanticClause struct {
	Text    string `json:"text"`
	Source  string `json:"source"` // user, third_party, uncertain
	Mode    string `json:"mode"`   // assert, question, report, hypothetical
	Negated bool   `json:"negated"`
	Quoted  bool   `json:"quoted"`
}
type SemanticHypothesis struct {
	Intent   string   `json:"intent"`
	Decision string   `json:"decision"`
	Evidence []string `json:"evidence"`
	Blockers []string `json:"blockers,omitempty"`
}
type SemanticTrace struct {
	Version    string               `json:"version"`
	Clauses    []SemanticClause     `json:"clauses"`
	Hypotheses []SemanticHypothesis `json:"hypotheses"`
	Resolution string               `json:"resolution"` // accepted, blocked, insufficient
}

var (
	clauseSep        = regexp.MustCompile(`\s*(?:[;.!]|,\s*(?:mas|por[eé]m|s[oó] que)\b)\s*`)
	quoteRE          = regexp.MustCompile(`(?s)["“][^"”]+["”]`)
	peopleRE         = regexp.MustCompile(`\b(?:minh[ao]s? |meu[s]? |um |uma |o |a )?(?:colega|irm(?:a|ao)|filh[ao]|primo|prima|pai|mae|tia|tio|vizinha|vizinho|cliente|mecanico|professor|professora|gerente|supervisor|diretor|chefe|amiga|amigo)\b|\b(?:ele|ela)\b`)
	attributionRE    = regexp.MustCompile(`\b(?:diz|disse|falou|contou|afirmou|comentou|relatou|garantiu|acha|achou|pediu|escreveu)\b`)
	firstRE          = regexp.MustCompile(`\b(?:eu|me|meu|minha|comigo|mim|levei|tomei|fui|fiz|acho|sinto)\b`)
	directiveRE      = regexp.MustCompile(`\b(?:escrev[ae]|redij[ae]|prepar[ae]|responda|mand[ae]|avis[ae]|transmita|diga|fala pro|fale ao)\b`)
	draftNegRE       = regexp.MustCompile(`\b(?:nao (?:me )?(?:escreva|redija|prepare|responda|mande|envie|faca)|nada de (?:email|e-mail|mensagem|resposta)|nenhum (?:email|e-mail|rascunho)|so (?:quero|queria|preciso) (?:reclamar|desabafar|conversar)|so estou desabafando|so to desabafando|nao vou mandar|nao quero (?:mandar|responder|enviar))\b`)
	noDateRE         = regexp.MustCompile(`\b(?:sem (?:garantir|prometer|assumir|indicar|fixar|inventar|mencionar|definir)(?: (?:nenhuma|qualquer|outra))? (?:data|prazo|segunda)|nao (?:quero|vou|posso) (?:assumir|prometer|garantir|fixar)(?: (?:nenhum|qualquer))? (?:prazo|data|compromisso)|nao invente prazo|nao prometa data|sem data|sem prazo|segunda nao serve)\b`)
	timeRE           = regexp.MustCompile(`\b(?:segunda(?:-feira)?|amanha|terca(?:-feira)?|quarta(?:-feira)?|quinta(?:-feira)?|sexta(?:-feira)?)\b`)
	delayRE          = regexp.MustCompile(`\b(?:avaliar|ver|olhar|retomar|retorno|volto|respondo|responder|revisar|so consigo|adiar|deixar|esperar|analiso|verifico)\b`)
	normLiteralRE    = regexp.MustCompile(`\b(?:exatamente|palavras exatas|literalmente|sem suavizar|sem censura|do jeito que falei)\b`)
	notLiteralRE     = regexp.MustCompile(`\bnao\s+(?:escreva|redija|repita|diga)\s+literalmente\b`)
	swearRE          = regexp.MustCompile(`\b(?:foder|foda|cu|merda|caralho|porra)\b`)
	childConfusionRE = regexp.MustCompile(`\b(?:nao (?:entendi|consigo (?:acompanhar|entender))|sem entender|fiquei confus[oa]|estou confus[oa]|me perdi|perdi a explicacao|qual (?:e )?a regra|explica|explicar|fale mais devagar)\b`)
	childRejectRE    = regexp.MustCompile(`\b(?:detestei|nao gostei|nao quero|prefiro .*diferente|quero (?:trocar|parar|terminar)|chega (?:desse|dessa)|jogo (?:ta|esta|ficou|e) (?:uma droga|chato|cansativo)|brincadeira (?:ta|esta|ficou|e) (?:chata|cansativa))\b`)
	childPositiveRE  = regexp.MustCompile(`\b(?:eu (?:gosto|gostei|adorei)|eu adoro|eu amei|gosto desse|eu achei legal)\b`)
	relationRE       = regexp.MustCompile(`\b(?:me deu (?:um )?(?:gelo|vacuo|ghosting)|levei (?:um )?(?:gelo|vacuo|ghosting)|tomei (?:um )?(?:gelo|vacuo|ghosting)|me (?:bloqueou|ignorou|deixou falando sozinh[oa])|nao me (?:responde|respondeu)|me deixou no vacuo|parou de (?:falar comigo|me responder)|nao recebi resposta|ninguem responde minhas mensagens)\b`)
	blameRE          = regexp.MustCompile(`\b(?:acho que (?:eu |a )?(?:culpa|errei|estraguei)|talvez eu tenha|sinto que|culpa (?:e )?minha|pisei na bola|fui (?:injust[oa]|cruel|babaca)|agi mal|estraguei as coisas|fiz merda|fui escrot[oa]|me culpo|eu (?:feri|magoei|errei))\b`)
	negBlameRE       = regexp.MustCompile(`\b(?:nao (?:acho|creio|penso|concordo)|sem me culpar|nao foi culpa minha|nao me culpo)\b`)
	defineRE         = regexp.MustCompile(`\b(?:significado de|definicao de|que significa|o que e|explique o que e|explica o que e)\s+ghosting\b`)
	motorRE          = regexp.MustCompile(`\b(?:motor|carro|veiculo|acelerador|falha|potencia|engasg|ratei|vibra)\b`)
	symptomRE        = regexp.MustCompile(`\b(?:falha(?:ndo|das?)?|engasg(?:a|ando)?|vibr(?:a|ando)?|ratei(?:a|ando)?|corta|perde (?:potencia|forca)|tranco|falhadas|ruido metalico|barulho metalico)\b`)
	motorResolvedRE  = regexp.MustCompile(`\b(?:nao (?:apresenta|perde|vibra|rateia|falha|engasga|esta (?:falhando|engasgando))|nao .* mais|ja nao|depois do conserto|desde o reparo|resolvi o defeito|era so (?:um )?(?:exemplo|hipotese)|hipotetic[oa])\b`)
	workQuestionRE   = regexp.MustCompile(`\b(?:voce acha|sua opiniao|sera que|eu devia|acha razoavel|seria razoavel)\b`)
)

func contains(re *regexp.Regexp, s string) bool { return re.MatchString(s) }
func splitSemantic(text string) []SemanticClause {
	// Keep quotes as protected spans: quoted profanity must not be treated as a command.
	normalized := norm(text)
	// For this bounded parser, punctuation is split only outside quoted passages.
	var pieces []string
	var buf strings.Builder
	quote := false
	for _, r := range normalized {
		if r == '"' || r == '“' || r == '”' {
			quote = !quote
			buf.WriteRune(r)
			continue
		}
		if !quote && (r == ';' || r == '.' || r == '!') {
			if s := strings.TrimSpace(buf.String()); s != "" {
				pieces = append(pieces, s)
			}
			buf.Reset()
			continue
		}
		buf.WriteRune(r)
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		pieces = append(pieces, s)
	}
	if len(pieces) == 0 {
		pieces = []string{normalized}
	}
	result := make([]SemanticClause, 0, len(pieces))
	for _, p := range pieces {
		c := SemanticClause{Text: p, Source: "user", Mode: "assert", Quoted: contains(quoteRE, p)}
		if strings.Contains(p, "?") {
			c.Mode = "question"
		}
		if a, b := attributionRE.FindStringIndex(p), peopleRE.FindStringIndex(p); a != nil && b != nil && b[0] < a[0] {
			c.Mode = "report"
			c.Source = "third_party"
			// A quoted report explicitly followed by a first-person rebuttal is mixed.
			// Keep the original claim attributed even if the user later rebuts it.
		}
		if strings.Contains(p, "hipotese") || strings.Contains(p, "hipotetico") {
			c.Mode = "hypothetical"
		}
		c.Negated = strings.Contains(p, "nao ") || strings.Contains(p, "sem ") || strings.Contains(p, "nem ")
		result = append(result, c)
	}
	return result
}
func newTrace(req Request) SemanticTrace {
	return SemanticTrace{Version: "0.9", Clauses: splitSemantic(req.Text), Hypotheses: []SemanticHypothesis{}, Resolution: "insufficient"}
}
func traceReturn(o Outcome, trace SemanticTrace, hypothesis SemanticHypothesis) (Outcome, bool) {
	trace.Hypotheses = append(trace.Hypotheses, hypothesis)
	trace.Resolution = "accepted"
	o.Semantic = &trace
	o.Evidence = append(o.Evidence, hypothesis.Evidence...)
	o.Reasons = append(o.Reasons, "semantic_v09")
	return o, true
}
func traceBlock(intent, reply, reason string, tr SemanticTrace) (Outcome, bool) {
	h := SemanticHypothesis{Intent: intent, Decision: "clarify", Blockers: []string{reason}}
	tr.Hypotheses = append(tr.Hypotheses, h)
	tr.Resolution = "blocked"
	o := out("clarify", intent, reply, "low", "semantic_guard", reason)
	o.Semantic = &tr
	return o, true
}

// semanticInterpret only accepts explicit, user-attributed evidence. Anything
// else is delegated to the existing conservative interpreter.
func semanticInterpret(req Request) (Outcome, bool) {
	t := norm(strings.TrimSpace(req.Text))
	if t == "" {
		return Outcome{}, false
	}
	tr := newTrace(req)
	sit, domain := req.Context["situation"], req.Context["domain"]
	isWork := domain == "work" || any(t, "chefe", "diretor", "supervisor", "gestor", "email", "e-mail", "gerente")
	thirdReport := false
	for _, cl := range tr.Clauses {
		if cl.Source == "third_party" {
			thirdReport = true
		}
	}
	unquoted := quoteRE.ReplaceAllString(t, "")
	selfAction := contains(directiveRE, unquoted)
	if sit == "offered_game" {
		if any(t, "mas entendi sim", "mas eu entendi", "entendi sim") {
			return Outcome{}, false
		}
		// Negative assessment of a component is not rejection of the whole game.
		if any(t, "nao gostei do som", "nao gostei da musica", "nao gostei da trilha", "nao gostei das regras", "nao gostei so da fase", "nao gostei da fase", "nao gostei apenas da", "nao gostei so da") {
			return Outcome{}, false // Keep the established component-scoping rule.
		}
		// A reported opinion, rhetorical question or mixed positivity is not rejection.
		if thirdReport || (strings.Contains(t, "?") && !contains(childConfusionRE, t) && !any(t, "que que e isso", "o que e isso")) || contains(childPositiveRE, t) {
			return traceBlock("unknown", "Entendi que há opiniões ou perguntas diferentes. O que você achou?", "game_scope_uncertain", tr)
		}
		if contains(childConfusionRE, t) || any(t, "que que e isso", "o que e isso") {
			h := SemanticHypothesis{Intent: "did_not_understand", Decision: "reply_and_wait", Evidence: []string{"user_confusion_in_game_context"}}
			return traceReturn(out("reply_and_wait", h.Intent, "Acho que expliquei mal. Qual parte ficou confusa?", "bounded"), tr, h)
		}
		if contains(childRejectRE, t) {
			h := SemanticHypothesis{Intent: "reject_game", Decision: "reply_and_wait", Evidence: []string{"user_negative_game_preference"}}
			return traceReturn(out("reply_and_wait", h.Intent, "Acho que eu falei besteira, você não gostou, né?", "bounded"), tr, h)
		}
	}
	if contains(defineRE, t) {
		h := SemanticHypothesis{Intent: "define_ghosting", Decision: "reply_and_wait", Evidence: []string{"explicit_definition_request"}}
		return traceReturn(out("reply_and_wait", h.Intent, "Ghosting é quando alguém interrompe a comunicação sem explicar o motivo.", "bounded"), tr, h)
	}
	if contains(relationRE, t) {
		if thirdReport || contains(negBlameRE, t) {
			return traceBlock("unknown", "Não quero presumir culpa ou intenção. Quer contar o que aconteceu?", "relationship_attribution_or_negation", tr)
		}
		if contains(blameRE, t) {
			h := SemanticHypothesis{Intent: "ghosting_self_blame", Decision: "reply_and_wait", Evidence: []string{"self_experienced_disconnection", "explicit_self_blame"}}
			return traceReturn(out("reply_and_wait", h.Intent, "Por que você acha que pode ter sido culpa sua?", "bounded"), tr, h)
		}
	}
	if isWork {
		if contains(draftNegRE, t) && !contains(notLiteralRE, t) && !contains(noDateRE, t) {
			h := SemanticHypothesis{Intent: "do_not_draft", Decision: "reply_and_wait", Evidence: []string{"explicit_no_draft_constraint"}}
			return traceReturn(out("reply_and_wait", h.Intent, "Entendi. Não vou preparar nem enviar uma resposta agora.", "bounded"), tr, h)
		}
		if (contains(workQuestionRE, t) && strings.Contains(t, "?") && (domain == "work" || contains(timeRE, t) || any(t, "responder", "resposta", "prazo"))) && !selfAction {
			h := SemanticHypothesis{Intent: "question_about_work", Decision: "clarify", Evidence: []string{"work_advice_question_without_draft_permission"}}
			return traceReturn(out("clarify", h.Intent, "Você quer conversar sobre o prazo ou preparar uma mensagem?", "low"), tr, h)
		}
		if thirdReport && !selfAction && !any(t, "ele pode esperar", "fica para segunda", "fica pra segunda") {
			return traceBlock("unknown", "Preciso de mais contexto: isso é a decisão de outra pessoa. O que você gostaria que eu fizesse?", "third_party_not_user_command", tr)
		}
		if selfAction && contains(noDateRE, t) {
			return traceBlock("defer_work", "Você quer uma resposta sem compromisso de data. Como prefere formular o prazo?", "date_commitment_not_authorized", tr)
		}
		if selfAction && contains(normLiteralRE, t) && !contains(notLiteralRE, t) && contains(swearRE, t) {
			quoted := quoteRE.FindString(req.Text)
			if quoted != "" {
				h := SemanticHypothesis{Intent: "literal_message", Decision: "propose_draft", Evidence: []string{"explicit_literal_instruction", "quoted_payload"}}
				o := out("propose_draft", h.Intent, strings.Trim(quoted, "\"“”"), "bounded")
				o.RequiresApproval = true
				return traceReturn(o, tr, h)
			}
		}
		if selfAction && contains(delayRE, t) && contains(timeRE, t) {
			// Only commit to two supported dates; never invent other dates.
			deadline := ""
			if strings.Contains(t, "segunda") {
				deadline = "na segunda-feira"
			} else if strings.Contains(t, "amanha") {
				deadline = "amanhã"
			}
			if deadline == "" {
				return traceBlock("defer_work", "Você quer adiar. Qual prazo devo mencionar?", "unsupported_date", tr)
			}
			h := SemanticHypothesis{Intent: "defer_work", Decision: "propose_draft", Evidence: []string{"explicit_draft_command", "expressed_delay", "supported_explicit_date"}}
			o := out("propose_draft", h.Intent, "Recebi sua mensagem. Vou retomar esse assunto "+deadline+" e retorno quando tiver uma avaliação.", "bounded")
			o.RequiresApproval = true
			return traceReturn(o, tr, h)
		}
	}
	if domain == "auto_mechanics" {
		// A hypothetical lesson or an unverified report is not a direct symptom.
		if any(t, "nao existe falha", "so descrevendo o relato", "so citei", "possivel cenario", "hipotese de aula", "falha hipotetica") {
			return traceBlock("unknown", "Esse não parece ser um defeito verificado. Qual é o contexto?", "hypothetical_or_unverified", tr)
		}
		if any(t, "nao e depois que esquenta", "nao depois que esquenta") && strings.Contains(t, "frio") {
			// Existing condition-scoping logic preserves cold-engine constraint.
			return Outcome{}, false
		}
		if thirdReport && contains(symptomRE, t) {
			return traceBlock("unknown", "Esse sintoma foi relatado por outra pessoa. Você conseguiu verificá-lo?", "third_party_mechanic_report", tr)
		}
		if contains(motorResolvedRE, t) && (contains(symptomRE, t) || contains(motorRE, t)) && !any(t, "nao e depois que esquenta", "nao depois que esquenta") {
			if strings.Contains(t, "hipotese") || strings.Contains(t, "exemplo") || strings.Contains(t, "aula") {
				return traceBlock("unknown", "Entendi que era um exemplo ou hipótese, não um defeito confirmado.", "hypothetical_symptom", tr)
			}
			h := SemanticHypothesis{Intent: "ambiguous_symptom", Decision: "clarify", Evidence: []string{"negated_or_resolved_mechanical_symptom"}}
			return traceReturn(out("clarify", h.Intent, "Entendi a negação ou resolução do sintoma. O que acontece agora?", "low"), tr, h)
		}
		if contains(symptomRE, t) && !thirdReport {
			h := SemanticHypothesis{Intent: "technical_symptom", Decision: "propose_draft", Evidence: []string{"direct_mechanical_symptom", "no_causal_diagnosis"}}
			o := out("propose_draft", h.Intent, "Sintoma relatado: "+strings.TrimSpace(req.Text)+". Isso ainda não identifica a causa.", "bounded")
			o.RequiresApproval = true
			return traceReturn(o, tr, h)
		}
	}
	return Outcome{}, false
}
