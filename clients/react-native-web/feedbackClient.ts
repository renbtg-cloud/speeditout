/** Consent-gated feedback transport prototype for web / React Native.
 * Never submits raw audio or transcripts implicitly. A feedback note, when
 * supplied intentionally, is still stored as plaintext in the research log.
 * This API is loopback-only until real authentication/TLS exists.
 */
export type FeedbackCategory = 'misheard' | 'wrong_intent' | 'rejected_suggestion' |
  'insufficient_evidence' | 'uncertain' | 'helpful';
export interface FeedbackInput {
  workspace: string;
  user_id: string;
  category: FeedbackCategory;
  reference?: string;
  note?: string;
}
export interface SavedFeedback {saved: true; feedback_id: number; automatically_learned: false;}
export interface FeedbackDeletion {deleted_from_active_index: true; journal_tombstone_only: true;}
const known: ReadonlySet<string> = new Set([
  'misheard','wrong_intent','rejected_suggestion','insufficient_evidence','uncertain','helpful',
]);
async function postJSON<T>(baseUrl: string, path: string, data: object, signal?: AbortSignal): Promise<T> {
  const res = await fetch(`${baseUrl.replace(/\/$/, '')}${path}`, {
    method: 'POST', headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(data), signal,
  });
  if (!res.ok) throw new Error(`Feedback request failed (HTTP ${res.status})`);
  return await res.json() as T;
}
export async function submitFeedback(baseUrl: string, input: FeedbackInput, explicitConsent: boolean, signal?: AbortSignal): Promise<SavedFeedback> {
  if (!explicitConsent) throw new Error('Explicit feedback consent required');
  if (!input.workspace || !input.user_id || !known.has(input.category) ||
      (input.reference?.length ?? 0)>128 || (input.note?.length ?? 0)>512) {
    throw new Error('Invalid feedback input');
  }
  const data = await postJSON<unknown>(baseUrl, '/v1/feedback', {...input, consent: true}, signal);
  if (!data || typeof data!=='object') throw new Error('Invalid feedback confirmation');
  const x=data as Record<string,unknown>;
  if (x.saved!==true || !Number.isSafeInteger(x.feedback_id) || Number(x.feedback_id)<=0 || x.automatically_learned!==false)
    throw new Error('Invalid feedback confirmation');
  return x as unknown as SavedFeedback;
}
export async function deleteFeedback(baseUrl: string, workspace: string, userId: string, id: number, explicitConsent: boolean, signal?: AbortSignal): Promise<FeedbackDeletion> {
  if (!explicitConsent) throw new Error('Explicit feedback deletion consent required');
  if (!workspace || !userId || !Number.isSafeInteger(id) || id<=0) throw new Error('Invalid feedback deletion');
  const data = await postJSON<unknown>(baseUrl, '/v1/feedback/delete', {workspace,user_id:userId,id,consent:true}, signal);
  if (!data || typeof data!=='object') throw new Error('Invalid deletion confirmation');
  const x=data as Record<string,unknown>;
  if (x.deleted_from_active_index!==true || x.journal_tombstone_only!==true) throw new Error('Invalid deletion confirmation');
  return x as unknown as FeedbackDeletion;
}
