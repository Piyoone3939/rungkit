# Design notes

[日本語](design.md)

## Goal

Do for ladder what SolidCraft (open-source 3D CAD that AI agents can drive) does for CAD:

- The UI, the CLI and MCP share one command engine
- An agent edits with deterministic commands, not coordinates or clicks
- An agent can check its own result (diagnostics, simulation, formal verification)

## Decisions

| Topic | Decision | Why |
|---|---|---|
| Rung representation | A series-parallel tree; no coordinates | Practically all real ladder fits, and it converts mechanically to the MELSEC list. Layout is computed from the tree |
| Circuits that are not series-parallel (bridges) | Not supported | The MELSEC list cannot express them either and they are rare; PLCopen XML import reports `E_NON_SERIES_PARALLEL` |
| First profiles | `melsec-iqr` and `iec` | Mitsubishi is common in Japan; IEC is the standard. FX5 (octal X/Y) can follow |
| Timers and counters | Only as outputs at the right end (MELSEC style) | IEC-style function blocks inside a rung come later |
| Agent input and output | `.ldt` text and JSON | LLMs cannot write ASCII-art ladder (arXiv 2410.15200) but do write boolean expressions |
| Language | Go | At hand and familiar. The only third-party library is the official MCP Go SDK, used by the presentation layer only. The UI will come through WebAssembly |
| Structure | Onion architecture | See "Layers" |

Release-level decisions (license, distribution, scope) are in [decisions.en.md](decisions.en.md).

## Layers

```
cmd/rungkit (composition)
  └ presentation: cli, mcpserver
      └ application: workspace, editing, scenario   ← declares the interfaces
          └ domain: ladder, profile, check, sim      ← standard library only
  └ infrastructure: ldt, ldtest, jsoncodec, export/*, plcopen, filestore, locale  ← implement them

shared: domain/i18n (message translation, used by every layer)
```

- The domain depends on nothing but the standard library; the application only on the domain
- The application declares interfaces for what it needs from outside (`editing.Notation`, `workspace.Files` / `Notation` / `Exporter` / `ScenarioParser`), on the consumer side as Go prefers
- Infrastructure implements them. Presentation does not know infrastructure; `cmd/rungkit` wires them together
- `internal/architecture_test.go` checks the imports. Test files are exempt (they may use the `.ldt` parser to build fixtures)

## Edit commands

- Every edit is a list of operations applied in order to a copy of the program. The copy replaces the program only if all operations succeed and every touched rung is valid (all or nothing)
- Intermediate states may be invalid; the shape is checked at the end, so several operations can restructure a rung
- `check` errors such as device mistakes are returned, not refused, so an agent can fix them next
- One applied list is one undo step. IDs handed out are never reused, even after undo
- `insert_parallel` with a range (target..to) finds the innermost series containing both elements and wraps that range
- `.ldt` views tag elements with `@e3`; pasting tagged text back is accepted (tags are ignored and new IDs assigned)

## Shape rules

- A `Series` is an output part if only its last item is an output part, a condition if none is
- A `Parallel` is a condition if all items are conditions, an output part (output branch) if all are outputs; mixing is `E_MIXED_PARALLEL`
- A rung body must be an output part (`E_NO_OUTPUT`)
- Normalization flattens a series in a series and a parallel in a parallel, and unwraps single-item containers

The text grammar cannot produce trees that break these rules; trees from JSON are checked.

## IDs

- Loading assigns `r1, r2, …` to rungs and `e1, e2, …` to elements
- Editing never changes other elements' IDs; new IDs never collide with existing ones
- Element IDs are not stored in files, to keep git diffs clean

## MELSEC instruction list

- Conditions: the first item is `LD`, series `AND`, parallel `OR`; blocks of several items combine with `ANB` / `ORB`
- Output branches: `MPS` only when a branch other than the last adds conditions, `MRD` in between, `MPP` to restore for the last
- An unconditional output uses `LD SM400` (always ON)
- The same sequence is written in two layouts: `melsec` (one instruction per line, for reading and diffs) and `gxw3` (the CSV GX Works3 imports with Edit > Import File, written as GX Works3 writes it: UTF-16 LE with a BOM, tab separated, every item quoted, CR LF; project name, module type and column headers on the first three lines; from the second argument on, each argument goes on its own row in the device column; rung comments become line statements. Source: the CSV import section of the GX Works3 Operating Manual)

## Structured Text

- Rungs become statements in order; outputs are written immediately, as in ladder
- An output branch saves the shared condition in a temporary `_rN_k` first (the same meaning as MPS)
- Edges use R_TRIG / F_TRIG; pulse instructions (`MOVP`, …) use R_TRIG
- MELSEC timer presets are 100 ms units, written as `T#…ms`
- `RST` of a counter cannot be exported yet

## Simulator

