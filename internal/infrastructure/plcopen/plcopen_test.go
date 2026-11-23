package plcopen

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
)

var fixed = time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)

// roundTrip exports src and imports it back, returning the .ldt text.
func roundTrip(t *testing.T, src string) (string, string) {
	t.Helper()
	p, _, err := ldt.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	xmlText, err := Export(p, "Main", fixed)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	q, err := Import([]byte(xmlText))
	if err != nil {
		t.Fatalf("import: %v\n%s", err, xmlText)
	}
	out, err := ldt.Format(q)
	if err != nil {
		t.Fatal(err)
	}
	return out, xmlText
}

func TestRoundTripExample(t *testing.T) {
	src, err := os.ReadFile("../../../examples/conveyor.ldt")
	if err != nil {
		t.Fatal(err)
	}
	out, xmlText := roundTrip(t, string(src))
	// INCP comes back as ADDP(D0, K1, D0): PLCopen has no INC.
	want := strings.Replace(string(src), "INCP(D0)", "ADDP(D0, K1, D0)", 1)
	if out != want {
		t.Errorf("round trip changed the program:\n%s\nxml:\n%s", out, xmlText)
	}
}

func TestRoundTripShapes(t *testing.T) {
	rungs := []string{
		`(X0 | Y0) & !X1 -> OUT(Y0)`,
		`(X0 & X1 | X2 & X3) & !X4 -> OUT(Y0)`,
		`X0 & (X1 | X2 & X3) -> OUT(Y0)`,
		`(rise(X0) & fall(X1) | [D0 <> K5]) -> OUT(Y0)`,
		"X0 -> {\n    M1 -> OUT(Y1)\n    M2 -> OUT(Y2)\n    M3 -> OUT(Y3)\n  }",
		"X0 -> {\n    M1 -> {\n      OUT(Y1)\n      M2 -> OUT(Y2)\n    }\n    OUT(Y3)\n  }",
		"X0 -> {\n    OUT(Y0)\n    (M1 | M2) -> SET(M10)\n    RST(M11)\n    PLS(M12)\n    PLF(M13)\n  }",
		"X0 -> {\n    TON(T0, K50)\n    CTU(C0, K10)\n    ADD(D0, K1, D1)\n    MOVP(H1F, D2)\n    MUL(D3, D4, D5)\n  }",
		`[D0 >= K1000] -> MOV(K0, D0)`,
		`X0 & D0.3 -> OUT(D1.F)`,
	}
	for _, r := range rungs {
		src := "profile melsec-iqr\n\nrung {\n  " + r + "\n}\n"
		out, xmlText := roundTrip(t, src)
		if out != src {
			t.Errorf("round trip:\n%s\ngot:\n%s\nxml:\n%s", src, out, xmlText)
		}
	}
}

func TestRailLevelRungsSplit(t *testing.T) {
	// Two independent circuits in one rung come back as two rungs: PLCopen
	// has no rung boundaries, only connected parts.
	src := "profile melsec-iqr\n\nrung \"a\" {\n  X0 -> OUT(Y0)\n  X1 -> OUT(Y1)\n}\n"
	out, _ := roundTrip(t, src)
	want := "profile melsec-iqr\n\nrung \"a\" {\n  X0 -> OUT(Y0)\n}\n\nrung {\n  X1 -> OUT(Y1)\n}\n"
	if out != want {
		t.Errorf("got\n%s", out)
	}
}

func TestRoundTripIEC(t *testing.T) {
	src := `profile iec

sym Start : BOOL @ %IX0.0 "起動 & 確認"
sym Stop : BOOL @ %IX0.1
sym Motor : BOOL @ %QX0.0
sym Delay : TON
sym N : INT

rung "始動" {
  (Start | Motor) & !Stop -> {
    OUT(Motor)
    TON(Delay, T#2s500ms)
  }
}

rung {
  Delay & [N < 100] -> ADD(N, 1, N)
}
`
	out, xmlText := roundTrip(t, src)
	if out != src {
		t.Errorf("got\n%s\nxml:\n%s", out, xmlText)
	}
	for _, want := range []string{
		`<project xmlns="http://www.plcopen.org/xml/tc6_0201" xmlns:xhtml="http://www.w3.org/1999/xhtml">`,
		`creationDateTime="2026-11-23T09:00:00"`,
		`<variable name="Start" address="%IX0.0">`,
		`<xhtml:p>起動 &amp; 確認</xhtml:p>`,
		`<derived name="TON"/>`,
		`<block localId="`, `typeName="TON" instanceName="Delay"`,
		`<expression>T#2s500ms</expression>`,
	} {
		if !strings.Contains(xmlText, want) {
			t.Errorf("xml lacks %s", want)
		}
	}
}

