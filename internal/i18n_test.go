package internal_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
)

// verbs lists the format verbs of a message, resolving explicit argument
// indexes (%[2]s) so translations may reorder arguments.
var reVerb = regexp.MustCompile(`%(\[(\d+)\])?[-+# 0]*[0-9.]*[a-zA-Z%]`)

func verbs(s string) []string {
	var out []string
	next := 1
	for _, m := range reVerb.FindAllStringSubmatch(s, -1) {
		v := m[0][len(m[0])-1:]
		if v == "%" {
			continue
		}
		idx := next
		if m[2] != "" {
			idx, _ = strconv.Atoi(m[2])
		}
		next = idx + 1
		out = append(out, strconv.Itoa(idx)+v)
	}
	return out
}

func sameVerbs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]int{}
	for _, v := range a {
		set[v]++
	}
	for _, v := range b {
		set[v]--
	}
	for _, n := range set {
		if n != 0 {
			return false
		}
	}
	return true
}

func hasJapanese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}

// Every Japanese string in the code is a message ID and needs an English
// translation with the same format verbs.
func TestEveryMessageHasEnglish(t *testing.T) {
	fset := token.NewFileSet()
	found := 0
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
			strings.Contains(filepath.ToSlash(path), "domain/i18n/") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			if !hasJapanese(s) {
				return true
			}
			found++
			en, ok := i18n.EnglishOf(s)
			if !ok {
				t.Errorf("%s: no English for %q", fset.Position(lit.Pos()), s)
				return true
			}
			if !sameVerbs(verbs(s), verbs(en)) {
				t.Errorf("%s: format verbs differ\n  ja %q %v\n  en %q %v", fset.Position(lit.Pos()), s, verbs(s), en, verbs(en))
			}
			if hasJapanese(en) {
				t.Errorf("English for %q still has Japanese: %q", s, en)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found < 100 {
		t.Fatalf("only %d messages found; is the test running in internal/?", found)
	}
}
