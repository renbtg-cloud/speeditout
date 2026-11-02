package engine

import (
	"encoding/json"
	"errors"
	"io"
)

const modelEnvelopeLimit = 16 << 10

// decodeModelEnvelope reads the ENTIRE response and validates it rather than
// accepting the first valid JSON prefix from an untrusted local service.
// Even local-only model inference must not be treated as a trusted actor.
func decodeModelEnvelope(body io.Reader, result interface{}) error {
	b, err := io.ReadAll(io.LimitReader(body, modelEnvelopeLimit+1))
	if err != nil {
		return err
	}
	if len(b) > modelEnvelopeLimit {
		return errors.New("model response exceeds envelope limit")
	}
	if err := validateModelJSON(string(b), modelEnvelopeLimit); err != nil {
		return err
	}
	return json.Unmarshal(b, result)
}
