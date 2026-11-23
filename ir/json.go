package ir

import (
	"encoding/json"
	"fmt"
)

// The JSON form is what MCP tools exchange. Nodes are a tagged union keyed by
// "t": series, parallel, contact, compare, coil, timer, counter, instr.

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

// MarshalProgram encodes p as indented JSON.
func MarshalProgram(p *Program) ([]byte, error) {
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
func MarshalNode(n Node) ([]byte, error) {
	jn, err := toJSON(n)
	if err != nil {
		return nil, err
	}
	return json.Marshal(jn)
}

func toJSON(n Node) (*jsonNode, error) {
	switch v := n.(type) {
	case *Series, *Parallel:
		var items []Node
		t := "series"
		if s, ok := v.(*Series); ok {
			items = s.Items
		} else {
			t, items = "parallel", v.(*Parallel).Items
		}
		jn := &jsonNode{T: t}
		for _, it := range items {
			b, err := MarshalNode(it)
			if err != nil {
				return nil, err
			}
			jn.Items = append(jn.Items, b)
		}
		return jn, nil
	case *Contact:
		return &jsonNode{T: "contact", ID: v.ID, Mode: string(v.Mode), Operand: v.Operand}, nil
	case *Compare:
		return &jsonNode{T: "compare", ID: v.ID, Op: v.Op, LHS: v.LHS, RHS: v.RHS}, nil
	case *Coil:
		return &jsonNode{T: "coil", ID: v.ID, Mode: string(v.Mode), Operand: v.Operand}, nil
	case *Timer:
		return &jsonNode{T: "timer", ID: v.ID, Kind: v.Kind, Instance: v.Instance, Preset: v.Preset}, nil
	case *Counter:
		return &jsonNode{T: "counter", ID: v.ID, Kind: v.Kind, Instance: v.Instance, Preset: v.Preset}, nil
	case *Instr:
		return &jsonNode{T: "instr", ID: v.ID, Name: v.Name, Args: v.Args}, nil
	case nil:
		return nil, fmt.Errorf("empty node")
	}
	return nil, fmt.Errorf("unknown node %T", n)
}

// UnmarshalProgram decodes JSON. The result is not normalized and may lack
// IDs; call Normalize and AssignIDs before use.
func UnmarshalProgram(data []byte) (*Program, error) {
	var jp jsonProgram
	if err := json.Unmarshal(data, &jp); err != nil {
		return nil, err
	}
	p := &Program{Profile: jp.Profile}
	for _, s := range jp.Symbols {
		p.Symbols = append(p.Symbols, &Symbol{s.Name, s.Type, s.Address, s.Comment})
	}
	for i, r := range jp.Rungs {
		body, err := UnmarshalNode(r.Body)
		if err != nil {
			return nil, fmt.Errorf("rungs[%d]: %w", i, err)
		}
		p.Rungs = append(p.Rungs, &Rung{ID: r.ID, Label: r.Label, Comment: r.Comment, Body: body})
	}
	return p, nil
}

// UnmarshalNode decodes one node.
func UnmarshalNode(data []byte) (Node, error) {
	var jn jsonNode
	if err := json.Unmarshal(data, &jn); err != nil {
		return nil, err
	}
	switch jn.T {
	case "series", "parallel":
		var items []Node
		for i, raw := range jn.Items {
			it, err := UnmarshalNode(raw)
			if err != nil {
				return nil, fmt.Errorf("items[%d]: %w", i, err)
			}
			items = append(items, it)
		}
		if jn.T == "series" {
			return &Series{Items: items}, nil
		}
		return &Parallel{Items: items}, nil
	case "contact":
		switch m := ContactMode(jn.Mode); m {
		case NO, NC, Rise, Fall:
			return &Contact{Base{jn.ID}, m, jn.Operand}, nil
		}
		return nil, fmt.Errorf("contact: unknown mode %q", jn.Mode)
	case "compare":
		return &Compare{Base{jn.ID}, jn.Op, jn.LHS, jn.RHS}, nil
	case "coil":
		switch m := CoilMode(jn.Mode); m {
		case Out, Set, Reset, Pulse, PulseFall:
			return &Coil{Base{jn.ID}, m, jn.Operand}, nil
		}
		return nil, fmt.Errorf("coil: unknown mode %q", jn.Mode)
	case "timer":
		return &Timer{Base{jn.ID}, jn.Kind, jn.Instance, jn.Preset}, nil
	case "counter":
		return &Counter{Base{jn.ID}, jn.Kind, jn.Instance, jn.Preset}, nil
	case "instr":
		return &Instr{Base{jn.ID}, jn.Name, jn.Args}, nil
	}
	return nil, fmt.Errorf("unknown node type %q", jn.T)
}
