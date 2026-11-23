# rungkit

[English](README.md) | 日本語

AIが操作する前提で作った、PLCのラダープログラム開発環境です。
ラダー図を「直列・並列の木」として持ち、AIがMCPで編集し、シミュレーターで自分の作業を確かめられます。三菱（MELSEC iQ-R）とIECに対応し、テキスト（`.ldt`）・JSON・三菱のリスト形式・Structured Text・PLCopen XML を読み書きします。

> 状態: 試作（0.x）。編集コマンド、シミュレーター、MCPサーバー、PLCopen XML まであります。画面（ブラウザ版）はこれからです。

## インストール

```sh
go install github.com/Piyoone3939/rungkit/cmd/rungkit@latest
```

公開後は、各OS向けの実行ファイルを GitHub Releases に置きます。

## AIから使う（MCP）

```sh
claude mcp add rungkit -- rungkit mcp
```

- `rungkit mcp FILE` のようにファイルを渡すと、開いた状態で起動します
- MCPサーバーが読み書きできるのは、起動したフォルダの中だけです（`-root DIR` で変えられます）。標準入力（`-`）も使えません

ツールは次のとおりです。

| ツール | 内容 |
|---|---|
| `open_program` / `new_program` / `save_program` | 開く / 新しく作る / 保存（保存するまでファイルは変わらない） |
| `read_program` | プログラム全体を、ラングID（`// r1`）と要素ID（`X0@e3`）付きで表示 |
| `edit_program` | 編集コマンドのリストをまとめて適用。1つでも失敗したら何も変わらない。1回が元に戻す1単位 |
| `undo` / `redo` | 元に戻す / やり直す |
| `check_program` | エラーと警告（コード、ラングID、要素ID付き） |
| `run_scenario` | シナリオをシミュレーターで実行（テキストか `.ldtest` のパス。`trace` でスキャンごとの記録） |
| `export_program` | `melsec` / `st` / `plcopen` / `json` / `ldt` に書き出し |

編集コマンド（`edit_program` の `ops`）:

| コマンド | 内容 |
|---|---|
| `add_rung(text, after?, comment?, label?)` | ラングを追加 |
| `replace_rung(rung, text)` / `delete_rung(rung)` / `move_rung(rung, after)` / `set_rung(rung, comment?, label?)` | ラングの置き換え・削除・移動・コメント |
| `insert_series(target, position, text)` | 要素の前後に条件を直列で入れる |
| `insert_parallel(target, to?, text)` | 要素（または直列の範囲 target..to）に並列の枝を足す。出力に足すと出力の分岐になる |
| `replace_element(target, text)` / `delete_element(target)` / `set_operand(target, operand)` | 要素の置き換え・削除・デバイス変更 |
| `set_symbol(name, type?, address?, comment?)` / `delete_symbol(name)` | デバイスコメント・変数宣言 |

例: 停止ボタンの後ろに非常停止を足す

```json
{"ops": [
  {"op": "insert_series", "target": "e3", "position": "after", "text": "!X3"},
  {"op": "set_symbol", "name": "X3", "comment": "非常停止"}
]}
```

ラングが正しい形でなくなる編集（出力の右に接点を置く、など）は理由付きで断ります。デバイスの間違い（入力に書き込む、など）は断らずに `diagnostics` で返すので、続けて直せます。

## 書き方（.ldt）

```
profile melsec-iqr

sym X0 "運転ボタン"
sym X1 "停止ボタン"
sym Y0 "コンベアモータ"

rung "運転 自己保持" {
  (X0 | Y0) & !X1 -> OUT(Y0)
}

rung "ランプと到着の記録" {
  Y0 -> {
    OUT(Y1)
    rise(X2) -> SET(M10)
  }
}
```

| 書き方 | 意味 |
|---|---|
| `A & B` | 直列（AND）。`&` は `\|` より先に結びつく |
| `A \| B` | 並列（OR） |
| `!X1` | b接点 |
| `rise(X0)` / `fall(X0)` | 立上り / 立下り |
| `[D0 >= K100]` | 比較（`=` `<>` `<` `<=` `>` `>=`） |
| `条件 -> 出力` | 出力は右端に1つ、または `{ }` で分岐 |
| `OUT` `SET` `RST` `PLS` `PLF` | コイル |
| `TON(T0, K50)` / `CTU(C0, K10)` | タイマー / カウンタ |
| `MOV` `ADD` `SUB` `MUL` `DIV` `INC` `DEC`（末尾 `P` でパルス実行） | 命令 |
| `// ...` | コメント |

