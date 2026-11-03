package engine

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var accent = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "ê", "e", "è", "e", "ë", "e", "í", "i", "ì", "i", "ï", "i", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ò", "o", "ú", "u", "ü", "u", "ù", "u", "ç", "c", "ñ", "n")

func norm(s string) string {
	return accent.Replace(strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, s)))
}
func any(t string, terms ...string) bool {
	for _, s := range terms {
		if strings.Contains(t, s) {
			return true
		}
	}
	return false
}

var rude = []string{"tomar no cu", "vai se foder", "ir se foder", "se foda", "foda-se", "fuck off", "porra", "caralho", "merda", "puta que pariu"}
var delay = []string{"segunda", "terca", "quarta", "quinta", "sexta", "amanha", "depois", "esper", "mais tempo", "preciso de tempo", "nao hoje", "nao agora", "nao vou fazer", "adiar", "revisar", "analisar", "retomo", "retomar"}
var techSymptoms = []string{"engasg", "falha", "perde forca", "morre", "rateia", "rateand", "pipoc", "vibr", "soluc", "afog"}
var techTriggers = []string{"piso fundo", "acelero", "aceler", "esquent", "quente", "frio", "subida", "marcha lenta", "motor", "carro"}
var rejection = []string{"nao gostei", "nao quero", "chato", "chata", "sem graca", "bobo", "boba", "que saco", "nada a ver", "muito facil", "ta ruim", "horrivel"}
var confused = []string{"nao entendi", "nao compreendi", "que que e isso", "nao saquei", "to boiando"}
var ghosting = []string{"ghosting", "me deu ghost", "me deixou no vacuo", "sumiu", "parou de responder", "me ignorou"}
var selfBlame = []string{"mereci", "culpa minha", "fui babaca", "eu vacilei", "fiz merda", "eu errei", "fui escroto", "fui escrota"}
var literal = []string{"literalmente", "palavras exatas", "sem suavizar", "sem censura", "sem mudar nada", "do jeito que falei", "nao ameniza", "exatamente:"}
var quoted = regexp.MustCompile(`["“](.+?)["”]`)

type Request struct {
	UserID    string            `json:"user_id"`
	Text      string            `json:"text"`
	Context   map[string]string `json:"context"`
	Workspace string            `json:"workspace"`
}
type Outcome struct {
	Decision          string         `json:"decision"`
	Intent            string         `json:"intent"`
	Reply             string         `json:"reply"`
	Confidence        string         `json:"confidence"`
	Reasons           []string       `json:"reasons"`
	Evidence          []string       `json:"evidence"`
	MatchedExampleIDs []int64        `json:"matched_example_ids"`
	RequiresApproval  bool           `json:"requires_approval"`
	ExternalCalls     int            `json:"external_calls"`
	DecisionAudit     *DecisionAudit `json:"decision_audit,omitempty"`
	Semantic          *SemanticTrace `json:"semantic,omitempty"`
}

func out(dec, intent, reply, conf string, reasons ...string) Outcome {
	return Outcome{Decision: dec, Intent: intent, Reply: reply, Confidence: conf, Reasons: reasons, Evidence: []string{}, MatchedExampleIDs: []int64{}, ExternalCalls: 0}
}

type Engine struct {
	Memory *Memory
	Model  *LocalModel
	// ModelBudget bounds optional inference without changing deterministic work.
	// Zero preserves the historical provider-configured timeout.
	ModelBudget time.Duration
}

// Interpret preserves the original research API for synchronous callers.
func (e *Engine) Interpret(req Request) Outcome {
	return e.InterpretContext(context.Background(), req)
}

// InterpretContext propagates caller cancellation to optional local inference.
// Essential rule/memory behavior remains independent of model availability.
func (e *Engine) InterpretContext(c context.Context, req Request) Outcome {
	if c == nil {
		c = context.Background()
	}
	// The budget covers BOTH navigator classification in interpretCore and
	// evidence/ranking in auditDraft. In v0.15 it only covered auditDraft;
	// that left an unexpectedly slow fallback navigator unbounded.
	if e.Model != nil && e.ModelBudget > 0 {
		bounded, cancel := context.WithTimeout(c, e.ModelBudget)
		defer cancel()
		c = bounded
	}
	base := e.interpretCore(c, req)
	return enforceDecisionPolicy(e.auditDraft(c, req, base))
}

