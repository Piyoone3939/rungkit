package ldt

import "rungkit/internal/domain/ladder"

// Notation adapts the .ldt text format to the application layer, which only
// knows the interface it declares (editing.Notation, workspace.Notation).
type Notation struct{}

// ParseFragment reads a condition or output items.
func (Notation) ParseFragment(text string) (ladder.Node, error) { return ParseFragment(text) }

// FormatRung writes a rung body, optionally with element IDs.
func (Notation) FormatRung(body ladder.Node, withIDs bool) string {
	if withIDs {
		return FormatRungWithIDs(body)
	}
	return FormatRung(body)
}

// FormatProgram writes a whole program, optionally with IDs.
func (Notation) FormatProgram(p *ladder.Program, withIDs bool) (string, error) {
	if withIDs {
		return FormatWithIDs(p)
	}
	return Format(p)
}

// Exporter writes the canonical .ldt text (for workspace export).
type Exporter struct{}

// Name is the export format name.
func (Exporter) Name() string { return "ldt" }

// Export writes p as .ldt.
func (Exporter) Export(p *ladder.Program) (string, error) { return Format(p) }
