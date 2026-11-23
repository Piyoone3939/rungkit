// Package filestore loads and saves programs on the local file system. The
// file extension picks the format: .json uses JSON, .xml PLCopen XML,
// anything else .ldt text.
// The path "-" reads from standard input.
package filestore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rungkit/internal/application/workspace"
	"rungkit/internal/domain/ladder"
	"rungkit/internal/infrastructure/jsoncodec"
	"rungkit/internal/infrastructure/ldt"
	"rungkit/internal/infrastructure/plcopen"
)

// Store implements workspace.Files.
type Store struct {
	// Stdin is read for the path "-" (os.Stdin when nil).
	Stdin io.Reader
}

func (s Store) read(path string) ([]byte, error) {
	if path == "-" {
		in := s.Stdin
		if in == nil {
			in = os.Stdin
		}
		return io.ReadAll(in)
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
		return fmt.Errorf("保存先は .ldt / .json / .xml（PLCopen）のファイルにします（%s）", path)
	}
	return os.WriteFile(path, out, 0o644)
}

// ReadText reads a text file.
func (s Store) ReadText(path string) (string, error) {
	b, err := s.read(path)
	return string(b), err
}