例: [examples/conveyor.ldt](examples/conveyor.ldt)

## 使い方

```sh
go run ./cmd/rungkit check examples/conveyor.ldt          # エラーと警告
go run ./cmd/rungkit fmt -w examples/conveyor.ldt         # 正規の書き方にそろえる
go run ./cmd/rungkit export -to melsec examples/conveyor.ldt
go run ./cmd/rungkit export -to st examples/conveyor.ldt
go run ./cmd/rungkit export -to json examples/conveyor.ldt
```

`check` はエラーに番号（`E_READ_ONLY` など）と行・列を付けます。AIが1件ずつ直せるようにするためです。

メッセージは日本語と英語に対応しています。`-lang ja|en`、環境変数 `RUNGKIT_LANG`、OSの言語設定の順に決まります。

## PLCopen XML

PLCopen XML（TC6 v2.01）のラダー図（LD）を読み書きできます。Beremiz / OpenPLC Editor / CODESYS などとやり取りするための形式です。

```sh
go run ./cmd/rungkit export -to plcopen -name Conveyor examples/conveyor.ldt > conveyor.xml
go run ./cmd/rungkit check conveyor.xml          # .xml はそのまま開ける（fmt / test / mcp も同じ）
```

- 書き出したXMLは公式スキーマ（tc6_xml_v201.xsd）で検証済みです
- 対応する要素: 接点（a/b/立上り/立下り）、コイル（SET/RST/パルス）、TON/TOF/TP/CTU、比較（EQ/NE/LT/LE/GT/GE）、MOVE/ADD/SUB/MUL/DIV、パルス命令（R_TRIG 経由）、ラングのコメント
- 読み込みでは配線のグラフを直列・並列の木に戻します。コイルの直列つなぎや、配線のOR（1つの入力に複数の線）も読めます。直列・並列で表せない回路（ブリッジ回路）は `E_NON_SERIES_PARALLEL` で断ります
- PLCopen はIECの形式なので、三菱プロファイルは `K10` → `10`、`H1F` → `16#1F`、タイマー `K50` → `T#5000ms` のように書き出し、プロファイル名を `addData` に残して、読み込むときに三菱の書き方へ戻します
- 往復で変わるもの: `INC`/`DEC` は `ADD`/`SUB`（同じ動き）、ワードの `RST` は `MOV(K0, …)`、1つのラングにある独立した回路は別々のラングになります
- 書き出せないもの: タイマー・カウンタの `RST`、三菱タイマーの設定値にワードデバイスを使うもの

## シミュレーター（.ldtest）

プログラムを仮想時間でスキャンごとに動かし、シナリオどおりに動くかを確かめます。

```
scan 10ms

test "ワークが来たら5秒後に1つ数える" {
  set X0 on
  run 1             // 1スキャン
  set X0 off
  set X2 on
  run 4s            // 仮想時間で4秒
  expect T0 off
  wait T0 on within 2s
  expect D0 1
}
```

| 命令 | 意味 |
|---|---|
| `set OP 値` | 入力などを設定（`on` `off` `123` `K123` `H1F` `5s`） |
| `force OP 値` / `unforce OP` | 値を固定する（プログラムが書いても変わらない） / 解除 |
| `run N` / `run 5s` | Nスキャン / 指定時間だけ動かす |
| `expect OP 値` | 今の値を確かめる |
| `wait OP 値 within 6s` | 値になるまで動かす。時間内にならなければ失敗 |
| `watch OP...` | スキャンごとの記録に入れる |

`T0.ET`（タイマーの経過時間）と `C0.CV`（カウンタの現在値）も読めます。テストは1件ごとに初期状態から始まります。

```sh
go run ./cmd/rungkit test examples/conveyor.ldt examples/conveyor.ldtest
go run ./cmd/rungkit test -trace examples/conveyor.ldt examples/conveyor.ldtest   # 値が変わったスキャンを表で表示
go run ./cmd/rungkit test -json examples/conveyor.ldt examples/conveyor.ldtest    # AI向けの結果
```

動き方はラダーに合わせています。

