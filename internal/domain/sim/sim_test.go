package sim

import (
	"strings"
	"testing"

	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
)

func newSim(t *testing.T, src string) *Sim {
	t.Helper()
	if !strings.HasPrefix(src, "profile") {
		src = "profile melsec-iqr\n" + src
	}
	p, _, err := ldt.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s, err := New(p)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return s
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, s *Sim, op string) int64 {
	t.Helper()
	v, err := s.Get(op)
	must(t, err)
	return v
}

func TestSelfHold(t *testing.T) {
	s := newSim(t, `rung { (X0 | Y0) & !X1 -> OUT(Y0) }`)
	must(t, s.Set("X0", 1))
	must(t, s.Run(1))
	must(t, s.Set("X0", 0))
	must(t, s.Run(3))
	if get(t, s, "Y0") != 1 {
		t.Fatal("Y0 should hold")
	}
	must(t, s.Set("X1", 1))
	must(t, s.Run(1))
	if get(t, s, "Y0") != 0 {
		t.Fatal("Y0 should drop")
	}
}

func TestOutputsVisibleToLaterRungs(t *testing.T) {
	s := newSim(t, "rung { X0 -> OUT(M0) }\nrung { M0 -> OUT(Y0) }\nrung { Y1 -> OUT(Y2) }\nrung { X0 -> OUT(Y1) }")
	must(t, s.Set("X0", 1))
	must(t, s.Run(1))
	if get(t, s, "Y0") != 1 {
		t.Error("a later rung sees M0 in the same scan")
	}
	if get(t, s, "Y2") != 0 {
		t.Error("an earlier rung sees Y1 only on the next scan")
	}
	must(t, s.Run(1))
	if get(t, s, "Y2") != 1 {
		t.Error("Y2 should follow one scan later")
	}
}

func TestBranchKeepsPower(t *testing.T) {
	// The second branch uses the power saved before RST(M5) ran, but reads M1
	// from current memory.
	s := newSim(t, `rung { M5 -> { RST(M5)  M1 -> SET(M10) } }`)
	must(t, s.Set("M5", 1))
	must(t, s.Set("M1", 1))
	must(t, s.Run(1))
	if get(t, s, "M10") != 1 || get(t, s, "M5") != 0 {
		t.Fatal("branch should use the saved power flow")
	}
}

func TestEdgesAndPulse(t *testing.T) {
	s := newSim(t, "rung { rise(X0) -> OUT(M0) }\nrung { fall(X0) -> OUT(M1) }\nrung { X0 -> PLS(M2) }\nrung { X0 -> PLF(M3) }")
	type row struct{ x0, m0, m1, m2, m3 int64 }
	steps := []row{{1, 1, 0, 1, 0}, {1, 0, 0, 0, 0}, {0, 0, 1, 0, 1}, {0, 0, 0, 0, 0}, {1, 1, 0, 1, 0}}
	for i, r := range steps {
		must(t, s.Set("X0", r.x0))
		must(t, s.Run(1))
		got := row{r.x0, get(t, s, "M0"), get(t, s, "M1"), get(t, s, "M2"), get(t, s, "M3")}
		if got != r {
			t.Errorf("scan %d: got %+v want %+v", i+1, got, r)
		}
	}
}

func TestEdgeEvaluatedWithoutPower(t *testing.T) {
	// rise(X1) sits behind an open X0. It still tracks X1, so turning X0 on
	// later with X1 already on does not produce a rising edge.
	s := newSim(t, `rung { X0 & rise(X1) -> SET(M0) }`)
	must(t, s.Set("X1", 1))
	must(t, s.Run(1))
	must(t, s.Set("X0", 1))
	must(t, s.Run(1))
	if get(t, s, "M0") != 0 {
		t.Fatal("no edge expected")
	}
}

func TestTONTiming(t *testing.T) {
	s := newSim(t, `rung { X0 -> TON(T0, K10) }`) // 1 s
	must(t, s.SetScanTime(100))
	must(t, s.Set("X0", 1))
	must(t, s.Run(10)) // first scan starts measuring at 0 ms
	if get(t, s, "T0") != 0 || get(t, s, "T0.ET") != 900 {
		t.Fatalf("after 10 scans: T0=%d ET=%d", get(t, s, "T0"), get(t, s, "T0.ET"))
	}
	must(t, s.Run(1))
	if get(t, s, "T0") != 1 || get(t, s, "T0.ET") != 1000 {
		t.Fatalf("after 11 scans: T0=%d ET=%d", get(t, s, "T0"), get(t, s, "T0.ET"))
	}
	must(t, s.Run(5))
	if get(t, s, "T0.ET") != 1000 {
		t.Fatal("ET stops at the preset")
	}
	must(t, s.Set("X0", 0))
	must(t, s.Run(1))
	if get(t, s, "T0") != 0 || get(t, s, "T0.ET") != 0 {
		t.Fatal("TON resets when its input drops")
	}
}

