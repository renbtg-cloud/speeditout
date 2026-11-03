package engine

// DecisionAudit records bounded, non-sensitive policy facts. It is not the
// model's narrative and not an assertion that an unverified theory is true.
// It reports whether a draft is gated, never grants permission to send.
type DecisionAudit struct {
	PolicyVersion            string `json:"policy_version"`
	ApprovalRequired         bool   `json:"approval_required"`
	ModelOutcome             string `json:"model_outcome"`
	ExternalActionAuthorized bool   `json:"external_action_authorized"`
}

func detectModelOutcome(reasons []string) string {
	for _, r := range reasons {
		switch r {
		case "authority_evidence_v013", "authority_qwen_veto":
			return "veto_applied"
		case "local_model_label":
			return "navigator_admitted"
		}
	}
	for _, r := range reasons {
		switch r {
		case "authority_evidence_unavailable", "authority_model_unavailable", "model_unavailable_or_busy":
			return "unavailable"
		case "authority_evidence_unproven", "authority_evidence_independent_request", "authority_model_no_veto":
			return "not_proven"
		case "model_label_not_admitted":
			return "navigator_rejected"
		}
	}
	return "not_needed"
}

func enforceDecisionPolicy(o Outcome) Outcome {
	violation := ""
	switch o.Decision {
	case "propose_draft":
		if !o.RequiresApproval {
			violation = "draft_without_approval"
		}
	case "clarify", "reply_and_wait":
		if o.RequiresApproval {
			violation = "unexpected_approval_flag"
		}
	default:
		violation = "unsupported_decision"
	}
	if o.Reply == "" {
		violation = "missing_reply"
	}
	if o.ExternalCalls != 0 {
		violation = "unexpected_external_action"
	}
	if violation != "" {
		o = out("clarify", "unknown", "Antes de agir, preciso confirmar sua intenção.", "low", "decision_policy_blocked", violation)
	}
	o.DecisionAudit = &DecisionAudit{
		PolicyVersion: "0.16", ApprovalRequired: o.RequiresApproval,
		ModelOutcome:             detectModelOutcome(o.Reasons),
		ExternalActionAuthorized: false,
	}
	return o
}
