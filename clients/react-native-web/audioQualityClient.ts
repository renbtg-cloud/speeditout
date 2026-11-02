/** Transport contract shared by future React Native and web clients.
 * Experimental only: the current backend has no auth/TLS and must stay loopback-only.
 * Recording/capture and ASR are intentionally NOT implemented here.
 * Raw recordings are never sent to this endpoint.
 */
export type TranscriptionStatus = 'unavailable' | 'uncertain' | 'ready';
export type Platform = 'android' | 'ios' | 'web' | 'desktop' | 'unknown';
export type InputRoute = 'speakerphone' | 'handset' | 'headset' | 'computer_mic' | 'unknown';
export interface SignalMetrics {
  rms_dbfs: number;
  clipped_fraction: number;
  duration_seconds: number;
}
export interface AudioAssessment {
  signal?: SignalMetrics;
  transcript?: string;
  transcription_status: TranscriptionStatus;
  platform: Platform;
  input_route: InputRoute;
  attempts: number;
}
export interface AudioAdvice {
  level: 'retry' | 'confirm' | 'usable' | 'unknown';
  reasons: string[];
  message: string;
  suggestions?: string[];
  transcript: string;
  needs_user_confirmation: boolean;
  auto_send_allowed: false;
}
/** Use the actual authorized backend URL; physical mobile devices cannot access
 * the developer machine at 127.0.0.1. Never expose the unprotected research server. */
export async function assessAudio(baseUrl: string, request: AudioAssessment): Promise<AudioAdvice> {
  const response = await fetch(`${baseUrl.replace(/\/$/, '')}/v1/audio/assess`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  if (!response.ok) throw new Error(`Audio assessment failed (HTTP ${response.status})`);
  return await response.json() as AudioAdvice;
}
