package filestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const prog = "profile melsec-iqr\n\nrung {\n  X0 -> OUT(Y0)\n}\n"

func TestRootConfinement(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "work")
	outside := filepath.Join(base, "secret")
	for _, d := range []string{root, filepath.Join(root, "sub"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{filepath.Join(root, "a.ldt"), filepath.Join(root, "sub", "b.ldt"), filepath.Join(outside, "c.ldt")} {
		if err := os.WriteFile(f, []byte(prog), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := Store{Root: root}

	// inside: relative and absolute paths work
	for _, p := range []string{"a.ldt", "sub/b.ldt", filepath.Join(root, "a.ldt"), "sub/../a.ldt"} {
		if _, _, err := s.LoadProgram(p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	p, _, _ := s.LoadProgram("a.ldt")
	if err := s.SaveProgram("sub/new.ldt", p); err != nil {
		t.Errorf("save inside: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "sub", "new.ldt")); err != nil {
		t.Errorf("saved relative to root: %v", err)
	}

	// outside: refused for reading and writing
	for _, path := range []string{"../secret/c.ldt", filepath.Join(outside, "c.ldt"), "-"} {
		if _, _, err := s.LoadProgram(path); err == nil {
			t.Errorf("load %s: want error", path)
		}
		if _, err := s.ReadText(path); err == nil {
			t.Errorf("read %s: want error", path)
		}
	}
	if err := s.SaveProgram("../secret/x.ldt", p); err == nil {
		t.Error("save outside: want error")
	}
	if _, err := os.Stat(filepath.Join(outside, "x.ldt")); err == nil {
		t.Error("file was written outside the root")
	}

	// a link inside the root that points outside is refused
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err == nil {
		if _, _, err := s.LoadProgram("escape/c.ldt"); err == nil || !strings.Contains(err.Error(), "outside") && !strings.Contains(err.Error(), "外") {
			t.Errorf("symlink escape: %v", err)
		}
	} else {
		t.Logf("symlinks unavailable here (%v); skipped", err)
	}
}

func TestNoRootAllowsAnyPath(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.ldt")
	if err := os.WriteFile(f, []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Store{}).LoadProgram(f); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Store{Stdin: strings.NewReader(prog)}).LoadProgram("-"); err != nil {
		t.Fatal(err)
	}
}

func TestTooLarge(t *testing.T) {
	f := filepath.Join(t.TempDir(), "big.ldt")
	if err := os.WriteFile(f, make([]byte, MaxFileSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{}).ReadText(f); err == nil {
		t.Fatal("want error")
	}
}

func TestSaveRefusesUnreadable(t *testing.T) {
	dir := t.TempDir()
	p, _, err := (Store{}).LoadProgram(writeTemp(t, dir, prog))
	if err != nil {
		t.Fatal(err)
	}
	p.Rungs[0].Label = "運転"
	path := filepath.Join(dir, "out.ldt")
	if err := (Store{}).SaveProgram(path, p); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("an unreadable file was written")
	}
}

func writeTemp(t *testing.T, dir, text string) string {
	t.Helper()
	f := filepath.Join(dir, "in.ldt")
	if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}
