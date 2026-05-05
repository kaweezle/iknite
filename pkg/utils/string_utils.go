// cSpell: words jeremywhuff
package utils

import (
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Source - https://stackoverflow.com/a/76870660
// Posted by jeremywhuff
// Retrieved 2026-05-03, License - CC BY-SA 4.0

func CamelCase(s string, upperFirst bool) string {
	if s == "" {
		return s
	}

	// Remove all characters that are not alphanumeric or spaces or underscores or hyphens
	s = regexp.MustCompile("[^a-zA-Z0-9_ -]+").ReplaceAllString(s, "")

	// Replace all underscores and hyphens with spaces
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	// Title case s
	s = cases.Title(language.AmericanEnglish, cases.NoLower).String(s)

	// Remove all spaces
	s = strings.ReplaceAll(s, " ", "")

	// Lowercase the first letter if upperFirst is false
	if !upperFirst && s != "" {
		s = strings.ToLower(s[:1]) + s[1:]
	}

	return s
}
