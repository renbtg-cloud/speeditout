package engine

import (
	"regexp"
	"strings"
)

// v0.7 bounded interpretation preflight. We deliberately distinguish the
// source of a claim, a direct request and a negative assertion BEFORE an
// old substring matching rule can turn an attributed statement into action.
// This is not a general semantic parser, nor a model of a human mind.
var (
	thirdPartyReport = regexp.MustCompile(`\b(?:(?:minh[ao]s?|meus?|minhas?) (?:amig[ao]s?|irm(?:aos?|as?)|filh[ao]s?|mae|pai|avo|primo|prima|colega|gerente|chefe|gestor|supervisor|tio)|(?:um|o|a) (?:menino|menina|colega|mecanico|cara)|ele|ela|a chefia|o diretor|o supervisor)\b.{0,75}\b(?:disse|diz|falou|contou|acha|achou|pensa|pensou|pediu|escreveu|relatou|afirmou|tem|ficou|se culpa|sumiu)\b`)
	firstOwnCounter  = regexp.MustCompile(`\b(?:mas|so que|porem) eu\b`)
	notNow           = regexp.MustCompile(`\b(?:agora nao|nao mais|nao acontece mais|depois (?:melhorou|parou)|mas (?:ja )?passou)\b`)
	noDeadline       = regexp.MustCompile(`\b(?:nao (?:vou |quero |posso )?(?:prometer|garantir|assumir|me comprometer)(?: que resolvo)?(?: na| pra| para)? segunda|sem (?:inventar|prometer|assumir|mencionar) (?:outro )?prazo|sem (?:inventar|definir) (?:outra )?data|nao (?:me )?compromet[oa] com segunda|nao posso me comprometer com segunda|nao consigo me comprometer com segunda|nao posso segunda|segunda nao serve|nao diga segunda|nao prometa data|sem prometer que resolvo na segunda|sem inventar outro prazo)\b`)
	negMotor         = regexp.MustCompile(`\b(?:motor|carro|veiculo)? ?nao (?:esta |ta |estava )?(?:engasgando|engasga|falha|falhando|morre|rateia|perde (?:forca|potencia))\b`)
)

