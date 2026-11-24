# Interoperability check / 実際のツールでの確認

Checks rungkit's exports against real tools, inside Docker (nothing is installed on the host).

rungkit が書き出したものを、実際のツールで確かめます。Docker の中で動かすので、手元には何も入りません。

| | Check / 確認すること | Tool / 使うもの |
|---|---|---|
| C | PLCopen XML is loaded and built by Beremiz (XML → ST by Beremiz → C) / Beremiz が PLCopen XML を読み、ビルドできるか | [Beremiz](https://github.com/beremiz/beremiz) (`Beremiz_cli.py build`) |
| D | Structured Text compiles / ST がコンパイルできるか | [matiec](https://github.com/beremiz/matiec) (IEC → C compiler used by OpenPLC and Beremiz / OpenPLC と Beremiz が使うコンパイラ) |

```sh
eval/interop/run.sh
```

Inputs: [examples/conveyor.ldt](../../examples/conveyor.ldt) (melsec), [examples/verify/melsec-semantics.ldt](../../examples/verify/melsec-semantics.ldt) (melsec), [iec.ldt](iec.ldt) (iec).

## Results / 結果（2026-11-24）

All six pass. Exports also validate against the official schema (tc6_xml_v201.xsd).

6項目すべて成功。書き出したXMLは公式スキーマでも有効です。

```
D  ST -> matiec       conveyor: OK
D  ST -> matiec       iec: OK
D  ST -> matiec       melsec-semantics: OK
C  PLCopen -> Beremiz conveyor: OK
C  PLCopen -> Beremiz iec: OK
C  PLCopen -> Beremiz melsec-semantics: OK
```

## What it found and fixed / 見つかって直したこと

| Found / 見つかったこと | Fix / 修正 |
|---|---|
| ST: MELSEC `MUL` was written as `D20 := 300 * 300`, a type error (90000 does not fit INT) and without the high word / 三菱の `MUL` が型エラー（INTに収まらない）、上位ワードもなかった | 32-bit product, low word to D(n), high word to D(n+1); `DIV` also writes the remainder (`MOD`) / 32ビットで計算して下位・上位を書く。`DIV` は余りも書く |
| PLCopen: Beremiz needs a path (`position`) on every wire, though the schema makes it optional / Beremiz は配線の座標を必須として扱う | every wire carries its path / すべての配線に座標を書く |
| PLCopen: no task configuration, so the project could not be built / 実行の設定がなくビルドできなかった | a default task (`task0`, 10 ms, `instance0` of the program) / 既定のタスクを書き出す |
| PLCopen: a contact on a timer was written as `T0`; IEC needs `T0.Q` / タイマーの接点が `T0` だった（IEC では `T0.Q`） | written as `T0.Q`, read back as `T0` / `T0.Q` で書き、読むときに戻す |

## Known limits / 分かっている制限

- **Execution order in Beremiz.** Beremiz's LD → ST generator runs coils in position order and timer or counter calls afterwards, so a rung that reads a timer sees it one scan later than on a MELSEC PLC. `executionOrderId` cannot fix this: Beremiz applies it to blocks but not to coils. For rung-by-rung semantics use rungkit's own ST export.
- **Beremiz の実行順。** Beremiz のラダー→ST変換は、コイルを位置順に実行し、タイマー・カウンタの呼び出しをそのあとに回します。そのため、タイマーを読むラングは三菱より1スキャン遅れて値を見ます。`executionOrderId` はブロックにしか効かずコイルには効かないので、これでは直せません。ラング順どおりの動きが必要なら、rungkit の ST 出力を使ってください。
- **Importing Beremiz's own samples.** The LD samples that ship with Beremiz (`CounterLD` in `first_steps` and others) wire function blocks as data flow inside the ladder (SEL, ADD and an inOutVariable). rungkit does not read this style yet; it belongs to "function blocks inside a rung", planned after the UI.
- **Beremiz 付属のサンプルの読み込み。** Beremiz のラダーのサンプル（`first_steps` の `CounterLD` など）は、ラダーの中でブロックを値の流れとしてつないでいます（SEL・ADD・inOutVariable）。この書き方はまだ読めません。「ラングの途中に置くファンクションブロック」（画面の後に予定）の範囲です。
- Not checked yet: CODESYS (a GUI application).
- まだ確かめていないもの: CODESYS（画面のアプリのため）。

## OpenPLC Editor v4 (source review) / OpenPLC Editor v4（ソースを読んで確認）

Not run: running its code was not part of this check. Instead its PLCopen XML importer (File > Import PLCopen XML; `src/frontend/utils/PLC/xml-parser/language/ladder-xml.ts`, commit 0bd001c of 2026-11-23) was read against rungkit's export.

実行はしていません。OpenPLC Editor v4 の PLCopen XML 読み込み処理（ファイル → Import PLCopen XML。`src/frontend/utils/PLC/xml-parser/language/ladder-xml.ts`、2026-11-23 のコミット 0bd001c）を読み、rungkit の書き出しと突き合わせました。

| Point / 項目 | OpenPLC Editor v4 | rungkit export / rungkit の書き出し |
|---|---|---|
| Rungs / ラングの分け方 | connected parts of the wiring, each with exactly one left and one right rail / 配線でつながった部分ごと。左右の母線が1本ずつ必要 | one pair of rails per rung / ラングごとに母線を1組 ✓ |
| Rung shape / 形 | series-parallel reduction, as in rungkit / rungkit と同じ直列・並列への還元 | series-parallel by construction / もともと直列・並列 ✓ |
| Contacts and coils / 接点・コイル | `negated`, `edge`, `storage` | the same attributes / 同じ属性 ✓ |
| Blocks / ブロック | the rung runs through the first input and first output pin (EN/ENO when present); variable boxes on the other pins / ラングは最初の入力・出力ピン（EN/ENO があればそれ）を通り、ほかのピンは変数の箱 | TON/TOF/TP (IN → Q), CTU (CU → Q), R_TRIG (CLK → Q), MOVE/ADD/… (EN → ENO) ✓ |
| Comparisons / 比較 | the rung must leave through ENO / ENO から出る必要がある | the rung leaves through `OUT` (the comparison result) / 比較の結果 `OUT` から出る ✗ |
| Rung comments / ラングのコメント | not read / 読まない | dropped / 消える |

Expected result: programs without comparisons import as rungs the editor lays out itself. A rung with a comparison (`[D0 >= K10]`) is still imported, but OpenPLC warns that it "kept the layout from the XML" because elements are wired to the secondary output `OUT`. Writing comparisons through ENO would change their meaning (ENO follows EN, not the result), so rungkit keeps `OUT`. Confirmation in the editor itself is welcome.

見込み: 比較のないプログラムは、OpenPLC が自分で並べ直したラングとして取り込まれます。比較（`[D0 >= K10]`）のあるラングも取り込まれますが、「2つ目の出力 `OUT` につながっている」ため、XML の配置のまま取り込んだという警告が出るはずです。比較を ENO から出すと意味が変わる（ENO は結果ではなく EN に従う）ので、rungkit は `OUT` のままにしています。エディタ本体での確認を歓迎します。
