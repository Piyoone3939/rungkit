package plcopen

import (
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
	"github.com/Piyoone3939/rungkit/internal/domain/profile"
)

// ---- XML model (namespaces are ignored; local names match) ----

type xProject struct {
	POUs    []xPOU  `xml:"types>pous>pou"`
	AddData []xData `xml:"addData>data"`
}

type xData struct {
	Name  string `xml:"name,attr"`
	Inner string `xml:",innerxml"`
}

type xPOU struct {
	Name      string      `xml:"name,attr"`
	PouType   string      `xml:"pouType,attr"`
	Interface xInterface  `xml:"interface"`
	LD        *xElemGroup `xml:"body>LD"`
}

type xInterface struct {
	Groups []xVarGroup `xml:",any"`
}

type xVarGroup struct {
	Vars []xVar `xml:"variable"`
}

type xVar struct {
	Name    string `xml:"name,attr"`
	Address string `xml:"address,attr"`
	Type    struct {
		Elems []struct {
			XMLName xml.Name
			Name    string `xml:"name,attr"`
		} `xml:",any"`
	} `xml:"type"`
	Doc xInner `xml:"documentation"`
}

type xInner struct {
	Inner string `xml:",innerxml"`
}

type xElemGroup struct {
	Elems []xElem `xml:",any"`
}

type xElem struct {
	XMLName      xml.Name
	LocalID      int         `xml:"localId,attr"`
	Negated      bool        `xml:"negated,attr"`
	Edge         string      `xml:"edge,attr"`
	Storage      string      `xml:"storage,attr"`
	TypeName     string      `xml:"typeName,attr"`
	InstanceName string      `xml:"instanceName,attr"`
	ExecOrder    int         `xml:"executionOrderId,attr"`
	Position     xPos        `xml:"position"`
	Ins          []xPointIn  `xml:"connectionPointIn"`
	Outs         []xPointOut `xml:"connectionPointOut"`
	Variable     string      `xml:"variable"`
	Expression   string      `xml:"expression"`
	InputVars    []xBlockVar `xml:"inputVariables>variable"`
	OutputVars   []xBlockVar `xml:"outputVariables>variable"`
	Content      xInner      `xml:"content"`
}

type xPos struct {
	X float64 `xml:"x,attr"`
	Y float64 `xml:"y,attr"`
}

type xPointIn struct {
	Connections []xConn `xml:"connection"`
}

type xPointOut struct {
	FormalParameter string `xml:"formalParameter,attr"`
}

type xConn struct {
	RefLocalID      int    `xml:"refLocalId,attr"`
	FormalParameter string `xml:"formalParameter,attr"`
}

type xBlockVar struct {
	FormalParameter string     `xml:"formalParameter,attr"`
	Ins             []xPointIn `xml:"connectionPointIn"`
}

// ---- import ----

// Import reads PLCopen XML. The first program POU with a Ladder Diagram body
// becomes the program.
func Import(data []byte) (*ladder.Program, error) {
	var proj xProject
	if err := xml.Unmarshal(data, &proj); err != nil {
		return nil, i18n.Errorf("XMLを読めません: %w", err)
	}
	p := &ladder.Program{Profile: "iec"}
	for _, d := range proj.AddData {
		if d.Name == profileData {
			if m := regexp.MustCompile(`profile="([^"]+)"`).FindStringSubmatch(d.Inner); m != nil {
				p.Profile = m[1]
			}
		}
	}
	prof, err := profile.Get(p.Profile)
	if err != nil {
		return nil, err
	}
	var pou *xPOU
	for i := range proj.POUs {
		if proj.POUs[i].LD != nil && (pou == nil || (pou.PouType != "program" && proj.POUs[i].PouType == "program")) {
			pou = &proj.POUs[i]
		}
	}
	if pou == nil {
		return nil, i18n.Errorf("ラダー（LD）で書かれた POU がありません")
	}
	im := &importer{p: p, melsec: strings.HasPrefix(p.Profile, "melsec"), prof: prof}
	im.symbols(pou.Interface)
	if err := im.body(pou.LD.Elems); err != nil {
		return nil, fmt.Errorf("POU %s: %w", pou.Name, err)
	}
	p.Normalize()
	p.AssignIDs()
	return p, nil
}

type importer struct {
	p      *ladder.Program
	prof   profile.Profile
	melsec bool
}

func typeName(v xVar) string {
	if len(v.Type.Elems) == 0 {
		return ""
	}
	t := v.Type.Elems[0]
	if t.XMLName.Local == "derived" {
		return strings.ToUpper(t.Name)
	}
	return strings.ToUpper(t.XMLName.Local)
}

