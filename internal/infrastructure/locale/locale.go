// Package locale finds the user's language: RUNGKIT_LANG first, then the
// POSIX locale variables, then the operating system setting (Windows).
package locale

import (
	"os"
	"strings"
)

// Detect returns "ja" or "en".
func Detect() string {
	if l := os.Getenv("RUNGKIT_LANG"); l != "" {
		return normalize(l)
	}
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if l := os.Getenv(v); l != "" && l != "C" && l != "POSIX" {
			return normalize(l)
		}
	}
	if l := systemLocale(); l != "" {
		return normalize(l)
	}
	return "en"
}

func normalize(l string) string {
	if strings.HasPrefix(strings.ToLower(l), "ja") {
		return "ja"
	}
	return "en"
}