func TestMelsecSpellingsInXML(t *testing.T) {
	p, _, _ := ldt.Parse("rung { [D0 >= H1F] -> { TON(T0, K50)  MOV(K-3, D1) } }\n")
	x, err := Export(p, "Main", fixed)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<expression>16#1F</expression>", "<expression>T#5000ms</expression>",
		"<expression>-3</expression>", `<variable name="D0">`, `<derived name="TON"/>`, `profile="melsec-iqr"`} {
		if !strings.Contains(x, want) {
			t.Errorf("xml lacks %s", want)
		}
	}
}

func TestExportRefusals(t *testing.T) {
	for _, src := range []string{
		"rung { X0 -> RST(T0) }",
		"rung { X0 -> TON(T0, D0) }",
		"rung { X0 -> OUT(X1) }", // check error
	} {
		p, _, err := ldt.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Export(p, "Main", fixed); err == nil {
			t.Errorf("%s: want error", src)
		}
	}
}

// A hand-written file in the style other editors produce: coils chained in
// series, a wired OR, a comment above the rung and no rungkit addData.
const foreign = `<?xml version="1.0" encoding="utf-8"?>
<project xmlns="http://www.plcopen.org/xml/tc6_0201" xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <types><pous>
    <pou name="Other" pouType="functionBlock"><interface/><body><ST><xhtml:p>x</xhtml:p></ST></body></pou>
    <pou name="Main" pouType="program">
      <interface>
        <localVars>
          <variable name="A"><type><BOOL/></type><documentation><xhtml:p><![CDATA[ボタン <A>]]></xhtml:p></documentation></variable>
          <variable name="B"><type><BOOL/></type></variable>
          <variable name="C"><type><BOOL/></type></variable>
          <variable name="Q1"><type><BOOL/></type></variable>
          <variable name="Q2"><type><BOOL/></type></variable>
        </localVars>
      </interface>
      <body><LD>
        <comment localId="9"><position x="0" y="10"/><content><xhtml:p>chained coils</xhtml:p></content></comment>
        <leftPowerRail localId="1"><position x="0" y="40"/>
          <connectionPointOut formalParameter=""/><connectionPointOut formalParameter=""/>
        </leftPowerRail>
        <contact localId="2" negated="false"><position x="50" y="40"/>
          <connectionPointIn><connection refLocalId="1"/></connectionPointIn><connectionPointOut/><variable>A</variable></contact>
        <contact localId="3" negated="false"><position x="50" y="80"/>
          <connectionPointIn><connection refLocalId="1"/></connectionPointIn><connectionPointOut/><variable>B</variable></contact>
        <contact localId="4" negated="true"><position x="120" y="40"/>
          <connectionPointIn><connection refLocalId="2"/><connection refLocalId="3"/></connectionPointIn><connectionPointOut/><variable>C</variable></contact>
        <coil localId="5"><position x="200" y="40"/>
          <connectionPointIn><connection refLocalId="4"/></connectionPointIn><connectionPointOut/><variable>Q1</variable></coil>
        <coil localId="6" storage="set"><position x="260" y="40"/>
          <connectionPointIn><connection refLocalId="5"/></connectionPointIn><connectionPointOut/><variable>Q2</variable></coil>
        <rightPowerRail localId="7"><position x="320" y="40"/>
          <connectionPointIn><connection refLocalId="6"/></connectionPointIn></rightPowerRail>
      </LD></body>
    </pou>
  </pous></types>
</project>`

func TestImportForeign(t *testing.T) {
	p, err := Import([]byte(foreign))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := ldt.Format(p)
	want := `profile iec

sym A : BOOL "ボタン <A>"
sym B : BOOL
sym C : BOOL
sym Q1 : BOOL
sym Q2 : BOOL

rung "chained coils" {
  (A | B) & !C -> {
    OUT(Q1)
    SET(Q2)
  }
}
`
	if out != want {
		t.Errorf("got\n%s", out)
	}
}