- Virtual time with a fixed scan time (10 ms by default, `scan` in `.ldtest`); every run gives the same result
- Every instruction runs every scan (no short-circuit). Edge contacts and pulse instructions keep their previous input per element ID
- MELSEC timers (`T`, low-speed 100 ms) imitate the CPU: a 100 ms clock runs on its own, END processing counts the ticks that passed during the scan, and the next execution of `OUT T` adds them to the current value while the coil is on, including the first scan it is on (CPU Module User's Manual (Application), "Accuracy of timers"). An input turning on in the middle of a tick turns the timer on up to 100 ms before "preset plus one scan"
- IEC timers start measuring in the scan the input turns on (elapsed 0) and add one scan time per scan, so the output appears after the preset plus one scan
- MELSEC counters stop at the set value (a set value of 0 counts as 1); IEC CTU counts up to 32767
- Runtime errors (division by zero, …) name the scan, rung and element
- Every `.ldtest` test runs on a fresh simulator; results are available as JSON (`run_scenario` returns them as is)

## MCP server

- Ten tools: open, new, save, read, check, edit, undo, redo, export, run scenario. Editing is one `edit_program` tool taking a list of operations, which keeps the tool list small
- Files change only on save. Saving is limited to `.ldt` / `.json` / `.xml`, so a mistyped path cannot overwrite another file
- Syntax and workflow are sent as instructions on connect
- Only files inside the working folder (`-root`, default: the folder it started in) can be read or written. Relative paths resolve inside it, symbolic links leading outside are refused, and standard input (`-`) is refused because it carries the MCP protocol. Files over 16 MB are not read

## Two languages

- Messages use the Japanese text as the message ID and look up an English catalog (`domain/i18n/catalog_en.go`), gettext style; this kept the existing code almost unchanged
- `internal/i18n_test.go` checks that every Japanese string in the code has an English entry with the same format verbs. Translations that reorder arguments use indexes such as `%[2]d`
- The language is chosen once at startup: `-lang` → `RUNGKIT_LANG` → `LC_ALL` / `LC_MESSAGES` / `LANG` → the OS setting (Windows user locale) → English
- Values created at package initialization (such as errors) are translated when shown, because they exist before the language is chosen
- Diagnostic codes (`E_READ_ONLY`, …) are the same in every language, so agents and tools can rely on them

## PLCopen XML

- Export: each rung gets its own left and right power rails and is laid out on a grid from the tree (series to the right, parallel downward). Contacts and coils are elements; comparisons, instructions and timers are blocks fed by inVariable / outVariable
- Import: output points (element ID + output name) are grouped into nets with union-find. Several wires into one input form a wired OR; coils and instruction ENO pass their input net on
- Conditions (contacts, comparisons) become edges between nets and outputs become "net → end" edges; series-parallel reduction turns them back into a tree, or reports `E_NON_SERIES_PARALLEL`. Branches that reach no output are dropped
- The left rail is shared by all rungs, so rungs are the parts connected without it. Rung order and output execution order follow position (top to bottom, left to right); a comment belongs to the rung right below it
- Whatever hangs off a timer's or counter's Q becomes a separate branch reading the instance as a contact (the timer runs first, so the value is the same)
- melsec programs are written with IEC spellings and the profile is kept in `addData`; import restores constants and timer presets to MELSEC spellings when it finds it
- Exports were validated against the official schema (tc6_xml_v201.xsd from Beremiz). The tests do not validate the schema (the Go standard library has no XSD validator)
- For Beremiz builds: every wire carries its path (`position`; optional in the schema but required by Beremiz), a default task is written, and contacts on timers and counters read `T0.Q`
- Beremiz runs coils in position order and blocks such as timers afterwards. `executionOrderId` applies only to blocks, so rung order cannot be enforced ([eval/interop](../eval/interop/README.md))

## Drawings and the browser version

- `render` lays a rung out on a grid from the tree (series to the right, parallel downward, outputs pushed to the right rail) and draws the same layout as text (`ascii`) and SVG (`svg`). SVG elements carry `data-elem` (the element ID) and a `<title>` with the device comment; colors use `currentColor`, so drawings follow the page theme
- The browser version is Go compiled to WebAssembly (`cmd/rungkit-web`). With no file system it uses `filestore.Memory` and the unchanged workspace. `internal/presentation/web` exposes functions to JavaScript (update, export, format, runScenario, … returning JSON); the page (`web/`) only calls them
- Input is kept in the browser's localStorage and sent nowhere. Clicking an element in the drawing jumps to its line in the `.ldt`
- This version edits text and redraws; editing on the drawing itself is the next step

## Next

1. Import the `-to gxw3` CSV into GX Works3 itself ([how to check](verify-melsec.en.md))
2. Publish the browser version on GitHub Pages (once the repository is public)
3. Edit on the drawing in the browser version (today the text is edited and the drawing follows)
4. Check PLCopen XML in OpenPLC Editor v4 and CODESYS themselves (Beremiz and matiec are done: [eval/interop](../eval/interop/README.md))
5. Formal verification of safety properties (ESBMC-PLC and others)
