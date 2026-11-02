# Audio adaptation proposal — v0.10 (not a mobile app)

## The product promise

When a person is not understood, Speedit out owns the communication problem.
Do not blame the user or claim to diagnose a hardware source without evidence.
Start with the actual ASR transcript if any, state uncertainty, then **yield the floor**.

Example after uncertain audio: "Posso ter entendido errado. O que captei foi: '...'. Acertei?"
Do not claim "tenho quase certeza" unless confidence is actually calibrated on relevant data.

## Backend contract

`POST /v1/audio/assess` (JSON, 64-KiB transport limit). Input:

```json
{
  "signal": {"rms_dbfs": -52, "clipped_fraction": 0, "duration_seconds": 4.0},
  "transcript": "Preciso falar disso segunda",
  "transcription_status": "uncertain",
  "platform": "android",
  "input_route": "speakerphone",
  "attempts": 2
}
```

Output includes `level` (`retry`/`confirm`/`usable`/`unknown`), `reasons`,
`message`, `suggestions`, `transcript`, `needs_user_confirmation`,
`auto_send_allowed` (always false).

`signal` is optional **device-provided measurements**, not a speech confidence score;
`transcription_status` comes from a future recognizer and is not inferred by the API.
Go `audioquality.MeasurePCM16LE` can calculate objective level/clipping metrics
for mono PCM16; `cmd/audioinspect -wav file.wav` supports mono PCM16 RIFF/WAVE.
**It cannot estimate SNR, intelligibility or whether the intended words were recognized.**

Thresholds (`RMS < -45 dBFS`, clipping >=1%, duration < 250 ms) are
provisional **heuristics**. They need device and language-specific calibration;
these simple measurements may have false positives and negatives.

Examples of conditional guidance only after repeated difficulty:
- Known speakerphone route: "Se estiver usando o viva-voz, experimentar o modo telefone pode ajudar. Quer tentar?"
- Known computer mic: "Falar um pouco mais perto do microfone do computador talvez ajude."
- Unknown route: "Se você tiver um headset com microfone perto da boca, pode valer a pena experimentar."
- No audio clue: avoid speculating about mic problems; offer repetition, text correction, or typing.

Do not produce advice unprompted in the first exchange. Don't infer accessories, hardware quality,
medical/communication conditions, demographics, or a user's spoken words from signal level.
No audio capture, upload, storage or automatic sending has been implemented.

## Future application integration

React Native: evaluate Expo Audio microphone permissions/recording and native DSP route if truly required; the prototype has no RN microphone capture. Web: standard microphone permissions and device audio APIs; maintain a shared transcript-correction flow. TypeScript transport types in `clients/react-native-web/audioQualityClient.ts`.

No API of a particular device guarantees the same acoustic route or signal processing capabilities. The OS may already have noise suppression or echo cancellation enabled. Do not blindly stack DSP filters.

## Open-source enhancement candidates (not integrated)

- WebRTC Audio Processing Module: echo cancellation, noise suppression and automatic gain control (C++); can operate standalone. https://webrtc.googlesource.com/src/+/refs/heads/main/modules/audio_processing/g3doc/audio_processing_module.md
- Xiph RNNoise: RNN-based speech noise suppression; evaluate embedded resource usage, weights redistribution rights and real ASR error rates before shipping. https://github.com/xiph/rnnoise
- DeepFilterNet: Rust-based speech enhancement; source MIT/Apache-2.0; verify model redistribution requirements separately. https://github.com/Rikorose/DeepFilterNet

## Test promotion criteria

Record opt-in human utterances covering workshop noise, traffic, music, distant phone,
Brazilian regional accents, slang, children's speech and assistive communication.
Compare untreated/enhanced on **the same ASR** and blinded independent transcriptions.
Measure word/character error rate, named entities, intent fidelity, false deletions,
latency, energy, battery and memory, across devices. Do not optimize solely for audio SNR.
Avoid requiring users to provide private audio for research; use consent, retention controls,
and local-first alternatives when feasible. Human dignity and child safety are mandatory.

## Research-stage limitations

No ASR or neural denoiser was installed. FFmpeg `afftdn` below is a separate offline smoke test.
The Go HTTP server has NO authentication/TLS: research loopback only, synthetic data only.
