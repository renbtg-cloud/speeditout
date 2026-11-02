# Speedit out v0.9 — bounded semantic evidence frames

The core now represents *partial* evidence explicitly rather than allowing every substring match to decide what the user meant. Each accepted or blocked interpretation can return a `semantic` field containing:

- `clauses`: normalized text, hypothesized source (`user`, `third_party`, `uncertain`), discourse mode (`assert`, `question`, `report`, `hypothetical`), quoted flag and negation flag.
- `hypotheses`: a proposed intent/decision with supporting evidence or an explicit blocker.
- `resolution`: `accepted`, `blocked` or an unstated legacy fallback if the bounded semantic preflight cannot resolve the case.

**Warning:** these are heuristic evidence *labels*, not validated linguistic parses. In particular, simple discourse attribution and negation flags do not parse arbitrary Portuguese clauses or resolve complex coreference. Never treat `source` or `negated` as independently established facts.

## Control boundary

1. `semanticInterpret(req)` searches for evidence in a few bounded domains (game feedback, personal ghosting accounts, work email, mechanical descriptions).
2. Direct prohibitions, third-party reports and denied deadline commitments can block a draft.
3. Only a user-initiated drafting request with a supported date can produce certain work drafts; explicit approval remains required.
4. Previously approved memory preferences still modify or block the draft, scoped by workspace, user and recipient.
5. If the bounded preflight cannot justify a decision, the v0.8 scoped/legacy interpreter and optional **locally controlled** Qwen adapter remain available.

A third-party quotation is not user permission to send; literal profanity is copied to a *draft* only when directly requested and quoted. It is never sent automatically.

## A real design limitation

This release is **not** a general structured reasoning engine, despite the typed intermediate representation. Its candidate generation still uses regular expressions and a limited explicit vocabulary. The Qwen model does not yet propose multiple structured hypotheses for independent evidence-based ranking, nor is there genetic selection here. That is an *unimplemented research objective*, not an existing feature.

## Evidence handling

`cases/prospective-v09-frozen.json` (60 examples) was frozen before implementation, but subsequently inspected and used to fix bugs. It is development data now, not a final untouched benchmark. `cases/final-postfreeze-v09.json` (48 examples) was written **after the engine source was frozen**, then run once on v0.8 and v0.9. It was written by the developer (not independent participants); the labels are subjective to the agreed scenario taxonomy. We did not modify source based on its failures.
