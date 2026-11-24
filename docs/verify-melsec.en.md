# Checking against Mitsubishi's own tools (free)

[日本語](verify-melsec.md)

How to check rungkit's MELSEC support against the **GX Works3 trial version** (free). No PLC hardware is needed.

| | What is checked | Files |
|---|---|---|
| A | The **instruction list** rungkit writes is accepted by GX Works3 and gives the same ladder | [examples/verify/melsec-semantics.ldt](../examples/verify/melsec-semantics.ldt) |
| B | The **simulator** behaves like the GX Works3 simulator | the same file and [melsec-semantics.ldtest](../examples/verify/melsec-semantics.ldtest) |

Please fill in the record at the end and share it in an issue. Without GX Works3, see [C. Checking against the manuals](#c-checking-against-the-manuals-no-gx-works3-needed).

---

## 0. Setup

1. Download the **GX Works3 trial version** from Mitsubishi Electric's FA software update page (free FA membership required).
   - Japanese page: [MELSEC software downloads](https://www.mitsubishielectric.co.jp/fa/products/software/plc/update_index.html); the English trial is listed there as well
   - Check the trial's time and feature limits on the download page, including **whether the simulation function is available** (B needs it)
2. Install, start it and create a **new project**:
   - Series **RCPU**, type **R04CPU** (any iQ-R works), language **Ladder**
   - rungkit's `melsec-iqr` profile targets iQ-R (X/Y in hexadecimal). FX5 numbers X/Y differently, so use RCPU
3. Write the list with rungkit:

```sh
go run ./cmd/rungkit export -to melsec examples/verify/melsec-semantics.ldt > melsec-semantics.txt
go run ./cmd/rungkit export -to gxw3 -name Verify -o melsec-semantics.csv examples/verify/melsec-semantics.ldt
go run ./cmd/rungkit test examples/verify/melsec-semantics.ldt examples/verify/melsec-semantics.ldtest
```

---

## A. Is the instruction list accepted?

According to the GX Works3 Operating Manual, the ladder editor imports a CSV of listed instructions. `-to gxw3` writes that format (UTF-16, as GX Works3 itself writes it). It has not been tried in GX Works3 yet, so check it in A-1; if it fails, type the instructions in A-2.

### A-1. Import the CSV

1. Open the ladder editor and select Edit > Import File
2. Choose the file type "CSV (tab delimited)(*.csv)" and open `melsec-semantics.csv`
3. When asked for the conversion method for incorrect instructions, choose "Convert to line statement" and click Yes (importing replaces the program)
4. Convert (F4)

Note whether it imports, the text of any error, conversion errors, and whether the rung comments (V1–V9) came in as line statements. If it fails, the first ten lines of a CSV that GX Works3 writes with Edit > Export to File tell us what to change.

### A-2. Type the instructions (always possible)

In the ladder editor you can type instructions (start typing on an empty spot to open the input box). Type the lines of `melsec-semantics.txt` in order (`LDP X0`, Enter, `INC D10`, Enter, …; skip `;` comment lines). Check:

1. **Every instruction is accepted** (especially `LDP` `ANI` `MPS` `MRD` `MPP` `ANB` `ORB` `LD>=` `*` `/` `+`)
2. **Convert (F4)** reports no errors
3. The ladder drawn matches the rungs in the `.ldt`, in particular the output branch in V8 (`RST M5` → `AND M6` → `SET M7` without `MPS`)

---

## B. Does the simulator behave the same?

1. Debug → Simulation → Start Simulation (GX Simulator3 starts, writes the project and goes to RUN)
2. Online → Monitor → Start Monitoring; change device values with Debug → Modify Value, and watch D/C/T values in the Device/Buffer Memory Batch Monitor
3. Do each step below and write what GX Works3 shows in the "Real" column. The "rungkit" column is what `rungkit test` expects

| # | Step | Watch | rungkit | Real |
|---|---|---|---|---|
| V1 | Turn X0 ON **before** RUN, then RUN | D10 | 1 (rising edge on the first scan) | |
| V2 | Toggle X1 ON→OFF four times | C0 value / C0 contact | 3 / ON (stops at the set value) | |
| V3 | Right after RUN | D20 / D21 | 24464 / 1 | |
| V4 | Right after RUN | D30 / D31 | -3 / -1 | |
| V5 | Right after RUN | D40 | -32768 | |
| V6 | X2 ON (X3 OFF), then X3 ON too | Y0 | OFF → ON (the later OUT wins) | |
| V7 | X4 ON; time until T1 turns ON | how T1's value grows / time to ON | 1 s (+1 scan) | |
| V8 | M5 and M6 ON | M5 / M7 | OFF / ON | |
| V9 | Press and release X5 twice | D50 | 2 | |

For V7, note the timer unit in Parameter → CPU Parameter → Operation Related Setting → Timer Limit Setting (100 ms for the low-speed timer by default, as far as we know). The simulator may not run in real time; it is enough to see the value count 1, 2, 3 … every 100 ms and turn ON at 10.

---

## C. Checking against the manuals (no GX Works3 needed)

If you cannot get GX Works3, check V1–V9 against Mitsubishi Electric's **public manuals**. This is weaker than running the simulator, but it shows whether rungkit matches what the specification says.

### Getting the manuals

- The English PDFs on Mitsubishi Electric's global FA site could be downloaded without membership
- The PDFs restrict editing and copying, but any browser or PDF viewer can **read** them; there is no need to remove the protection
- Manual numbers change between editions, so search by **title**

| Short | Title | Where to look |
|---|---|---|
| Instr | MELSEC iQ-R Programming Manual (CPU Module Instructions, Standard Functions/Function Blocks) | the page of each instruction |
| CPU | MELSEC iQ-R CPU Module User's Manual (Application) | timers, counters, scan |
| GXW3 | GX Works3 Operating Manual | importing for A (CSV, text) |

### What to check

On an instruction page, read "Processing details", "Precautions" and "Operation error".

| # | Question | rungkit | Where | Search for |
|---|---|---|---|---|
| V1 | Does `LDP` turn ON in the first scan when the input was already ON before RUN? | yes | Instr: `LDP` / CPU: STOP to RUN | `LDP`, `STOP to RUN`, `rising edge` |
| V2 | Does a counter stop at its set value? | stops | Instr: `OUT C` / CPU: Counter | `OUT C`, `Counter`, `set value` |
| V3 | Does 16-bit `*` write the 32-bit result to (d) and (d)+1, and which is the high word? | (d) low, (d)+1 high | Instr: BIN 16-bit multiplication | `BIN 16-bit multiplication`, `*(P)` |
| V4 | Where do 16-bit `/` quotient and remainder go, and what sign has a negative remainder? | (d) quotient, (d)+1 remainder; -7/2 gives -3 and -1 | Instr: BIN 16-bit division | `BIN 16-bit division`, `remainder` |
| V5 | Does `INC` go from 32767 to -32768? | yes | Instr: `INC` | `INC(P)`, `32767`, `-32768` |
| V6 | With two `OUT`s to one coil, does the later one win? | yes | CPU or Instr: `OUT` / double coil | `double coil`, `OUT` |
| V7 | When does a timer's current value grow (when `OUT T` runs, or in END processing), and when does its contact turn ON? | one scan added each time `OUT T` runs; ON in scan preset+1 after the input | CPU: Timer (accuracy, timing) / Instr: `OUT T` | `Timer`, `accuracy`, `END processing`, `low-speed timer` |
| V8 | Do `MPS` / `MRD` / `MPP` store the result before the branch for later branches? | yes (M7 ON even though M5 turned OFF) | Instr: `MPS`, `MRD`, `MPP` | `MPS`, `operation result` |
| V9 | Is `PLS` ON for one scan on the rising edge, and what happens at STOP to RUN? | one scan per press | Instr: `PLS` | `PLS`, `one scan` |
| A | Is there a way to import an instruction list or CSV, and in what format? | unknown | GXW3: import / CSV / text | `CSV`, `import`, `text file`, `instruction list` |

V1, V7 and V9 are the most likely to differ: manuals often add notes about "STOP to RUN" or "END processing". For V7 the way the timer counts may itself be different.

### Recording

For each item give the manual's title, number and revision (on the cover) and the page, and summarize what it says in your own words; there is no need to copy long passages. For figures or tables the page number is enough.

```
Manual: title / number / revision
V1: (page) what it says:            same as rungkit / different / not stated
V2:
V3:
V4:
V5:
V6:
V7:
V8:
V9:
A:
```

---

### Results of C (2026-11-24)

Checked against: Instr = SH-081266ENG (AC), CPU = SH-081264ENG (AQ), GXW3 = SH-081215ENG (AN). Pages are the numbers printed in the manuals.

| # | Result | What the manual says (summary) | Where |
|---|---|---|---|
| V1 | same | An input already ON at power-on executes in the first scan (PLS precautions, latch relay example); after STOP to RUN it does not execute again. rungkit has no STOP, so this does not apply | Instr p.188 |
| V2 | same | After counting up, value and contact stay until RST; a set value of 0 works as 1 | Instr p.175 |
| V3 | same | 32-bit result, (d)+1 is the high word | Instr p.248 |
| V4 | partly | Quotient in (d), remainder in (d)+1; the sign of a negative remainder is not stated (rungkit gives -1) | Instr p.250 |
| V5 | same | Executed at 32767 it stores -32768 | Instr INC(P) |
| V6 | same | Each OUT sets the device from its own operation result; Y outputs the result of the last OUT | Instr p.58 |
| V7 | different, fixed | 100 ms ticks are counted at END and added when `OUT T` runs, from the first scan, so the timer can turn on before the preset (accuracy -(1 scan + 100 ms) to +1 scan). rungkit now works the same way | CPU p.385-390 |
| V8 | same | MPS stores the operation result; MRD and MPP read it and continue | Instr p.160 |
| V9 | same | ON for one scan on the rising edge, OFF otherwise | Instr p.187 |
| A | found | Edit > Import File reads a tab-delimited CSV; format as in A-1 and `-to gxw3` | GXW3 p.301-304 |

A counter with a set value of 0 (it was ON before counting) was also found and fixed.

---

## Record (copy this)

```
GX Works3 trial version:
CPU type:
Timer limit setting (low-speed timer):

[A-1] import function: found / not found
  menu:
  accepted format (separator, encoding, comment lines):
[A-2] instructions that could not be entered:
  convert errors:
  rungs whose ladder differs:

[B]
V1 D10 =
V2 C0 value = , C0 contact =
V3 D20 = , D21 =
V4 D30 = , D31 =
V5 D40 =
V6 Y0 = (X2 only) → (X3 too)
V7 T1 value when ON = , timing =
V8 M5 = , M7 =
V9 D50 =
notes:
```

Any difference is something to fix in rungkit (simulator: [internal/domain/sim](../internal/domain/sim/), list writer: [internal/infrastructure/export/melsec](../internal/infrastructure/export/melsec/)).
