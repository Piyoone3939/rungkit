// Package i18n translates user-facing messages. It is a shared kernel used by
// every layer and depends only on the standard library.
//
// Messages are written in Japanese in the source (gettext style: the
// Japanese text is the message ID) and translated through the English
// catalog in catalog_en.go. A test checks that every Japanese string literal
// in the code has an English entry with the same format verbs.
package i18n

import (
	"fmt"
	"sync/atomic"
)

// Japanese and English are the supported languages.
const (
	Japanese = "ja"
	English  = "en"
)

var lang atomic.Value

func init() { lang.Store(Japanese) }

// SetLanguage selects the language for messages. Anything but "ja" selects
// English.
func SetLanguage(l string) {
	if l != Japanese {
		l = English
	}
	lang.Store(l)
}

// Language returns the selected language.
func Language() string { return lang.Load().(string) }

// translate returns the format string for the selected language.
func translate(format string) string {
	if Language() == English {
		if en, ok := english[format]; ok {
			return en
		}
	}
	return format
}

// T formats a message in the selected language.
func T(format string, args ...any) string {
	f := translate(format)
	if len(args) == 0 {
		return f
	}
	return fmt.Sprintf(f, args...)
}

// Errorf is fmt.Errorf with a translated format (%w works as usual).
func Errorf(format string, args ...any) error {
	return fmt.Errorf(translate(format), args...)
}

// English returns the English translation of a message ID and whether it
// exists. Used by tests to check the catalog.
func EnglishOf(id string) (string, bool) {
	en, ok := english[id]
	return en, ok
}
