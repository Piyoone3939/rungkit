# Contributing / 貢献のしかた

[English](#english) | [日本語](#日本語)

## English

Thanks for helping. Issues and pull requests are welcome in English or Japanese.

### Build and test

```sh
go test ./...                      # all tests, including the architecture and translation checks
go run ./cmd/rungkit check examples/conveyor.ldt
sh web/build.sh                    # browser version into web/dist (serve that folder)
eval/interop/run.sh                # optional: Beremiz and matiec in Docker
```

CI runs `gofmt`, `go vet` and `go test` on Linux, Windows and macOS, and builds the browser version.

### Where code goes

The project uses an onion architecture ([docs/design.en.md](docs/design.en.md)); `internal/architecture_test.go` fails if a package imports a layer it must not.

- `internal/domain`: the ladder model, vendor rules, checks, the simulator. Standard library only
- `internal/application`: use cases (editing, scenarios, the workspace). Declare the interfaces you need here
- `internal/infrastructure`: adapters that implement them (file formats, exporters, files)
- `internal/presentation`: CLI, MCP server, browser bindings. Never import infrastructure; `cmd/` wires things together

### Messages

User-facing messages are written in Japanese and translated in `internal/domain/i18n/catalog_en.go`. Wrap them with `i18n.T` or `i18n.Errorf`. `internal/i18n_test.go` fails if a Japanese string has no English entry or the format verbs differ. If you do not write Japanese, write the message in English inside `i18n.T` and say so in the pull request; a maintainer will add the Japanese ID.

### Behavior changes

- Ladder semantics (simulator, MELSEC list, ST) should match the real PLC. If you change them, add a scenario or a test that shows the case, and explain where the behavior comes from
- Formats (`.ldt`, JSON, `.ldtest`, MCP tools) may change during 0.x; note changes in [CHANGELOG.md](CHANGELOG.md)
- rungkit does not talk to real PLCs; pull requests adding online features need discussion first ([docs/decisions.en.md](docs/decisions.en.md))

### Checking against Mitsubishi's tools

If you have GX Works3, results of [docs/verify-melsec.en.md](docs/verify-melsec.en.md) are very valuable; use the "MELSEC check result" issue form.

---

## 日本語

ありがとうございます。Issue と Pull Request は日本語でも英語でも大丈夫です。

### ビルドとテスト

```sh
go test ./...                      # すべてのテスト（層の依存と翻訳漏れの検査を含む）
go run ./cmd/rungkit check examples/conveyor.ldt
sh web/build.sh                    # ブラウザ版を web/dist に作る（そのフォルダを配信）
eval/interop/run.sh                # 任意: Docker で Beremiz と matiec を使って確かめる
```

CI は Linux・Windows・macOS で `gofmt`・`go vet`・`go test` を実行し、ブラウザ版もビルドします。

### コードの置き場所

オニオンアーキテクチャです（[docs/design.md](docs/design.md)）。決まりに反する import があると `internal/architecture_test.go` が失敗します。

- `internal/domain`: ラダーのモデル、メーカー別ルール、検証、シミュレーター。標準ライブラリだけ
- `internal/application`: 使い方の単位（編集、シナリオ、ワークスペース）。必要なインターフェースはここで宣言する
- `internal/infrastructure`: それを実装するアダプタ（ファイル形式、書き出し、ファイル）
- `internal/presentation`: CLI、MCP サーバー、ブラウザとのつなぎ。インフラは import しない。組み立ては `cmd/`

### メッセージ

利用者に見せるメッセージは日本語で書き、`internal/domain/i18n/catalog_en.go` に英訳を足します。`i18n.T` か `i18n.Errorf` で包んでください。英訳がない、または `%s` などがそろっていないと `internal/i18n_test.go` が失敗します。

### 動きを変えるとき

- シミュレーター・三菱リスト形式・ST の意味は、実機と合わせます。変えるときは、その場面を示すシナリオかテストを足し、根拠を説明してください
- 形式（`.ldt`・JSON・`.ldtest`・MCP ツール）は 0.x の間は変わることがあります。変えたら [CHANGELOG.md](CHANGELOG.md) に書いてください
- rungkit は実機の PLC と通信しません。オンライン機能を足す Pull Request は、先に Issue で相談してください（[docs/decisions.md](docs/decisions.md)）

### 三菱の実物との突き合わせ

GX Works3 をお持ちなら、[docs/verify-melsec.md](docs/verify-melsec.md) の結果がとても役に立ちます。Issue の「三菱の実物との突き合わせ結果」フォームで送ってください。
