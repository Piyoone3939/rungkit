# Checking against Mitsubishi's own tools (free)

[日本語](verify-melsec.md)

How to check rungkit's MELSEC support against the **GX Works3 trial version** (free). No PLC hardware is needed.

| | What is checked | Files |
|---|---|---|
| A | The **instruction list** rungkit writes is accepted by GX Works3 and gives the same ladder | [examples/verify/melsec-semantics.ldt](../examples/verify/melsec-semantics.ldt) |
| B | The **simulator** behaves like the GX Works3 simulator | the same file and [melsec-semantics.ldtest](../examples/verify/melsec-semantics.ldtest) |

Please fill in the record at the end and share it in an issue.

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
go run ./cmd/rungkit test examples/verify/melsec-semantics.ldt examples/verify/melsec-semantics.ldtest
```

---

## A. Is the instruction list accepted?

The official way to import a list into GX Works3 (menu and file format) is **not known yet**; finding it is part of the result. Try in this order.

### A-1. Look for an import function

Look for menus such as: Project → open other format / read, with text, CSV or GX Developer formats; a list (mnemonic) view or CSV import in the ladder editor; or search the help for "CSV", "text", "list", "mnemonic". If one exists, load `melsec-semantics.txt` and note whether **tabs or commas, Shift_JIS or UTF-8, and `;` comment lines** are accepted. If not, the first lines of a file exported *from* GX Works3 tell us the format to produce.

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