func (im *importer) symbols(in xInterface) {
	for _, g := range in.Groups {
		for _, v := range g.Vars {
			typ := typeName(v)
			if typ == "R_TRIG" || typ == "F_TRIG" {
				continue // helpers for pulse instructions, not symbols
			}
			doc := plainText(v.Doc.Inner)
			if im.melsec {
				if doc != "" { // MELSEC devices need no declaration; keep the comment
					im.p.Symbols = append(im.p.Symbols, &ladder.Symbol{Name: v.Name, Comment: doc})
				}
				continue
			}
			im.p.Symbols = append(im.p.Symbols, &ladder.Symbol{Name: v.Name, Type: typ, Address: v.Address, Comment: doc})
		}
	}
}

var (
	reCDATA = regexp.MustCompile(`(?s)<!\[CDATA\[(.*?)\]\]>`)
	reTag   = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace = regexp.MustCompile(`\s+`)
)

// plainText turns xhtml content into one line of text.
func plainText(s string) string {
	s = reCDATA.ReplaceAllStringFunc(s, func(m string) string {
		return html.EscapeString(reCDATA.FindStringSubmatch(m)[1])
	})
	s = reTag.ReplaceAllString(s, " ")
	return strings.TrimSpace(reSpace.ReplaceAllString(html.UnescapeString(s), " "))
}

// operand converts an IEC expression back to the profile's spelling.
func (im *importer) operand(expr string) string {
	expr = strings.TrimSpace(expr)
	if !im.melsec {
		return expr
	}
	if v, err := strconv.ParseInt(expr, 10, 64); err == nil {
		return "K" + strconv.FormatInt(v, 10)
	}
	if hex, ok := strings.CutPrefix(expr, "16#"); ok {
		return "H" + strings.ToUpper(hex)
	}
	return expr
}

// timerPreset converts a TIME literal back for MELSEC (100 ms units).
func (im *importer) timerPreset(expr string) (string, error) {
	expr = strings.TrimSpace(expr)
	if !im.melsec {
		return expr, nil
	}
	ms, err := profile.ParseDuration(expr)
	if err != nil {
		return "", i18n.Errorf("タイマーの設定値 %q を読めません", expr)
	}
	if ms%100 != 0 {
		return "", i18n.Errorf("タイマーの設定値 %s は100ms単位ではないので、三菱のタイマーにできません", expr)
	}
	return "K" + strconv.FormatInt(ms/100, 10), nil
}

// ---- graph ----

// point is an output point: an element and, for blocks, the output name.
type point struct {
	id    int
	param string
}

var (
	rail = point{id: -1} // the left power rail (all of them are one net)
	sink = point{id: -2} // virtual end every output leads to
)

type unionFind map[point]point

func (u unionFind) find(p point) point {
	for {
		q, ok := u[p]
		if !ok || q == p {
			return p
		}
		if r, ok := u[q]; ok {
			u[p] = r // path halving
		}
		p = q
	}
}

func (u unionFind) union(a, b point) { u[u.find(a)] = u.find(b) }

// key orders elements top to bottom, then left to right, then in document
// order; outputs of a branch execute in this order.
type key struct {
	y, x float64
	doc  float64
}

func (a key) less(b key) bool {
	if a.y != b.y {
		return a.y < b.y
	}
	if a.x != b.x {
		return a.x < b.x
	}
	return a.doc < b.doc
}

type edge struct {
	from, to point
	node     ladder.Node
	key      key
	// alts are the branches when node is a Parallel built by reduce; they
	// keep their own keys so later merges can order all branches.
	alts []*edge
}

type comment struct {
	y    float64
	text string
}

var compareOps = map[string]string{"EQ": "=", "NE": "<>", "LT": "<", "LE": "<=", "GT": ">", "GE": ">="}
var instrTypes = map[string]string{"MOVE": "MOV", "ADD": "ADD", "SUB": "SUB", "MUL": "MUL", "DIV": "DIV"}

