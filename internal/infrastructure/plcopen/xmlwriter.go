package plcopen

import (
	"encoding/xml"
	"strings"
)

// xw writes indented XML by hand. encoding/xml cannot write the xhtml
// namespace prefix PLCopen uses, and hand-writing keeps the element order the
// schema requires obvious.
type xw struct {
	b     strings.Builder
	depth int
}

func esc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (w *xw) tag(name string, attrs []string, end string) {
	w.b.WriteString(strings.Repeat("  ", w.depth) + "<" + name)
	for i := 0; i+1 < len(attrs); i += 2 {
		w.b.WriteString(" " + attrs[i] + `="` + esc(attrs[i+1]) + `"`)
	}
	w.b.WriteString(end + "\n")
}

// open writes <name a="v"...> and indents.
func (w *xw) open(name string, attrs ...string) {
	w.tag(name, attrs, ">")
	w.depth++
}

// close writes </name>.
func (w *xw) close(name string) {
	w.depth--
	w.b.WriteString(strings.Repeat("  ", w.depth) + "</" + name + ">\n")
}

// empty writes <name a="v"/>.
func (w *xw) empty(name string, attrs ...string) { w.tag(name, attrs, "/>") }

// text writes <name>text</name>.
func (w *xw) text(name, text string) {
	w.b.WriteString(strings.Repeat("  ", w.depth) + "<" + name + ">" + esc(text) + "</" + name + ">\n")
}
