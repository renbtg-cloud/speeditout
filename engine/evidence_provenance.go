package engine

import (
	"errors"
	"strings"
)

// locateEvidence requires that a selected model excerpt occur exactly once.
// Multiple literal occurrences cannot be attributed to a particular speaker
// without an explicit span offset. This guard NEVER turns a model claim into
// authorization; it can only stop a potentially mistaken model override.
func locateEvidence(original, excerpt string) (start, end int, err error) {
	if excerpt == "" {
		return -1, -1, nil
	}
	i := strings.Index(original, excerpt)
	if i < 0 {
		return -1, -1, errors.New("unverified evidence string")
	}
	if strings.Contains(original[i+len(excerpt):], excerpt) {
		return -1, -1, errors.New("ambiguous repeated evidence")
	}
	return i, i + len(excerpt), nil
}

// stripQuotedText masks text inside straight/curly double quotes AND straight
// single quotes. This permits attribution checks to inspect commands *outside*
// quotes without accidentally treating an example's imperative as the user's.
// Unclosed quotes conservatively mask the remaining text.
func stripQuotedText(text string) string {
	var b strings.Builder
	var active rune
	for _, r := range text {
		if active == 0 {
			if r == '\'' || r == '"' || r == '“' || r == '‘' {
				active = r
				b.WriteRune(' ')
				continue
			}
			b.WriteRune(r)
			continue
		}
		if (active == '\'' && (r == '\'' || r == '’')) || (active == '"' && r == '"') || (active == '“' && r == '”') || (active == '‘' && r == '’') {
			active = 0
		}
		b.WriteRune(' ')
	}
	return b.String()
}

func overlappingEvidence(original, reported, direct string) bool {
	if reported == "" || direct == "" {
		return false
	}
	a, ae, ea := locateEvidence(original, reported)
	b, be, eb := locateEvidence(original, direct)
	if ea != nil || eb != nil {
		return true
	}
	return a < be && b < ae
}