func (im *importer) body(elems []xElem) error {
	byID := map[int]*xElem{}
	keys := map[int]key{}
	for i := range elems {
		e := &elems[i]
		byID[e.LocalID] = e
		keys[e.LocalID] = key{e.Position.Y, e.Position.X, float64(i)}
	}
	conns := func(ins []xPointIn) []point {
		var ps []point
		for _, in := range ins {
			for _, c := range in.Connections {
				ps = append(ps, point{c.RefLocalID, c.FormalParameter})
			}
		}
		return ps
	}
	blockIn := func(e *xElem, param string) []point {
		for _, v := range e.InputVars {
			if v.FormalParameter == param {
				return conns(v.Ins)
			}
		}
		return nil
	}
	dataIn := func(e *xElem, param string) (string, error) {
		ps := blockIn(e, param)
		if len(ps) != 1 {
			return "", i18n.Errorf("ブロック %d の %s に値がつながっていません", e.LocalID, param)
		}
		src := byID[ps[0].id]
		if src == nil || src.XMLName.Local != "inVariable" {
			return "", i18n.Errorf("ブロック %d の %s は inVariable からつなぎます（ブロック同士の接続には対応していません）", e.LocalID, param)
		}
		return strings.TrimSpace(src.Expression), nil
	}
	dataOut := func(e *xElem, param string) (string, error) {
		for i := range elems {
			o := &elems[i]
			if o.XMLName.Local != "outVariable" {
				continue
			}
			for _, p := range conns(o.Ins) {
				if p.id == e.LocalID && p.param == param {
					return strings.TrimSpace(o.Expression), nil
				}
			}
		}
		return "", i18n.Errorf("ブロック %d の %s の行き先（outVariable）がありません", e.LocalID, param)
	}

	// Pass 1: decide which points are the same net.
	uf := unionFind{}
	joinAll := func(ps []point) {
		for _, p := range ps[1:] {
			uf.union(p, ps[0])
		}
	}
	pulseFrom := map[int][]point{} // instruction id -> CLK input of the R_TRIG feeding EN
	rtrigUsed := map[int]bool{}
	for i := range elems {
		e := &elems[i]
		switch e.XMLName.Local {
		case "leftPowerRail":
			uf.union(point{id: e.LocalID}, rail)
		case "contact", "coil":
			if ps := conns(e.Ins); len(ps) > 0 {
				joinAll(ps) // several wires into one input form a wired OR
				if e.XMLName.Local == "coil" {
					uf.union(point{id: e.LocalID}, ps[0]) // a coil passes power on unchanged
				}
			}
		case "block":
			for _, v := range e.InputVars {
				if ps := conns(v.Ins); len(ps) > 0 {
					joinAll(ps)
				}
			}
			if _, ok := instrTypes[strings.ToUpper(e.TypeName)]; ok {
				en := blockIn(e, "EN")
				if len(en) == 1 {
					if rt := byID[en[0].id]; rt != nil && rt.XMLName.Local == "block" && strings.ToUpper(rt.TypeName) == "R_TRIG" && en[0].param == "Q" {
						pulseFrom[e.LocalID] = blockIn(rt, "CLK")
						rtrigUsed[rt.LocalID] = true
						en = pulseFrom[e.LocalID]
					}
				}
				if len(en) > 0 {
					uf.union(point{e.LocalID, "ENO"}, en[0]) // ENO follows EN
				}
			}
		}
	}
	netOf := func(ps []point) (point, bool) {
		if len(ps) == 0 {
			return point{}, false
		}
		return uf.find(ps[0]), true
	}
	S := uf.find(rail)

	// Pass 2: conditions become edges between nets; outputs become edges
	// from their net to the sink.
	var edges []*edge
	var comments []comment
	addOut := func(e *xElem, ins []point, n ladder.Node) {
		if from, ok := netOf(ins); ok {
			edges = append(edges, &edge{from: from, to: sink, node: n, key: keys[e.LocalID]})
		}
	}
	hasRail := false
	for i := range elems {
		e := &elems[i]
		k := keys[e.LocalID]
		switch name := e.XMLName.Local; name {
		case "leftPowerRail":
			hasRail = true
		case "rightPowerRail", "inVariable", "outVariable":
		case "comment":
			if t := plainText(e.Content.Inner); t != "" {
				comments = append(comments, comment{e.Position.Y, t})
			}
		case "contact":
			c := &ladder.Contact{Mode: ladder.NO, Operand: strings.TrimSpace(e.Variable)}
			switch {
			case e.Negated:
				c.Mode = ladder.NC
			case e.Edge == "rising":
				c.Mode = ladder.Rise
			case e.Edge == "falling":
				c.Mode = ladder.Fall
			}
			if from, ok := netOf(conns(e.Ins)); ok {
				edges = append(edges, &edge{from: from, to: uf.find(point{id: e.LocalID}), node: c, key: k})
			}
		case "coil":
			if e.Negated {
				return i18n.Errorf("否定コイル（%d）にはまだ対応していません", e.LocalID)
			}
			c := &ladder.Coil{Mode: ladder.Out, Operand: strings.TrimSpace(e.Variable)}
			switch {
			case e.Storage == "set":
				c.Mode = ladder.Set
			case e.Storage == "reset":
				c.Mode = ladder.Reset
			case e.Edge == "rising":
				c.Mode = ladder.Pulse
			case e.Edge == "falling":
				c.Mode = ladder.PulseFall
			}
			addOut(e, conns(e.Ins), c)
		case "block":
			typ := strings.ToUpper(e.TypeName)
			switch {
			case compareOps[typ] != "":
				a, err := dataIn(e, "IN1")
				if err != nil {
					return err
				}
				b, err := dataIn(e, "IN2")
				if err != nil {
					return err
				}
				c := &ladder.Compare{Op: compareOps[typ], LHS: im.operand(a), RHS: im.operand(b)}
				from, ok := netOf(blockIn(e, "EN"))
				if !ok {
					from = S // a comparison without EN starts at the rail
				}
				edges = append(edges, &edge{from: from, to: uf.find(point{e.LocalID, "OUT"}), node: c, key: k})
			case typ == "TON" || typ == "TOF" || typ == "TP" || typ == "CTU":
				powerIn, dataParam := "IN", "PT"
				if typ == "CTU" {
					powerIn, dataParam = "CU", "PV"
					if len(blockIn(e, "R")) > 0 {
						return i18n.Errorf("CTU（%d）の R 入力の接続にはまだ対応していません", e.LocalID)
					}
				}
				preset, err := dataIn(e, dataParam)
				if err != nil {
					return err
				}
				inst := strings.TrimSpace(e.InstanceName)
				var n ladder.Node
				if typ == "CTU" {
					n = &ladder.Counter{Kind: typ, Instance: inst, Preset: im.operand(preset)}
				} else {
					pt, err := im.timerPreset(preset)
					if err != nil {
						return err
					}
					n = &ladder.Timer{Kind: typ, Instance: inst, Preset: pt}
				}
				addOut(e, blockIn(e, powerIn), n)
				// Whatever hangs off Q reads the instance like a contact. The
				// timer runs first (smaller key), so the value is the same.
				q := k
				q.doc += 0.5
				q.x += 0.5
				edges = append(edges, &edge{from: S, to: uf.find(point{e.LocalID, "Q"}),
					node: &ladder.Contact{Mode: ladder.NO, Operand: inst}, key: q})
			case instrTypes[typ] != "":
				in := &ladder.Instr{Name: instrTypes[typ]}
				params := []string{"IN1", "IN2"}
				if typ == "MOVE" {
					params = []string{"IN"}
				}
				for _, prm := range params {
					v, err := dataIn(e, prm)
					if err != nil {
						return err
					}
					in.Args = append(in.Args, im.operand(v))
				}
				dst, err := dataOut(e, "OUT")
				if err != nil {
					return err
				}
				in.Args = append(in.Args, im.operand(dst))
				en := blockIn(e, "EN")
				if clk, ok := pulseFrom[e.LocalID]; ok {
					in.Name += "P"
					en = clk
				}
				addOut(e, en, in)
			case typ == "R_TRIG" && rtrigUsed[e.LocalID]:
			default:
				return i18n.Errorf("ブロック %s（%d）にはまだ対応していません", e.TypeName, e.LocalID)
			}
		default:
			return i18n.Errorf("PLCopen の要素 <%s>（%d）にはまだ対応していません", name, e.LocalID)
		}
	}
	if !hasRail {
		return i18n.Errorf("左の母線（leftPowerRail）がありません")
	}
	return im.rungs(S, edges, comments)
}

