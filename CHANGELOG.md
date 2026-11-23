# Changelog

[日本語は下にあります / Japanese below](#日本語)

Formats (`.ldt`, JSON, `.ldtest`, MCP tools) may change while the version is 0.x; changes are listed here. See [docs/decisions.en.md](docs/decisions.en.md).

## Unreleased

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

---

## 日本語

0.x の間は形式（`.ldt`・JSON・`.ldtest`・MCPツール）を変えることがあり、変えたらここに書きます。[docs/decisions.md](docs/decisions.md) を参照してください。

### 未リリース

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