func scopedInterpret(req Request) (Outcome, bool) {
	t := norm(strings.TrimSpace(req.Text))
	ctx := req.Context
	situation, domain := ctx["situation"], ctx["domain"]
	work := domain == "work" || any(t, "chefe", "gerente", "diretor", "supervisor", "email", "e-mail", "chefia", "boss")
	third := thirdPartyReport.MatchString(t)
	own := firstOwnCounter.MatchString(t)

	if situation == "offered_game" {
		// Intent evidence about the current user outranks incidental keywords in quotations.
		if any(t, "nao to entendendo", "nao estou entendendo", "me perdi nas regras", "me perdi nessas regras", "explica novamente", "nao consigo acompanhar a explicacao") {
			return out("reply_and_wait", "did_not_understand", "Acho que expliquei mal. Qual parte ficou confusa?", "bounded", "explicit_confusion"), true
		}
		if any(t, "nao gostei do som", "nao gostei so da", "nao gostei apenas da", "nao gostei da fase", "nao gostei da etapa") {
			return out("clarify", "unknown", "Entendi que uma parte não agradou. Qual parte quer mudar?", "low", "component_not_whole_game"), true
		}
		if third && any(t, "mas eu ainda quero jogar", "mas eu quero jogar", "mas eu gostei", "mas eu adorei", "mas eu acho legal") {
			return out("clarify", "unknown", "Você está relatando opiniões diferentes; qual sua preferência para continuar?", "low", "user_positive_over_reported_negative"), true
		}
		if any(t, "jogo e inutil", "quero outra coisa", "prefiro outra coisa") && !third {
			return out("reply_and_wait", "reject_game", "Acho que eu falei besteira, você não gostou, né?", "bounded", "direct_rejection"), true
		}
		if any(t, "que que e isso", "o que e isso") {
			return out("reply_and_wait", "did_not_understand", "Acho que expliquei mal. Qual parte ficou confusa?", "bounded", "explicit_confusion"), true
		}
		// Questioning whether others might dislike a game is not rejecting it.
		// An interrogative in the middle of the utterance also counts.
		if strings.Contains(t, "?") && !any(t, "eu nao gostei", "nao quero brincar", "eu detestei") {
			return out("clarify", "unknown", "Você está perguntando ou contando uma impressão. O que você achou?", "low", "interrogative_not_rejection"), true
		}
		// Reported dissatisfaction or incomprehension belongs to someone else.
		// A favorable first-person opinion takes precedence over an attributed rejection.
		if third && (!own || any(t, "eu gostei", "eu achei legal", "eu achei boa", "eu adoro", "eu entendi", "eu achei massa")) {
			return out("clarify", "unknown", "Entendi que você está descrevendo a reação de outra pessoa. E a sua?", "low", "attribution_guard"), true
		}
		if any(t, "gostei do jogo", "gostei da brincadeira", "gostei sim", "eu gostei", "eu adoro", "eu entendi", "entendi sim") {
			if !any(t, "mas eu nao gostei", "so que eu nao gostei") {
				return out("clarify", "unknown", "Entendi. O que você gostaria de fazer agora?", "low", "positive_or_mixed_game_sentiment"), true
			}
		}
		if any(t, "nao gostei das regras", "nao gostei da trilha", "nao gostei da musica") {
			return out("clarify", "unknown", "Você não gostou de uma parte do jogo. Quer me dizer o que mudaria?", "low", "specific_component_feedback"), true
		}
		if any(t, "nao to conseguindo acompanhar", "nao consigo acompanhar", "nao consigo seguir", "nao consegui entender", "nao consigo entender", "estou perdido", "to perdido", "explica de outro jeito") {
			return out("reply_and_wait", "did_not_understand", "Acho que expliquei mal. Qual parte ficou confusa?", "bounded", "first_person_confusion"), true
		}
		if any(t, "previsivel", "repetitiv", "sem surpresa", "nao e divertido", "nao ta me divertindo", "nao me diverte", "nao quero brincar", "nao me interessa", "sem diversao") && !third {
			return out("reply_and_wait", "reject_game", "Acho que eu falei besteira, você não gostou, né?", "bounded", "self_rejection"), true
		}
	}

	// Guard personal relationship talk against second-hand reports.
	firstPersonInvolved := any(t, "sem me responder", "comigo", "me respondeu", "me deu", "me bloqueou", "eu fiz", "eu errei", "eu falei", "eu magoei", "magoei ela", "magoei ele", "fui cruel com ela", "eu fui babaca", "desde entao ela nao respondeu", "da minha vida")
	// Third-person subject does not imply third-party story: "ela não me respondeu" is my own experience.
	attributedOnly := third && !own && !firstPersonInvolved
	if attributedOnly && any(t, "vacuo", "ghost", "parou de responder", "parou de falar", "sumiu", "se culpa", "culpa dela", "culpa dele") {
		return out("clarify", "unknown", "Preciso de mais contexto: você está contando o que aconteceu com outra pessoa?", "low", "relationship_attribution_guard"), true
	}
	relationship := any(t, "me deu vacuo", "levei vacuo", "levei um vacuo", "me deixaram no vacuo", "me deixou no vacuo", "fiquei no vacuo", "fiquei falando sozinho", "fiquei falando sozinha", "me bloqueou", "parou de falar comigo", "parou de me responder", "sem me responder", "nao me respondeu", "sumiu de mim", "me deu ghosting", "levei ghosting", "ela desapareceu depois que eu", "ele desapareceu depois que eu", "ele sumiu", "ela sumiu", "ela ficou dias sem me responder", "me deu um gelo", "me ignorou", "nao me responde", "ela nao respondeu", "ele nao respondeu", "desapareceu da minha vida")
	blame := any(t, "acho que mereci", "culpa minha", "minha culpa", "fui cruel", "fui babaca", "fui escrota", "fui escroto", "eu vacilei", "eu magoei", "acho que magoei", "magoei ela", "magoei ele", "fiz uma besteira", "fiz merda", "eu estraguei", "acho que eu estraguei", "acho que fiz", "talvez tenha sido culpa minha", "pode ter sido culpa minha", "fiz algo horrivel", "eu errei", "fui um idiota", "fui cruel com ela", "acho que estraguei tudo", "acho que errei feio", "eu fui babaca", "talvez eu tenha estragado tudo")
	if relationship && blame && !attributedOnly {
		return out("reply_and_wait", "ghosting_self_blame", "Por que você acha que a culpa pode ter sido sua?", "bounded", "self_attribution", "self_blame"), true
	}
	if relationship && !blame && any(t, "nem ligo", "to de boa", "nao me importo") {
		return out("clarify", "unknown", "Entendi. Quer conversar mais sobre isso?", "low", "no_self_blame"), true
	}

	if work {
		// A question about another person's decision is not itself an order to draft.
		if (domain == "work" || any(t, "prazo", "segunda", "amanha", "responder", "resposta")) && strings.Contains(t, "?") && any(t, "voce acha", "o que voce acha", "acha razoavel", "eu devia") {
			return out("clarify", "question_about_work", "Você quer conversar sobre essa decisão ou preparar uma resposta?", "low", "question_not_drafting"), true
		}
		// A third-party instruction about *their* deadline is not a request from the user.
		if third && any(t, "pediu", "disse", "diz", "escreveu", "falou") && !any(t, "prepara", "prepare", "redija", "escreva para", "escreva pro", "escreva ao", "avisa", "avise", "responda", "manda", "mande") {
			return out("clarify", "unknown", "Isso é o que outra pessoa disse. O que você gostaria que eu fizesse?", "low", "third_party_work_attribution"), true
		}
		if !any(t, "nao escreva literalmente", "nao redija literalmente") && any(t, "nao faca email", "nao faca e-mail", "nao faca nenhum email", "nao escreva", "nao redija", "nao prepare", "nao quero que voce escreva", "nao quero que voce redija", "nao quero mandar", "nao quero responder", "nao quero enviar", "nao responda", "nao envie", "nao faca resposta", "so preciso conversar", "so quero conversar", "so quero desabafar", "so queria desabafar", "so estou desabafando", "estou apenas desabafando", "eu mesmo resolvo", "deixa quieto") {
			return out("reply_and_wait", "do_not_draft", "Entendi. Não vou preparar nem enviar uma resposta agora.", "bounded", "user_no_draft"), true
		}
		if (domain == "work" || any(t, "responder", "resposta", "prazo", "hoje", "amanha", "segunda")) && any(t, "voce acha que", "voce acha razoavel", "eu devia", "posso deixar", "o que voce acha") && strings.HasSuffix(strings.TrimSpace(t), "?") {
			return out("clarify", "question_about_work", "Você quer conversar sobre a decisão ou preparar uma resposta?", "low", "question_not_drafting"), true
		}
		wantsDraft := any(t, "prepara", "prepare", "redija", "escreva para", "escreva pro", "escreva ao", "escreva.", "escreva", "avisa", "avise", "responda", "manda", "mande", "diga ao", "fala pro", "diz pro", "fale ao")
		if wantsDraft && (noDeadline.MatchString(t) || any(t, "sem prometer nenhuma data", "sem prometer data", "sem inventar data", "redija sem data", "redija sem prazo", "nao me faca assumir prazo")) {
			return out("clarify", "defer_work", "Entendi que você quer adiar, mas não quer que eu invente ou prometa uma data. Que formulação prefere?", "low", "deadline_not_authorized"), true
		}
		if wantsDraft && (any(t, "segunda", "amanha", "deixar pra", "deixo pra", "retomo", "retorno", "adiar", "nao hoje")) {
			if strings.Contains(t, "amanha") && !strings.Contains(t, "segunda") {
				x := out("propose_draft", "defer_work", "Recebi sua mensagem. Vou retomar esse assunto amanhã e retorno quando tiver uma avaliação.", "bounded", "explicit_draft", "explicit_tomorrow")
				x.RequiresApproval = true
				return x, true
			}
			if strings.Contains(t, "segunda") {
				x := out("propose_draft", "defer_work", "Recebi sua mensagem. Vou retomar esse assunto na segunda-feira e retorno quando tiver uma avaliação.", "bounded", "explicit_draft", "explicit_monday")
				x.RequiresApproval = true
				return x, true
			}
		}
	}

	if domain == "auto_mechanics" || any(t, "motor", "carro", "veiculo", "escapamento") {
		if third && !own && any(t, "disse", "diz", "falou", "relatou", "afirmou", "acha", "achou") {
			return out("clarify", "unknown", "Esse sintoma foi relatado por outra pessoa. Você conseguiu observá-lo diretamente?", "low", "mechanic_reported_source"), true
		}
		if any(t, "nao existe falha", "estou so descrevendo o relato", "so descrevendo o relato") {
			return out("clarify", "unknown", "Preciso de contexto: isso é um sintoma observado ou apenas um relato?", "low", "negated_observation"), true
		}
		if negMotor.MatchString(t) || (notNow.MatchString(t) && any(t, "motor", "falhava", "engasgava", "rateava")) || (any(t, "nao faz mais isso", "parou de engasgar", "parou de falhar", "nao acontece mais") && any(t, "motor", "carro", "falha", "engasg", "rateav")) {
			return out("clarify", "ambiguous_symptom", "Entendi a negação ou correção de um sintoma anterior. O que acontece agora?", "low", "negated_or_resolved_symptom"), true
		}
		if any(t, "perde potencia", "perder potencia", "perda de potencia", "perde forca", "perda de forca", "sem reacao", "demora a responder", "ruido metalico", "barulho metalico", "cabecadas", "da cabecada", "da umas cabecadas", "em alta", "piso e ele fica", "motor comeca a perder", "motor falha") && !third {
			x := out("propose_draft", "technical_symptom", "Sintoma relatado: "+strings.TrimSpace(req.Text)+". Esta é apenas uma descrição; não identifica a causa.", "bounded", "direct_technical_symptom", "no_diagnosis")
			x.RequiresApproval = true
			return x, true
		}
	}
	return Outcome{}, false
}
