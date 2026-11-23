# rungkit

ラダープログラムを、AIと人が同じテキストで読み書きするための土台です。
PLCのラダー図を「直列・並列の木」として持ち、テキスト（`.ldt`）、JSON、三菱のリスト形式、IEC 61131-3 の ST に変換します。

> 状態: 試作（v0）。シミュレーターまであります。MCPサーバーと画面はまだありません。

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

## 構成

| パッケージ | 役割 |
|---|---|
| [ir](ir/) | データ構造、正規化、形のチェック、JSON |
| [ldt](ldt/) | テキスト形式の読み込みと書き出し |
| [profile](profile/) | メーカーごとのデバイスと命令 |
| [check](check/) | エラーと警告 |
| [export/melsec](export/melsec/) | 三菱リスト形式（LD/AND/ORB/ANB/MPS/MRD/MPP） |
| [export/st](export/st/) | Structured Text |
| [sim](sim/) | シミュレーターとシナリオ（.ldtest） |

設計の考え方は [docs/design.md](docs/design.md) にあります。

## 注意

- 三菱リスト形式の**命令の並び**は標準の変換規則どおりですが、GX Works3 に取り込むときの**ファイル形式は未確認**です。
- 実機の制御に使う前提ではありません。設計・検証・教育用です。
