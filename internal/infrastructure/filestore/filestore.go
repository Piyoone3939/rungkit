// Package filestore loads and saves programs on the local file system. The
// file extension picks the format: .json uses JSON, .xml PLCopen XML,
// anything else .ldt text.
// The path "-" reads from standard input.
//
// With Root set (the MCP server does this), every path must stay inside Root:
// relative paths are resolved there, symbolic links that lead outside are
// refused, and "-" is refused because stdin carries the MCP protocol.
package filestore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Piyoone3939/rungkit/internal/application/workspace"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/jsoncodec"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/plcopen"
)

// Store implements workspace.Files.
type Store struct {
	// Stdin is read for the path "-" (os.Stdin when nil).
	Stdin io.Reader
	// Root confines all paths to this directory when set.
	Root string
}

// MaxFileSize is the largest file read (16 MiB).
const MaxFileSize = 16 << 20

// resolve applies the Root confinement to path.
func (s Store) resolve(path string) (string, error) {
	if s.Root == "" {
		return path, nil
	}
	if path == "-" {
		return "", i18n.Errorf("標準入力（-）はここでは使えません")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	p = filepath.Clean(p)
	// Follow links on the part of the path that exists, so a link inside the
	// root cannot point outside it.
	real := p
	if r, err := filepath.EvalSymlinks(p); err == nil {
		real = r
	} else if r, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		real = filepath.Join(r, filepath.Base(p))
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", i18n.Errorf("%s は作業フォルダ（%s）の外にあるので使えません", path, root)
	}
	return p, nil
}

func (s Store) read(path string) ([]byte, error) {
	path, err := s.resolve(path)
	if err != nil {
		return nil, err
	}
	if path == "-" {
		in := s.Stdin
		if in == nil {
			in = os.Stdin
		}
		return io.ReadAll(io.LimitReader(in, MaxFileSize+1))
	}
	if st, err := os.Stat(path); err == nil && st.Size() > MaxFileSize {
		return nil, i18n.Errorf("%s は大きすぎます（%d バイトまで）", path, MaxFileSize)
	}
	return os.ReadFile(path)
}

func ext(path string) string { return strings.ToLower(filepath.Ext(path)) }

// LoadProgram reads a .ldt, .json or PLCopen .xml program.
func (s Store) LoadProgram(path string) (*ladder.Program, workspace.Positions, error) {
	data, err := s.read(path)
	if err != nil {
		return nil, nil, err
	}
	switch ext(path) {
	case ".json":
		p, err := jsoncodec.Unmarshal(data)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		return p, nil, nil
	case ".xml":
		p, err := plcopen.Import(data)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		return p, nil, nil
	}
	p, sm, err := ldt.Parse(string(data))
	if err != nil {
		return nil, nil, fmt.Errorf("%s:%w", path, err)
	}
	pos := workspace.Positions{}
	for id, at := range sm.Rungs {
		pos[id] = at.String()
	}
	for id, at := range sm.Elems {
		pos[id] = at.String()
	}
	return p, pos, nil
}

// SaveProgram writes a .ldt, .json or PLCopen .xml program. Other extensions are refused so
// a mistyped path cannot overwrite an unrelated file.
func (s Store) SaveProgram(path string, p *ladder.Program) error {
	var out []byte
	switch ext(path) {
	case ".ldt":
		text, err := ldt.Format(p)
		if err != nil {
			return err
		}
		// Never write a file rungkit itself could not read back.
		if _, _, err := ldt.Parse(text); err != nil {
			return i18n.Errorf("保存すると読み戻せない内容になるので保存しません: %w", err)
		}
		out = []byte(text)
	case ".json":
		b, err := jsoncodec.Marshal(p)
		if err != nil {
			return err
		}
		out = append(b, '\n')
	case ".xml":
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		text, err := plcopen.Export(p, name, time.Now())
		if err != nil {
			return err
		}
		out = []byte(text)
	default:
		return i18n.Errorf("保存先は .ldt / .json / .xml（PLCopen）のファイルにします（%s）", path)
	}
	target, err := s.resolve(path)
	if err != nil {
		return err
	}
	return os.WriteFile(target, out, 0o644)
}

// ReadText reads a text file.
func (s Store) ReadText(path string) (string, error) {
	b, err := s.read(path)
	return string(b), err
}
