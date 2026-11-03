/** Research-only typed client for future React Native and web front ends.
 * The Go service is currently loopback-only and has no auth/TLS: this is NOT
 * a public mobile endpoint and cannot be safely exposed to the Internet.
 * No messages are ever sent on the user's behalf by this code.
 */
export type Decision = 'clarify' | 'reply_and_wait' | 'propose_draft';
export type ModelOutcome = 'not_needed' | 'unavailable' | 'not_proven' |
  'navigator_rejected' | 'navigator_admitted' | 'veto_applied';

export interface InterpretationRequest {
  workspace: string;
  user_id: string;
  text: string;
  context?: Record<string, string>;
}
export interface DecisionAudit {
  policy_version: string;
  approval_required: boolean;
  model_outcome: ModelOutcome;
  external_action_authorized: false;
}
export interface InterpretationResult {
  decision: Decision;
  intent: string;
  reply: string;
  confidence: string;
  reasons: string[];
  requires_approval: boolean;
  external_calls: number;
  decision_audit?: DecisionAudit;
}

const decisions: ReadonlySet<string> = new Set(['clarify', 'reply_and_wait', 'propose_draft']);
const modelOutcomes: ReadonlySet<string> = new Set([
  'not_needed', 'unavailable', 'not_proven', 'navigator_rejected',
  'navigator_admitted', 'veto_applied',
]);

/** Reject malformed or contradictory API responses instead of trusting a cast.
 * The client does not obtain permission to send anything from this endpoint.
 */
export function parseInterpretationResult(body: unknown): InterpretationResult {
  if (!body || typeof body !== 'object' || Array.isArray(body)) {
    throw new Error('Invalid interpretation response');
  }
  const x = body as Record<string, unknown>;
  if (typeof x.decision !== 'string' || !decisions.has(x.decision) ||
      typeof x.intent !== 'string' || typeof x.reply !== 'string' ||
      !x.reply.trim() || typeof x.confidence !== 'string' ||
      !Array.isArray(x.reasons) || !x.reasons.every(v => typeof v === 'string') ||
      typeof x.requires_approval !== 'boolean' || x.external_calls !== 0 ||
      !x.decision_audit || typeof x.decision_audit !== 'object' ||
      Array.isArray(x.decision_audit)) {
    throw new Error('Decision policy check failed');
  }
  const audit = x.decision_audit as Record<string, unknown>;
  if (typeof audit.policy_version !== 'string' || !audit.policy_version ||
      typeof audit.model_outcome !== 'string' || !modelOutcomes.has(audit.model_outcome) ||
      audit.external_action_authorized !== false ||
      audit.approval_required !== x.requires_approval ||
      (x.decision === 'propose_draft') !== x.requires_approval) {
    throw new Error('Decision policy check failed');
  }
  return x as unknown as InterpretationResult;
}

/** Even a valid proposed draft is just a proposal, NOT sending permission. */
export function canOfferDraft(result: InterpretationResult): boolean {
  return result.decision === 'propose_draft' &&
    result.requires_approval === true &&
    result.decision_audit?.approval_required === true &&
    result.decision_audit.external_action_authorized === false &&
    result.external_calls === 0;
}

export async function interpretText(
  baseUrl: string,
  request: InterpretationRequest,
  options?: {signal?: AbortSignal},
): Promise<InterpretationResult> {
  if (!request.user_id || !request.workspace) {
    throw new Error('workspace and user_id are required');
  }
  const response = await fetch(`${baseUrl.replace(/\/$/, '')}/v1/interpret`, {
    method: 'POST', headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(request), signal: options?.signal,
  });
  if (!response.ok) throw new Error(`Interpretation failed (HTTP ${response.status})`);
  return parseInterpretationResult(await response.json());
}