// rungs splits the graph into connected parts (one rung each) and reduces
// every part to a series-parallel tree.
func (im *importer) rungs(S point, edges []*edge, comments []comment) error {
	// The rail is shared by every rung, so it is left out when finding parts.
	comp := unionFind{}
	anchor := map[*edge]point{}
	for i, e := range edges {
		switch {
		case e.from == S && e.to == sink:
			anchor[e] = point{id: -100 - i} // an output wired straight to the rail is its own rung
		case e.from == S:
			anchor[e] = e.to
		case e.to == sink:
			anchor[e] = e.from
		default:
			comp.union(e.from, e.to)
			anchor[e] = e.from
		}
	}
	groups := map[point][]*edge{}
	for _, e := range edges {
		c := comp.find(anchor[e])
		groups[c] = append(groups[c], e)
	}

	type built struct {
		top  key
		body ladder.Node
	}
	var rungs []built
	for _, es := range groups {
		body, top, err := reduce(S, es)
		if err != nil {
			return err
		}
		if body != nil {
			rungs = append(rungs, built{top, body})
		}
	}
	sort.Slice(rungs, func(i, j int) bool { return rungs[i].top.less(rungs[j].top) })
	notes := map[int][]string{}
	for _, c := range comments {
		at := len(rungs) - 1
		for i, r := range rungs {
			if r.top.y >= c.y {
				at = i
				break
			}
		}
		if at >= 0 {
			notes[at] = append(notes[at], c.text)
		}
	}
	for i, r := range rungs {
		body := ladder.Normalize(r.body)
		if err := ladder.CheckRung(body); err != nil {
			return i18n.Errorf("%d 番目のラング: %s %s", i+1, err.Code, err.Msg)
		}
		im.p.Rungs = append(im.p.Rungs, &ladder.Rung{Body: body, Comment: strings.Join(notes[i], " / ")})
	}
	return nil
}

