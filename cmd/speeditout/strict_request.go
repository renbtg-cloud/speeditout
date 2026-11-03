package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// validateRequestJSON rejects ambiguous client input before any state-changing
// endpoint sees it. encoding/json otherwise silently accepts duplicate keys;
// for example a second "consent" property could override the first one.
// The limits apply to nested values as well as the top-level request.
func validateRequestJSON(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid UTF-8")
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	if err := requestJSONValue(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON content")
	}
	return nil
}

func requestJSONValue(d *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("request JSON too deeply nested")
	}
	t, err := d.Token()
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	marker, isMarker := t.(json.Delim)
	if !isMarker {
		return nil
	}
	switch marker {
	case '{':
		seen := make(map[string]struct{})
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return errors.New("JSON object key not a string")
			}
			if _, duplicate := seen[name]; duplicate {
				return fmt.Errorf("duplicate JSON field %q", name)
			}
			seen[name] = struct{}{}
			if len(seen) > 128 {
				return errors.New("too many JSON object fields")
			}
			if err := requestJSONValue(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("unclosed JSON object")
		}
	case '[':
		for n := 0; d.More(); n++ {
			if n >= 4096 {
				return errors.New("JSON array too long")
			}
			if err := requestJSONValue(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unclosed JSON array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}
