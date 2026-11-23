// Package jsoncodec reads and writes programs as JSON.
//
// Nodes are a tagged union keyed by "t": series, parallel, contact, compare,
// coil, timer, counter, instr.
package jsoncodec

import (
	"encoding/json"
	"fmt"

	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
)

type jsonProgram struct {
	Profile string        `json:"profile"`
	Symbols []*jsonSymbol `json:"symbols,omitempty"`
	Rungs   []*jsonRung   `json:"rungs"`
}

type jsonSymbol struct {
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"`
	Address string `json:"address,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type jsonRung struct {
	ID      string          `json:"id,omitempty"`
	Label   string          `json:"label,omitempty"`
	Comment string          `json:"comment,omitempty"`
	Body    json.RawMessage `json:"body"`
}

type jsonNode struct {
	T        string            `json:"t"`
	ID       string            `json:"id,omitempty"`
	Items    []json.RawMessage `json:"items,omitempty"`
	Mode     string            `json:"mode,omitempty"`
	Operand  string            `json:"operand,omitempty"`
	Op       string            `json:"op,omitempty"`
	LHS      string            `json:"lhs,omitempty"`
	RHS      string            `json:"rhs,omitempty"`
	Kind     string            `json:"kind,omitempty"`
	Instance string            `json:"instance,omitempty"`
	Preset   string            `json:"preset,omitempty"`
	Name     string            `json:"name,omitempty"`
	Args     []string          `json:"args,omitempty"`
}

// Marshal encodes p as indented JSON.
func Marshal(p *ladder.Program) ([]byte, error) {
	jp := jsonProgram{Profile: p.Profile, Rungs: []*jsonRung{}}
	for _, s := range p.Symbols {
		jp.Symbols = append(jp.Symbols, &jsonSymbol{s.Name, s.Type, s.Address, s.Comment})
	}
	for _, r := range p.Rungs {
		body, err := MarshalNode(r.Body)
		if err != nil {
			return nil, err
		}
		jp.Rungs = append(jp.Rungs, &jsonRung{r.ID, r.Label, r.Comment, body})
	}
	return json.MarshalIndent(jp, "", "  ")
}

// MarshalNode encodes one node.
func MarshalNode(n ladder.Node) ([]byte, error) {
	jn, err := toJSON(n)
	if err != nil {
		return nil, err
	}
	return json.Marshal(jn)
}

func toJSON(n ladder.Node) (*jsonNode, error) {
	switch v := n.(type) {
	case *ladder.Series:
		return container("series", v.Items)
	case *ladder.Parallel:
		return container("parallel", v.Items)
	case *ladder.Contact:
		return &jsonNode{T: "contact", ID: v.ID, Mode: string(v.Mode), Operand: v.Operand}, nil
	case *ladder.Compare:
		return &jsonNode{T: "compare", ID: v.ID, Op: v.Op, LHS: v.LHS, RHS: v.RHS}, nil
	case *ladder.Coil:
		return &jsonNode{T: "coil", ID: v.ID, Mode: string(v.Mode), Operand: v.Operand}, nil
	case *ladder.Timer:
		return &jsonNode{T: "timer", ID: v.ID, Kind: v.Kind, Instance: v.Instance, Preset: v.Preset}, nil
	case *ladder.Counter:
		return &jsonNode{T: "counter", ID: v.ID, Kind: v.Kind, Instance: v.Instance, Preset: v.Preset}, nil
	case *ladder.Instr:
		return &jsonNode{T: "instr", ID: v.ID, Name: v.Name, Args: v.Args}, nil
	case nil:
		return nil, fmt.Errorf("empty node")
	}
	return nil, fmt.Errorf("unknown node %T", n)
}

func container(t string, items []ladder.Node) (*jsonNode, error) {
	jn := &jsonNode{T: t}
	for _, it := range items {
		b, err := MarshalNode(it)
		if err != nil {
			return nil, err
		}
		jn.Items = append(jn.Items, b)
	}
	return jn, nil
}

// Unmarshal decodes JSON, then normalizes the program and assigns missing IDs.
func Unmarshal(data []byte) (*ladder.Program, error) {
	var jp jsonProgram
	if err := json.Unmarshal(data, &jp); err != nil {
		return nil, err
	}
	p := &ladder.Program{Profile: jp.Profile}
	for _, s := range jp.Symbols {
		p.Symbols = append(p.Symbols, &ladder.Symbol{Name: s.Name, Type: s.Type, Address: s.Address, Comment: s.Comment})
	}
	for i, r := range jp.Rungs {
		body, err := UnmarshalNode(r.Body)
		if err != nil {
			return nil, fmt.Errorf("rungs[%d]: %w", i, err)
		}
		p.Rungs = append(p.Rungs, &ladder.Rung{ID: r.ID, Label: r.Label, Comment: r.Comment, Body: body})
	}
	p.Normalize()
	p.AssignIDs()
	return p, nil
}

// UnmarshalNode decodes one node (not normalized).
func UnmarshalNode(data []byte) (ladder.Node, error) {
	var jn jsonNode
	if err := json.Unmarshal(data, &jn); err != nil {
		return nil, err
	}
	base := ladder.Base{ID: jn.ID}
	switch jn.T {
	case "series", "parallel":
		var items []ladder.Node
		for i, raw := range jn.Items {
			it, err := UnmarshalNode(raw)
			if err != nil {
				return nil, fmt.Errorf("items[%d]: %w", i, err)
			}
			items = append(items, it)
		}
		if jn.T == "series" {
			return &ladder.Series{Items: items}, nil
		}
		return &ladder.Parallel{Items: items}, nil
	case "contact":
		switch m := ladder.ContactMode(jn.Mode); m {
		case ladder.NO, ladder.NC, ladder.Rise, ladder.Fall:
			return &ladder.Contact{Base: base, Mode: m, Operand: jn.Operand}, nil
		}
		return nil, fmt.Errorf("contact: unknown mode %q", jn.Mode)
	case "compare":
		return &ladder.Compare{Base: base, Op: jn.Op, LHS: jn.LHS, RHS: jn.RHS}, nil
	case "coil":
		switch m := ladder.CoilMode(jn.Mode); m {
		case ladder.Out, ladder.Set, ladder.Reset, ladder.Pulse, ladder.PulseFall:
			return &ladder.Coil{Base: base, Mode: m, Operand: jn.Operand}, nil
		}
		return nil, fmt.Errorf("coil: unknown mode %q", jn.Mode)
	case "timer":
		return &ladder.Timer{Base: base, Kind: jn.Kind, Instance: jn.Instance, Preset: jn.Preset}, nil
	case "counter":
		return &ladder.Counter{Base: base, Kind: jn.Kind, Instance: jn.Instance, Preset: jn.Preset}, nil
	case "instr":
		return &ladder.Instr{Base: base, Name: jn.Name, Args: jn.Args}, nil
	}
	return nil, fmt.Errorf("unknown node type %q", jn.T)
}

// Exporter writes programs as JSON (for workspace export).
type Exporter struct{}

// Name is the export format name.
func (Exporter) Name() string { return "json" }

// Export encodes p.
func (Exporter) Export(p *ladder.Program) (string, error) {
	b, err := Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