// reduce turns the edges of one rung into a tree by series and parallel
// reduction. Edges that are not on a path from the rail to an output are
// dropped. It returns nil when nothing reaches an output.
func reduce(S point, es []*edge) (ladder.Node, key, error) {
	es = prune(S, es)
	if len(es) == 0 {
		return nil, key{}, nil
	}
	top := es[0].key
	for _, e := range es {
		if e.key.less(top) {
			top = e.key
		}
	}
	for changed := true; changed; {
		changed = false
		// parallel: edges between the same two nets
		type pair struct{ a, b point }
		byPair := map[pair][]*edge{}
		var order []pair
		for _, e := range es {
			pr := pair{e.from, e.to}
			if _, seen := byPair[pr]; !seen {
				order = append(order, pr)
			}
			byPair[pr] = append(byPair[pr], e)
		}
		if len(order) < len(es) {
			changed = true
			var next []*edge
			for _, pr := range order {
				group := byPair[pr]
				if len(group) == 1 {
					next = append(next, group[0])
					continue
				}
				var alts []*edge
				for _, e := range group {
					if e.alts != nil {
						alts = append(alts, e.alts...)
					} else {
						alts = append(alts, e)
					}
				}
				sort.SliceStable(alts, func(i, j int) bool { return alts[i].key.less(alts[j].key) })
				par := &ladder.Parallel{}
				for _, e := range alts {
					par.Items = append(par.Items, e.node)
				}
				next = append(next, &edge{from: pr.a, to: pr.b, node: par, key: alts[0].key, alts: alts})
			}
			es = next
		}
		// series: a net with exactly one wire in and one wire out
		in, out := map[point][]*edge{}, map[point][]*edge{}
		for _, e := range es {
			out[e.from] = append(out[e.from], e)
			in[e.to] = append(in[e.to], e)
		}
		for v, ins := range in {
			if v == S || v == sink || len(ins) != 1 || len(out[v]) != 1 {
				continue
			}
			a, b := ins[0], out[v][0]
			k := a.key
			if b.key.less(k) {
				k = b.key
			}
			merged := &edge{from: a.from, to: b.to, node: &ladder.Series{Items: []ladder.Node{a.node, b.node}}, key: k}
			var next []*edge
			for _, e := range es {
				if e == a {
					next = append(next, merged)
				} else if e != b {
					next = append(next, e)
				}
			}
			es = next
			changed = true
			break // the in/out maps are stale now
		}
	}
	if len(es) != 1 || es[0].from != S || es[0].to != sink {
		return nil, top, i18n.Errorf("E_NON_SERIES_PARALLEL: 直列・並列で表せない回路（ブリッジ回路など）があります（%.0f, %.0f 付近）", top.x, top.y)
	}
	return es[0].node, top, nil
}

// prune keeps the edges that lie on a path from the rail to an output.
func prune(S point, es []*edge) []*edge {
	fwd := map[point]bool{S: true}
	back := map[point]bool{sink: true}
	for changed := true; changed; {
		changed = false
		for _, e := range es {
			if fwd[e.from] && !fwd[e.to] {
				fwd[e.to], changed = true, true
			}
			if back[e.to] && !back[e.from] {
				back[e.from], changed = true, true
			}
		}
	}
	var keep []*edge
	for _, e := range es {
		if fwd[e.from] && back[e.to] {
			keep = append(keep, e)
		}
	}
	return keep
}