func TestImportRefusesBridge(t *testing.T) {
	// A -+- C -+- (Q)
	//    |  E  |        E connects the middle of the two branches
	// B -+- D -+
	bridge := strings.NewReplacer("\n", "").Replace(`<project xmlns="http://www.plcopen.org/xml/tc6_0201"><types><pous><pou name="M" pouType="program"><interface/><body><LD>
<leftPowerRail localId="1"><connectionPointOut/></leftPowerRail>
<contact localId="2"><position y="0"/><connectionPointIn><connection refLocalId="1"/></connectionPointIn><variable>A</variable></contact>
<contact localId="3"><position y="40"/><connectionPointIn><connection refLocalId="1"/></connectionPointIn><variable>B</variable></contact>
<contact localId="4"><position y="0"/><connectionPointIn><connection refLocalId="2"/></connectionPointIn><variable>C</variable></contact>
<contact localId="5"><position y="40"/><connectionPointIn><connection refLocalId="3"/></connectionPointIn><variable>D</variable></contact>
<contact localId="6"><position y="20"/><connectionPointIn><connection refLocalId="2"/></connectionPointIn><variable>E</variable></contact>
<contact localId="7"><position y="40"/><connectionPointIn><connection refLocalId="6"/><connection refLocalId="3"/></connectionPointIn><variable>F</variable></contact>
<coil localId="8"><connectionPointIn><connection refLocalId="4"/><connection refLocalId="7"/></connectionPointIn><variable>Q</variable></coil>
</LD></body></pou></pous></types></project>`)
	_, err := Import([]byte(bridge))
	if err == nil || !strings.Contains(err.Error(), "E_NON_SERIES_PARALLEL") {
		t.Fatalf("got %v", err)
	}
}

func TestImportErrors(t *testing.T) {
	wrap := func(ld string) string {
		return `<project><types><pous><pou name="M" pouType="program"><interface/><body><LD>` + ld + `</LD></body></pou></pous></types></project>`
	}
	cases := []struct{ xml, want string }{
		{"not xml", "XMLを読めません"},
		{`<project><types><pous><pou name="M"><body><ST/></body></pou></pous></types></project>`, "LD"},
		{wrap(`<contact localId="2"><connectionPointIn/><variable>A</variable></contact>`), "leftPowerRail"},
		{wrap(`<leftPowerRail localId="1"/><jump localId="2"/>`), "<jump>"},
		{wrap(`<leftPowerRail localId="1"/><block localId="2" typeName="SR"/>`), "SR"},
		{wrap(`<leftPowerRail localId="1"/><block localId="2" typeName="TON" instanceName="T"><inputVariables><variable formalParameter="IN"><connectionPointIn><connection refLocalId="1"/></connectionPointIn></variable></inputVariables></block>`), "PT に値"},
	}
	for _, c := range cases {
		_, err := Import([]byte(c.xml))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%.60s: got %v, want %q", c.xml, err, c.want)
		}
	}
}

func TestDeadBranchesAreDropped(t *testing.T) {
	ld := `<project><types><pous><pou name="M" pouType="program"><interface/><body><LD>
<leftPowerRail localId="1"/>
<contact localId="2"><connectionPointIn><connection refLocalId="1"/></connectionPointIn><variable>A</variable></contact>
<contact localId="3"><connectionPointIn><connection refLocalId="1"/></connectionPointIn><variable>B</variable></contact>
<coil localId="4"><connectionPointIn><connection refLocalId="2"/></connectionPointIn><variable>Q</variable></coil>
</LD></body></pou></pous></types></project>`
	p, err := Import([]byte(ld))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rungs) != 1 || ldt.FormatRung(p.Rungs[0].Body) != "A -> OUT(Q)" {
		t.Fatalf("%+v", p.Rungs)
	}
}

// What Beremiz needs to build the project (see eval/interop).
func TestBuildableByBeremiz(t *testing.T) {
	p, _, _ := ldt.Parse("rung { X0 -> TON(T0, K10) }\nrung { T0 -> OUT(Y0) }\n")
	x, err := Export(p, "Main", fixed)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<pouInstance name="instance0" typeName="Main"/>`,
		`<task name="task0" priority="0" interval="T#10ms">`,
		`<variable>T0.Q</variable>`,
	} {
		if !strings.Contains(x, want) {
			t.Errorf("xml lacks %s", want)
		}
	}
	// every wire has a path
	if strings.Count(x, "<connection ") != strings.Count(x, "</connection>") {
		t.Error("a connection without position points")
	}
	q, err := Import([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	if got := ldt.FormatRung(q.Rungs[1].Body); got != "T0 -> OUT(Y0)" {
		t.Errorf("T0.Q read back as %q", got)
	}
}
