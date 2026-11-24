package filestore

import (
	"os"
	"sort"
	"sync"

	"github.com/Piyoone3939/rungkit/internal/application/workspace"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
)

// Memory implements workspace.Files in memory, for the browser version where
// there is no file system. Formats follow the extension as with Store.
type Memory struct {
	mu    sync.Mutex
	files map[string][]byte
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory { return &Memory{files: map[string][]byte{}} }

// Put stores a file.
func (m *Memory) Put(path string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = append([]byte(nil), data...)
}

// Get returns a stored file.
func (m *Memory) Get(path string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[path]
	return b, ok
}

// Paths lists the stored files.
func (m *Memory) Paths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ps []string
	for p := range m.files {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	return ps
}

// LoadProgram reads a stored program.
func (m *Memory) LoadProgram(path string) (*ladder.Program, workspace.Positions, error) {
	b, ok := m.Get(path)
	if !ok {
		return nil, nil, os.ErrNotExist
	}
	return decode(path, b)
}

// SaveProgram stores p in the format the extension names.
func (m *Memory) SaveProgram(path string, p *ladder.Program) error {
	out, err := encode(path, p)
	if err != nil {
		return err
	}
	m.Put(path, out)
	return nil
}

// ReadText returns a stored file as text.
func (m *Memory) ReadText(path string) (string, error) {
	b, ok := m.Get(path)
	if !ok {
		return "", os.ErrNotExist
	}
	return string(b), nil
}
