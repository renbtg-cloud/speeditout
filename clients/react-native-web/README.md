# Speedit out client transport contracts (research)

`audioQualityClient.ts` and `interpretationClient.ts` are TypeScript-only
transport prototypes, not an installed React Native or web application.
The Go backend currently has no authentication or TLS and binds to loopback;
**do not expose it to a phone or the public Internet**. A secure serving and
identity architecture is still required before mobile testing.

`canOfferDraft(...)` requires a valid server approval flag and policy audit,
but only returns whether the frontend may present a draft for explicit approval.
It does not send messages, and sending is not implemented.

The client accepts an optional `AbortSignal`; a cancellation must not alter
persisted user preferences or be interpreted as a reply from the user.

## Client verification (Node 22 and TypeScript 5.9)

```sh
tsc -p clients/react-native-web
node --test clients/react-native-web/tests/*.cjs
```

The parser rejects contradictory approval flags, model outcomes, nonzero
external-call claims and malformed responses. It **does not** provide a
user-approval flow or a network-authentication mechanism; it merely prevents
a malformed backend response from being treated as permission to show a draft.