func (e *Engine) interpretCore(c context.Context, req Request) Outcome {
	if c == nil {
		c = context.Background()
	}
	original := strings.TrimSpace(req.Text)
	t := norm(original)
	ctx := req.Context
	if ctx == nil {
		ctx = map[string]string{}
	}
	audience := ctx["recipient"]
	situation := ctx["situation"]
	ws := req.Workspace
	if ws == "" {
		ws = "personal"
	}
	user := req.UserID
	if user == "" {
		user = "anonymous"
	}
	style := e.Memory.Preference(ws, user, "tone", audience)
	if style == "" {
		style = "neutral"
	}
	if original == "" {
		return out("clarify", "unknown", "Não ouvi ou recebi nenhuma mensagem. Pode tentar de novo?", "low", "empty_input")
	}
	if v, ok := reportedQuoteGuard(req); ok {
		return v
	}
	if v, ok := authorityPreflight(req); ok {
		return v
	}
	if v, ok := semanticInterpret(req); ok {
		// Semantic guardrails are evaluated before legacy lexical shortcuts.
		if v.Intent == "defer_work" && v.Decision == "propose_draft" {
			greetPref := e.Memory.Preference(ws, user, "greeting", audience)
			if greetPref == "omit" || (greetPref == "" && style == "direct") {
				v.Reply = strings.TrimPrefix(v.Reply, "Recebi sua mensagem. ")
			}
			blocked := e.Memory.Preference(ws, user, "avoid_phrase", audience)
			if blocked != "" && strings.Contains(norm(v.Reply), norm(blocked)) {
				return out("clarify", "defer_work", "Sua preferência impede esse rascunho. Como prefere formular?", "low", "blocked_phrase")
			}
		}
		return v
	}
	if v, ok := scopedInterpret(req); ok {
		// Scoped rules must not bypass the user's consented recipient preferences.
		// They are an interpretation preflight, not an alternative unpersonalized engine.
		if v.Intent == "defer_work" && v.Decision == "propose_draft" {
			greetPref := e.Memory.Preference(ws, user, "greeting", audience)
			if greetPref == "omit" || (greetPref == "" && style == "direct") {
				v.Reply = strings.TrimPrefix(v.Reply, "Recebi sua mensagem. ")
			}
			blocked := e.Memory.Preference(ws, user, "avoid_phrase", audience)
			if blocked != "" && strings.Contains(norm(v.Reply), norm(blocked)) {
				return out("clarify", "defer_work", "Sua preferência impede esse rascunho. Como prefere formular?", "low", "blocked_phrase")
			}
		}
		return v
	}
	if situation == "offered_game" && any(t, "ela falou", "ele falou", "ela disse", "ele disse") && (strings.Contains(original, "'") || strings.Contains(original, "\"")) {
		return out("clarify", "unknown", "Você está contando o que outra pessoa disse. O que quer fazer com essa informação?", "low", "third_party_quote", "no_self_rejection_inferred")
	}
	if situation == "offered_game" {
		if any(t, "a professora", "o professor", "minha amiga", "meu amigo", "minha irma", "meu irmao") && any(t, "disse que", "falou que", "contou que") {
			return out("clarify", "unknown", "Você está me contando a opinião de outra pessoa. E o que você achou?", "low", "reported_other_person", "no_rejection_inferred")
		}
		if any(t, confused...) {
			return out("reply_and_wait", "did_not_understand", "Acho que expliquei mal. Qual parte ficou confusa?", "bounded", "game_context", "explicit_confusion")
		}
		if any(t, rejection...) {
			return out("reply_and_wait", "reject_game", "Acho que eu falei besteira, você não gostou, né?", "bounded", "game_context", "rejection_cue")
		}
	}
	if any(t, ghosting...) && any(t, "o que significa", "qual o significado", "o que quer dizer", "define ghosting", "pode explicar o termo") {
		return out("reply_and_wait", "define_ghosting", "Ghosting é quando alguém interrompe a comunicação com outra pessoa sem explicar o motivo. Quer falar do que aconteceu ou era só a definição?", "bounded", "explicit_definition_question", "no_self_blame_inferred")
	}
	if strings.Contains(t, "sumiu") && any(t, "cachorro", "gato", "animal", "pet", "crianca", "filho", "filha") {
		return out("clarify", "unknown", "Não quero presumir do que se trata. Você pode me contar um pouco mais?", "low", "disappearance_not_ghosting")
	}
	if any(t, ghosting...) && any(t, selfBlame...) {
		opening := ""
		if style == "casual" || style == "profane" {
			opening = "Putz... "
		}
		return out("reply_and_wait", "ghosting_self_blame", opening+"Por que você acha que mereceu?", "bounded", "ghosting_cue", "self_blame_cue")
	}
	lit := any(t, literal...) && !any(t, "nao literalmente", "nao escreva literalmente", "sem falar literalmente")
	if lit && any(t, rude...) {
		m := quoted.FindStringSubmatch(original)
		if len(m) > 1 {
			x := out("propose_draft", "literal_message", m[1], "bounded", "literal_instruction", "quoted_payload")
			x.RequiresApproval = true
			return x
		}
		return out("clarify", "literal_message", "Você quer usar essas palavras literalmente? Qual é o texto exato que devo preparar?", "low", "literal_instruction", "missing_exact_text")
	}
	if any(t, techSymptoms...) && (any(t, techTriggers...) || ctx["domain"] == "auto_mechanics") {
		if any(t, "nao engasga", "nao falha", "nao morre", "nao rateia", "nao perde forca", "nao esta frio", "nao ta frio") {
			return out("clarify", "ambiguous_symptom", "Entendi uma negação do sintoma. O que exatamente acontece, e em quais condições?", "low", "negated_symptom", "no_diagnosis")
		}
		issue := "funcionamento irregular"
		if any(t, "engasg", "rateia", "rateand", "afog") {
			issue = "engasgos ou falhas"
		} else if strings.Contains(t, "perde forca") {
			issue = "perda de força"
		} else if strings.Contains(t, "morre") {
			issue = "desligamento inesperado"
		}
		conditions := []string{}
		if any(t, "piso fundo", "acelero", "aceler") {
			conditions = append(conditions, "durante a aceleração")
		}
		if any(t, "depois que esquenta", "quando esquenta", "depois de esquentar", "motor quente", "ta quente") && !any(t, "nao e depois que esquenta", "nao depois que esquenta", "nao quando esquenta") {
			conditions = append(conditions, "após o aquecimento do motor")
		}
		found := false
		for _, c := range conditions {
			if strings.Contains(c, "aquecimento") {
				found = true
			}
		}
		if strings.Contains(t, "frio") && !found {
			conditions = append(conditions, "com o motor frio")
		}
		if strings.Contains(t, "subida") {
			conditions = append(conditions, "em subida")
		}
		suffix := ""
		if len(conditions) > 0 {
			suffix = " " + strings.Join(conditions, " e ")
		}
		x := out("propose_draft", "technical_symptom", "Sintoma relatado: "+issue+suffix+". Isso descreve o problema; ainda não identifica a causa.", "bounded", "mechanical_symptom", "no_diagnosis")
		x.RequiresApproval = true
		return x
	}
	work := ctx["domain"] == "work" || any(t, "chefe", "boss", "gestor", "diretor", "email", "e-mail", "supervisor", "escreva pro", "responda ao", "responder ao")
	if work && any(t, "nao responda", "nao envie", "nao quero mandar", "nao quero responder", "nao quero enviar", "nao quero mandar mensagem", "so precisava desabafar", "to so desabafando") {
		return out("reply_and_wait", "do_not_draft", "Entendi. Não vou preparar nem enviar uma resposta agora.", "bounded", "explicit_no_draft_instruction", "yield_to_user")
	}
	action := any(t, "esper", "deixa", "deixar", "fica pra", "fica para", "vou", "preciso", "pode", "responda", "responder", "posso", "diga", "diz ", "fala ", "fale ", "escreva", "vejo ", "adiar", "retom", "depois", "nao hoje", "nao agora")
	if work && any(t, delay...) && action {
		if any(t, "nao responda", "nao envie", "nao quero mandar", "so precisava desabafar", "nao quero enviar") {
			return out("reply_and_wait", "do_not_draft", "Entendi. Não vou preparar nem enviar uma resposta agora.", "bounded", "explicit_no_draft_instruction", "yield_to_user")
		}
		if any(t, "segunda eu nao posso", "nao posso segunda", "nao posso na segunda", "nao prometa segunda", "nao prometo segunda", "nao quero prometer segunda", "nao diga segunda", "sem mencionar segunda", "segunda nao serve", "nao quero dizer segunda", "nao prometa data") {
			return out("clarify", "defer_work", "Você não quer assumir segunda-feira como compromisso. Que prazo, se algum, posso mencionar?", "low", "monday_commitment_conflict")
		}
		if strings.HasSuffix(original, "?") && !any(t, "escreva", "redija", "responda") {
			return out("clarify", "question_about_work", "Você está me perguntando sobre o prazo ou quer que eu prepare uma resposta para enviar?", "low", "interrogative", "draft_intent_unclear")
		}
		timing := ""
		reason := []string{}
		if strings.Contains(t, "segunda") {
			timing = "Vou retomar esse assunto na segunda-feira e retorno quando tiver uma avaliação."
			reason = []string{"work_context", "defer_intent", "explicit_monday"}
		} else if strings.Contains(t, "amanha") {
			timing = "Vou retomar esse assunto amanhã e retorno quando tiver uma avaliação."
			reason = []string{"work_context", "defer_intent", "explicit_tomorrow"}
		} else if any(t, "terca", "quarta", "quinta", "sexta") {
			return out("clarify", "defer_work", "Entendi que você quer adiar. Qual é o dia ou prazo exato que posso mencionar?", "low", "work_context", "day_not_supported")
		} else {
			timing = "Preciso de algum tempo para analisar a solicitação antes de responder com segurança. Retorno quando tiver uma posição."
			reason = []string{"work_context", "defer_intent", "no_date_invented"}
		}
		if any(t, "nao promete", "sem prometer", "nao diga segunda", "sem mencionar segunda", "segunda nao serve", "nao quero dizer segunda", "nao prometa data") && strings.Contains(t, "segunda") {
			return out("clarify", "defer_work", "Você mencionou segunda-feira, mas pediu para não assumir esse compromisso. Prefere deixar o prazo em aberto?", "low", "conflicting_timing_preferences")
		}
		greetPref := e.Memory.Preference(ws, user, "greeting", audience)
		greet := ""
		if greetPref == "include" || (greetPref == "" && style != "direct") {
			greet = "Recebi sua mensagem. "
		}
		draft := greet + timing
		blocked := e.Memory.Preference(ws, user, "avoid_phrase", audience)
		if blocked != "" && strings.Contains(norm(draft), norm(blocked)) {
			x := out("clarify", "defer_work", "Eu prepararia uma resposta de adiamento, mas ela contém uma expressão que você pediu para evitar: '"+blocked+"'. Como prefere formular?", "low", append(reason, "blocked_phrase")...)
			return x
		}
		x := out("propose_draft", "defer_work", draft, "bounded", reason...)
		if any(t, rude...) {
			x.Reasons = append(x.Reasons, "profane_input_not_copied")
		}
		x.Evidence = []string{"Explicit calendar reference preserved only when supported by input"}
		for _, m := range e.Memory.SimilarExamples(ws, user, original, audience, 3) {
			x.MatchedExampleIDs = append(x.MatchedExampleIDs, m.ID)
		}
		x.RequiresApproval = true
		return x
	}
	fallback := out("clarify", "unknown", "Ainda não entendi bem o que você quer fazer com isso. Pode me dar um pouco de contexto?", "low", "no_supported_intent", "no_guess")
	for _, m := range e.Memory.SimilarExamples(ws, user, original, audience, 3) {
		fallback.MatchedExampleIDs = append(fallback.MatchedExampleIDs, m.ID)
	}
	if e.Model != nil {
		label, err := e.Model.Classify(c, req)
		if err == nil {
			if x, ok := Admit(req, label); ok {
				if x.Intent == "defer_work" {
					greetPref := e.Memory.Preference(ws, user, "greeting", audience)
					if greetPref == "omit" || (greetPref == "" && style == "direct") {
						x.Reply = strings.TrimPrefix(x.Reply, "Recebi sua mensagem. ")
					}
					blocked := e.Memory.Preference(ws, user, "avoid_phrase", audience)
					if blocked != "" && strings.Contains(norm(x.Reply), norm(blocked)) {
						return out("clarify", "defer_work", "Sua preferência de linguagem impede esse rascunho. Que formulação prefere?", "low", "blocked_phrase")
					}
				}
				x.MatchedExampleIDs = fallback.MatchedExampleIDs
				return x
			}
			fallback.Reasons = append(fallback.Reasons, "model_label_not_admitted")
		} else {
			fallback.Reasons = append(fallback.Reasons, "model_unavailable_or_busy")
		}
	}
	return fallback
}
