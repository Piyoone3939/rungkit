# Changelog

[日本語は下にあります / Japanese below](#日本語)

Formats (`.ldt`, JSON, `.ldtest`, MCP tools) may change while the version is 0.x; changes are listed here. See [docs/decisions.en.md](docs/decisions.en.md).

## 0.2.0 (2026-11-24)

- Browser version redesigned: dark engineering-tool look with a light theme, ribbon toolbar, floating panels, status bar
- Parts palette: drag contacts, coils, timers, counters and comparisons onto the diagram (before, after, parallel or replace by drop position), or click to add next to the selection
- In-page input dialog and notices instead of browser prompts; Delete and Esc on the diagram
- Releases include MCP Bundles (`rungkit-mcp-<os>-<arch>.mcpb`) and `server.json` for the MCP Registry (`scripts/mcpb.sh`); publishing is a manual workflow

## 0.1.0 (2026-11-24)

- Ladder text format (`.ldt`), JSON, checks with diagnostic codes
- MELSEC instruction list and Structured Text export
- Scan simulator with `.ldtest` scenarios
- Edit commands with undo/redo, and an MCP server (10 tools)
- PLCopen XML (TC6 v2.01) Ladder Diagram import and export
- English and Japanese messages (`-lang`, `RUNGKIT_LANG`, system language)
- MCP file access confined to a working folder (`rungkit mcp -root DIR`)
- Scenarios accept decimal times (`run 3.5s`, `T#1.5s`) and `expect D0 == 1`
- `run_scenario` traces keep only scans where a value changed (at most 200 records per test over MCP)
- Warning `W_SYMBOL_TYPE_IGNORED` for IEC types or addresses on melsec symbols
- Agent evaluation harness in `eval/agent`
- Rung labels are checked (`E_BAD_LABEL`); saving refuses `.ldt` text that would not parse back
- Verified with real tools in Docker (`eval/interop`): ST compiles with matiec; PLCopen XML builds in Beremiz
- ST: MELSEC `MUL` writes the 32-bit product to D(n), D(n+1); `DIV` writes the remainder to D(n+1)
- PLCopen XML: wire paths, a default task configuration, and `T0.Q` for contacts on timers and counters
- Browser version (Go → WebAssembly, `web/`): edit `.ldt` with a live ladder diagram, diagnostics, scenarios with traces and every export; GitHub Pages workflow
- Ladder drawings: `export -to ascii` and `export -to svg`
- `melsec-fx5` profile (octal X/Y)
- Issue templates (bug, feature, MELSEC check result) and CONTRIBUTING.md; CI builds the browser version
- `export -to gxw3`: the CSV GX Works3 imports (UTF-16); `export -o FILE` writes the export to a file
- Scenarios accept `#` comments
- Agent evaluation harness works with recent Claude Code on Windows; Haiku passes all five tasks again after the timer change
- Browser version: edit on the diagram (select an element or rung, then add, replace or delete; undo)
- PLCopen XML reviewed against OpenPLC Editor v4's importer (`eval/interop`)
- Simulator checked against Mitsubishi's manuals: MELSEC timers add ticks of a free-running 100 ms clock (they may turn on up to 100 ms earlier than before); a counter set value of 0 counts as 1

---

## 日本語

0.x の間は形式（`.ldt`・JSON・`.ldtest`・MCPツール）を変えることがあり、変えたらここに書きます。[docs/decisions.md](docs/decisions.md) を参照してください。

### 0.2.0（2026-11-24）

- ブラウザ版のデザインを刷新: エンジニアリングツール風の暗い画面（明るいテーマも）、リボン型のツールバー、浮いたパネル、ステータスバー
- 部品パレット: 接点・コイル・タイマー・カウンタ・比較を図へドラッグして置く（置いた位置で前・後・並列・置き換え）。クリックで選択中の要素の隣に置く
- ブラウザ標準の入力欄の代わりにページ内のダイアログと通知。図の上で Delete と Esc
- リリースに MCP バンドル（`rungkit-mcp-<os>-<arch>.mcpb`）と MCP レジストリ用の `server.json` を添付（`scripts/mcpb.sh`）。レジストリへの登録は手動のワークフロー

### 0.1.0（2026-11-24）

- ラダーのテキスト形式（`.ldt`）、JSON、コード付きの検証
- 三菱リスト形式と Structured Text への書き出し
- スキャンのシミュレーターと `.ldtest` シナリオ
- 元に戻す・やり直す付きの編集コマンドと、MCPサーバー（ツール10個）
- PLCopen XML（TC6 v2.01）のラダー図の読み書き
- 日本語・英語のメッセージ（`-lang`、`RUNGKIT_LANG`、OSの言語設定）
- MCPのファイルアクセスを作業フォルダの中に制限（`rungkit mcp -root DIR`）
- シナリオで小数の時間（`run 3.5s`、`T#1.5s`）と `expect D0 == 1` を受け付ける
- `run_scenario` の記録は値が変わったスキャンだけにする（MCPでは1テスト200件まで）
- 三菱プロファイルの記号にIECの型・アドレスを書いたら警告 `W_SYMBOL_TYPE_IGNORED`
- AIの評価用の仕組み（`eval/agent`）
- ラングのラベルを検査（`E_BAD_LABEL`）。読み戻せない `.ldt` は保存しない
- 実際のツールで確認（`eval/interop`、Docker）: ST は matiec でコンパイルでき、PLCopen XML は Beremiz でビルドできる
- ST: 三菱の `MUL` は32ビットの積を D(n)・D(n+1) に、`DIV` は余りを D(n+1) に書く
- PLCopen XML: 配線の座標、既定のタスク設定、タイマー・カウンタの接点を `T0.Q` で書く
- ブラウザ版（Go → WebAssembly、`web/`）: `.ldt` を書くとラダー図・エラーがその場で出る。シナリオの記録表示、すべての形式への書き出し。GitHub Pages への公開ワークフロー
- ラダー図の表示: `export -to ascii` と `export -to svg`
- `melsec-fx5` プロファイル（X/Y が8進数）
- Issue テンプレート（不具合・要望・三菱の確認結果）と CONTRIBUTING.md。CI でブラウザ版もビルド
- `export -to gxw3`: GX Works3 が取り込む CSV（UTF-16）。`export -o FILE` で書き出しをファイルに保存
- シナリオで `#` のコメントを受け付ける
- AIの評価の仕組みを最近の Claude Code（Windows）に対応。タイマー変更後も Haiku が5課題すべて合格
- ブラウザ版: 図の上で編集（要素やラングを選んで、追加・置き換え・削除。元に戻す）
- PLCopen XML を OpenPLC Editor v4 の読み込み処理と突き合わせ（`eval/interop`）
- シミュレーターを三菱のマニュアルと照合: 三菱のタイマーは勝手に進む100msの時計の目盛りを足す（これまでより最大100ms早くONになることがある）。カウンタの設定値0は1として扱う