// A MELSEC timer adds 100 ms ticks of a clock that runs on its own, so an
// input turning on in the middle of a tick reaches the preset sooner than
// the preset after it turned on (CPU module user's manual, accuracy of
// timers: up to one scan plus 100 ms early).
func TestTONTicksRunFreely(t *testing.T) {
	s := newSim(t, `rung { X0 -> TON(T0, K10) }`) // 1 s, 10 ms scans
	must(t, s.Run(5))                             // 50 ms into the first tick
	must(t, s.Set("X0", 1))
	must(t, s.Run(95)) // input on for 950 ms, ticks at 100..900 ms (9)
	if get(t, s, "T0") != 0 || get(t, s, "T0.ET") != 900 {
		t.Fatalf("after 950 ms: T0=%d ET=%d", get(t, s, "T0"), get(t, s, "T0.ET"))
	}
	must(t, s.Run(1)) // the tick at 1000 ms is added in this scan
	if get(t, s, "T0") != 1 {
		t.Fatal("T0 should be on 960 ms after its input")
	}
}

func TestTONPresetZero(t *testing.T) {
	s := newSim(t, `rung { X0 -> TON(T0, K0) }`)
	must(t, s.Set("X0", 1))
	must(t, s.Run(1))
	if get(t, s, "T0") != 1 {
		t.Fatal("a set value of 0 is up at the first execution")
	}
}

func TestCounterPresetZeroIsOne(t *testing.T) {
	s := newSim(t, `rung { X0 -> CTU(C0, K0) }`)
	must(t, s.Run(1))
	if get(t, s, "C0") != 0 {
		t.Fatal("MELSEC treats a set value of 0 as 1, so nothing is counted yet")
	}
	must(t, s.Set("X0", 1))
	must(t, s.Run(1))
	if get(t, s, "C0") != 1 || get(t, s, "C0.CV") != 1 {
		t.Fatalf("one count reaches it: CV=%d", get(t, s, "C0.CV"))
	}
}

func TestTimerPresetFromWordAndReset(t *testing.T) {
	s := newSim(t, "rung { X0 -> TON(T0, D0) }\nrung { X1 -> RST(T0) }")
	must(t, s.Set("D0", 3)) // 300 ms
	must(t, s.Set("X0", 1))
	must(t, s.Run(31))
	if get(t, s, "T0") != 1 {
		t.Fatal("T0 should be on after 300 ms")
	}
	must(t, s.Set("X1", 1))
	must(t, s.Run(1))
	// RST runs after the timer rung in this scan, so T0 is off now and
	// starts measuring again on the next scan.
	if get(t, s, "T0") != 0 || get(t, s, "T0.ET") != 0 {
		t.Fatal("RST should clear the timer")
	}
}

func TestIECTimers(t *testing.T) {
	src := "profile iec\nsym In : BOOL\nsym Q1 : BOOL\nsym Q2 : BOOL\nsym F : TOF\nsym P : TP\n" +
		"rung { In -> TOF(F, T#50ms) }\nrung { F -> OUT(Q1) }\nrung { In -> TP(P, T#30ms) }\nrung { P -> OUT(Q2) }"
	s := newSim(t, src)
	must(t, s.Set("In", 1))
	must(t, s.Run(1))
	if get(t, s, "Q1") != 1 || get(t, s, "Q2") != 1 {
		t.Fatal("TOF and TP turn on with the input")
	}
	must(t, s.Set("In", 0))
	must(t, s.Run(3)) // 20 ms after the fall (first off scan starts at 0)
	if get(t, s, "Q1") != 1 {
		t.Fatal("TOF still on")
	}
	if get(t, s, "Q2") != 0 {
		t.Fatal("TP pulse of 30 ms is over")
	}
	must(t, s.Run(3))
	if get(t, s, "Q1") != 0 {
		t.Fatal("TOF off after 50 ms")
	}
}

func TestCounter(t *testing.T) {
	s := newSim(t, "rung { X0 -> CTU(C0, K2) }\nrung { C0 -> OUT(Y0) }\nrung { X1 -> RST(C0) }")
	pulse := func() {
		must(t, s.Set("X0", 1))
		must(t, s.Run(2)) // held on: counts once
		must(t, s.Set("X0", 0))
		must(t, s.Run(1))
	}
	pulse()
	if get(t, s, "C0.CV") != 1 || get(t, s, "Y0") != 0 {
		t.Fatal("one count")
	}
	pulse()
	pulse()
	if get(t, s, "C0.CV") != 2 || get(t, s, "Y0") != 1 {
		t.Fatalf("MELSEC counter stops at its set value: CV=%d", get(t, s, "C0.CV"))
	}
	must(t, s.Set("X1", 1))
	must(t, s.Run(1))
	if get(t, s, "C0.CV") != 0 || get(t, s, "C0") != 0 {
		t.Fatal("RST clears the counter")
	}
}

