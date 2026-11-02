package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// validateModelJSON rejects ambiguous JSON before binding it to structs.
// encoding/json deliberately accepts duplicate object keys (last value wins),
// a poor property for security-relevant authority adjudication. Walk the
// structure token by token to reject duplicates, unbounded depth and trailing
// data, including within nested arrays/objects.
func validateModelJSON(raw string, maxBytes int) error {
	if len(raw) > maxBytes || !utf8.ValidString(raw) {
		return errors.New("invalid model JSON size or encoding")
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	if err := modelJSONValue(dec, 0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing model JSON data")
	}
	return nil
}

func modelJSONValue(dec *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("model JSON nesting too deep")
	}
	t, err := dec.Token()
	if err != nil {
		return fmt.Errorf("invalid model JSON: %w", err)
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return fmt.Errorf("model JSON object key: %w", err)
			}
			name, ok := key.(string)
			if !ok {
				return errors.New("model JSON key is not a string")
			}
			if seen[name] {
				return errors.New("model JSON has a duplicate object key")
			}
			seen[name] = true
			if err := modelJSONValue(dec, depth+1); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("model JSON unclosed object")
		}
	case '[':
		for dec.More() {
			if err := modelJSONValue(dec, depth+1); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("model JSON unclosed array")
		}
	default:
		return errors.New("invalid model JSON delimiter")
	}
	return nil
}