- ラングは上から順に、命令はすべて毎スキャン実行する（途中で評価を省略しない。立上り検出やタイマーが入力を見逃さない）
- 出力はその場で書くので、同じスキャンの後ろのラングは新しい値を見る
- 出力の分岐は、分岐前の通電状態を使う（三菱の MPS と同じ）
- 三菱の `T` は100ms単位、カウンタは設定値で止まる、`MUL` は D(n+1) に上位、`DIV` は D(n+1) に余り、ワードは16ビットで桁あふれ
- `SM400`（常時ON）、`SM401`（常時OFF）、`SM402`（最初のスキャンだけON）
- 0で割ると、スキャン番号とラング・要素のIDを付けたエラーで止まる

## プロファイル

| 名前 | 内容 |
|---|---|
| `melsec-iqr` | 三菱 iQ-R。X/Y/B/W は16進、`D0.3` のビット指定、`K`/`H` 定数、タイマーは100ms単位 |
| `iec` | IEC 61131-3。`sym 名前 : 型 @ アドレス` で宣言、`%IX0.0`、`T#5s` |

## 構成（オニオンアーキテクチャ）

依存は外から内への一方向です。[internal/architecture_test.go](internal/architecture_test.go) がこのルールを検査します。

| 層 | パッケージ | 役割 |
|---|---|---|
| ドメイン | [ladder](internal/domain/ladder/) | ラングの木、正規化、形のルール、ID |
| | [profile](internal/domain/profile/) | メーカーごとのデバイスと命令 |
| | [check](internal/domain/check/) | エラーと警告 |
| | [sim](internal/domain/sim/) | スキャンのシミュレーター |
| アプリケーション | [editing](internal/application/editing/) | 編集コマンド、元に戻す・やり直す |
| | [scenario](internal/application/scenario/) | シナリオの実行 |
| | [workspace](internal/application/workspace/) | 開く・保存・確認・編集・書き出し・シナリオ（CLIとMCPの窓口）。必要なインターフェースをここで宣言する |
| インフラ | [ldt](internal/infrastructure/ldt/) | `.ldt` テキストの読み書き |
| | [ldtest](internal/infrastructure/ldtest/) | `.ldtest` シナリオの読み込み |
| | [jsoncodec](internal/infrastructure/jsoncodec/) | JSON |
| | [export/melsec](internal/infrastructure/export/melsec/) / [export/st](internal/infrastructure/export/st/) | 三菱リスト形式 / Structured Text |
| | [plcopen](internal/infrastructure/plcopen/) | PLCopen XML の読み書き |
| | [filestore](internal/infrastructure/filestore/) | ファイルの読み書き（MCPでは作業フォルダの外を断る） |
| | [locale](internal/infrastructure/locale/) | OSの言語設定を読む |
| プレゼンテーション | [cli](internal/presentation/cli/) / [mcpserver](internal/presentation/mcpserver/) | コマンドライン / MCPサーバー |
| 共通 | [i18n](internal/domain/i18n/) | メッセージの日英翻訳（どの層からも使う。標準ライブラリだけに依存） |
| 組み立て | [cmd/rungkit](cmd/rungkit/) | アダプタをつないで起動するだけ |

- 設計: [docs/design.md](docs/design.md)
- 決めたこと（ライセンス・配布・範囲など）: [docs/decisions.md](docs/decisions.md)
- 三菱の実物との突き合わせ手順: [docs/verify-melsec.md](docs/verify-melsec.md)
- AIがどれだけ使えるかの評価: [eval/agent](eval/agent/README.md)（Sonnet 5/5、修正後の Haiku 5/5）

## 安全について

- rungkit は**実機のPLCと通信しません**（書き込み・モニタ・強制などのオンライン機能はありません）。書き出したものは、技術者が確認してからメーカーのツールで取り込んでください
- AIが書いたラダーは、`check` とシナリオで確かめてから使ってください。非常停止などの安全回路は、ソフトではなく配線と安全機器で作ってください
- 三菱リスト形式の**命令の並び**は標準の変換規則どおりですが、GX Works3 に取り込むときの**ファイル形式はまだ確認できていません**（[確認手順](docs/verify-melsec.md)）
- 三菱電機・MELSEC・GX Works などは各社の商標です。rungkit は各社と関係ありません

## ライセンス

[MIT](LICENSE-MIT) または [Apache-2.0](LICENSE-APACHE) のどちらかを選べます。