func TestArithmetic(t *testing.T) {
	s := newSim(t, `rung { X0 -> {
  MOV(K32767, D0)
  INC(D0)
  MUL(K300, K300, D10)
  DIV(K-7, K2, D20)
  ADD(H7FFF, K1, D30)
  MOV(K5, D40)
} }`)
	must(t, s.Set("X0", 1))
	must(t, s.Run(1))
	for op, want := range map[string]int64{
		"D0":  -32768,         // INC wraps like a 16-bit word
		"D10": 90000 & 0xFFFF, // low word of 90000 ...
		"D11": 1,              // ... high word in D11
		"D20": -3, "D21": -1,  // quotient and remainder
		"D30": -32768,
		"D40": 5,
	} {
		v := get(t, s, op)
		if op == "D10" {
			v &= 0xFFFF
		}
		if v != want {
			t.Errorf("%s = %d, want %d", op, v, want)
		}
	}
}

func TestPulseInstructionRunsOnce(t *testing.T) {
	s := newSim(t, `rung { X0 -> INCP(D0) }`)
	must(t, s.Set("X0", 1))
	must(t, s.Run(5))
	if get(t, s, "D0") != 1 {
		t.Fatalf("INCP ran %d times", get(t, s, "D0"))
	}
}

func TestDivideByZero(t *testing.T) {
	s := newSim(t, `rung { X0 -> DIV(K1, D0, D1) }`)
	must(t, s.Set("X0", 1))
	err := s.Run(1)
	re, ok := err.(*RuntimeError)
	if !ok || re.Scan != 1 || re.Rung != "r1" || re.Elem != "e2" {
		t.Fatalf("got %v", err)
	}
}

func TestBitOfWord(t *testing.T) {
	s := newSim(t, "rung { X0 -> OUT(D0.3) }\nrung { D0.3 -> OUT(Y0) }\nrung { X1 -> SET(D0.F) }")
	must(t, s.Set("X0", 1))
	must(t, s.Set("X1", 1))
	must(t, s.Run(1))
	if get(t, s, "D0") != int64(int16(-32768|8)) || get(t, s, "Y0") != 1 {
		t.Fatalf("D0 = %d", get(t, s, "D0"))
	}
}

func TestForce(t *testing.T) {
	s := newSim(t, "rung { X0 -> OUT(Y0) }\nrung { Y0 -> OUT(Y1) }")
	must(t, s.Force("Y0", 1))
	must(t, s.Run(1))
	if get(t, s, "Y0") != 1 || get(t, s, "Y1") != 1 {
		t.Fatal("forced Y0 ignores the program")
	}
	s.Unforce("Y0")
	must(t, s.Run(1))
	if get(t, s, "Y0") != 0 {
		t.Fatal("released")
	}
}

func TestSpecialRelays(t *testing.T) {
	s := newSim(t, "rung { SM402 -> MOV(K7, D0) }\nrung { SM400 -> INC(D1) }\nrung { OUT(Y0) }")
	must(t, s.Run(3))
	if get(t, s, "D0") != 7 || get(t, s, "D1") != 3 || get(t, s, "Y0") != 1 {
		t.Fatal("SM402 first scan, SM400 always on")
	}
	must(t, s.Set("D0", 0))
	must(t, s.Run(1))
	if get(t, s, "D0") != 0 {
		t.Fatal("SM402 only on the first scan")
	}
}

func TestDoubleCoilLastWins(t *testing.T) {
	s := newSim(t, "rung { X0 -> OUT(Y0) }\nrung { X1 -> OUT(Y0) }")
	must(t, s.Set("X0", 1))
	must(t, s.Run(1))
	if get(t, s, "Y0") != 0 {
		t.Fatal("the later OUT overwrites")
	}
}

func TestRefusesBrokenProgram(t *testing.T) {
	p, _, _ := ldt.Parse("rung { X0 -> OUT(X1) }")
	if _, err := New(p); err == nil {
		t.Fatal("want error")
	}
}

func TestTraceAndOperands(t *testing.T) {
	s := newSim(t, `rung { (X0 | Y0) & [D0 < K5] -> { OUT(Y0)  INC(D0) } }`)
	p := s.p
	if got := strings.Join(Operands(p), " "); got != "X0 Y0 D0" {
		t.Fatalf("operands %s", got)
	}
	must(t, s.Watch("Y0", "D0", "Y0"))
	must(t, s.Set("X0", 1))
	must(t, s.Run(7))
	tr := s.Trace()
	if len(tr) != 7 || tr[0].Values[1] != 1 || tr[6].Values[1] != 5 || tr[6].TimeMs != 70 {
		t.Fatalf("trace %+v", tr)
	}
	if err := s.Watch("Q9"); err == nil {
		t.Fatal("bad operand")
	}
}
