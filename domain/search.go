package domain

import "strings"

// LineMatches reports whether line contains query. An empty query never
// matches. Matching is case-insensitive unless caseSensitive is set.
func LineMatches(line, query string, caseSensitive bool) bool {
	if query == "" {
		return false
	}
	if caseSensitive {
		return strings.Contains(line, query)
	}
	return strings.Contains(strings.ToLower(line), strings.ToLower(query))
}
