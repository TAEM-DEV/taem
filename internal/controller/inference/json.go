// Package inference provides shared utilities for inference controllers.
//
// extractJSON strips markdown code fences and whitespace from LLM responses
// before JSON parsing. Models frequently wrap JSON in ```json ... ``` blocks
// despite instructions to return raw JSON.
package inference

import (
	"strings"
)

// ExtractJSON strips markdown code fences from an LLM response to extract
// the raw JSON content. Handles:
//   - ```json\n{...}\n```
//   - ```\n{...}\n```
//   - Leading/trailing whitespace
//   - Multiple code blocks (takes the first one)
func ExtractJSON(content string) string {
	s := strings.TrimSpace(content)

	// Strip ```json or ``` prefix
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.Index(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.Index(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}

	s = strings.TrimSpace(s)

	// If it still doesn't start with { or [, try to find JSON in the string
	if len(s) > 0 && s[0] != '{' && s[0] != '[' {
		if idx := strings.Index(s, "{"); idx >= 0 {
			s = s[idx:]
			// Find matching close brace
			depth := 0
			for i, c := range s {
				if c == '{' {
					depth++
				} else if c == '}' {
					depth--
					if depth == 0 {
						s = s[:i+1]
						break
					}
				}
			}
		}
	}

	return s
}
