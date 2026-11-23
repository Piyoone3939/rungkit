package internal_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The onion rule: code may only depend on its own ring or rings inside it.
//
//	domain          <- nothing but the standard library
//	application     <- domain
//	infrastructure  <- application, domain   (adapters for application ports)
//	presentation    <- application, domain   (never infrastructure; the
//	                                           composition root in cmd/ wires it)
//
// Test files are not checked: tests may use adapters as fixture builders.
var allowed = map[string][]string{
	"domain":         {"domain"},
	"application":    {"domain", "application"},
	"infrastructure": {"domain", "application", "infrastructure"},
	"presentation":   {"domain", "application", "presentation"},
}

const module = "github.com/Piyoone3939/rungkit/internal/"

func TestOnionDependencies(t *testing.T) {
	fset := token.NewFileSet()
	checked := 0
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		layer := strings.Split(filepath.ToSlash(path), "/")[0]
		ok, known := allowed[layer]
		if !known {
			t.Errorf("%s is outside the known layers", path)
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		checked++
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			switch {
			case strings.HasPrefix(p, module):
				target := strings.Split(strings.TrimPrefix(p, module), "/")[0]
				if !contains(ok, target) {
					t.Errorf("%s (%s) must not import %s (%s)", path, layer, p, target)
				}
			case strings.HasPrefix(p, "github.com/Piyoone3939/rungkit/"):
				t.Errorf("%s imports %s from outside internal/", path, p)
			case strings.Contains(strings.Split(p, "/")[0], "."): // third-party module
				if layer == "domain" || layer == "application" {
					t.Errorf("%s (%s) must not depend on the third-party package %s", path, layer, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 20 {
		t.Fatalf("only %d files checked; is the test running in internal/?", checked)
	}
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if x == y {
			return true
		}
	}
	return false
}
