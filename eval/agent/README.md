# Agent evaluation / AIの評価

Can an AI agent do real ladder work with only rungkit's MCP tools? Each task gives Claude Code (headless) a Japanese request; the saved file is then graded with a hidden acceptance scenario the agent never sees, plus the original conveyor scenario as a regression test.

AIエージェントが rungkit の MCP ツールだけで実際のラダー作業をこなせるかを測ります。Claude Code（ヘッドレス）に日本語で依頼し、保存されたファイルを、AIに見せていない合格判定シナリオと、元のコンベアのシナリオ（既存の動作を壊していないか）で採点します。

| Task | Request (summary) / 依頼の要約 |
|---|---|
| t1 | Add an emergency stop X3 (with a device comment) / 非常停止 X3 を足す（コメント付き） |
| t2 | Change the wait from 5 s to 3 s / 待ち時間を5秒→3秒に |
| t3 | New program: X10 toggles Y10 (alternate) / 新規作成: X10 で Y10 をオルタネイト |
| t4 | Light Y2 when D0 ≥ 100 (with a comment) / D0 が100以上で Y2 点灯（コメント付き） |
| t5 | Fix a motor that stops when the button is released / ボタンを離すと止まるモータを直す |

```sh
eval/agent/run.sh sonnet t1     # needs Claude Code; uses your account
eval/agent/grade.sh
```

## Results / 結果（2026-11-24）

| Model | t1 | t2 | t3 | t4 | t5 | Cost / 費用 |
|---|---|---|---|---|---|---|
| Sonnet | ✅ 6 turns | ✅ 12 | ✅ 18 | ✅ 7 | ✅ 7 | $1.38 |
| Haiku, first run / 1回目 | ✅ 8 | ❌ stopped / 途中で停止 | ✅ 32 | ✅ 7 | ✅ 9 | $0.82 |
| Haiku, after fixes / 修正後 | ✅ 6 | ✅ 9 | ✅ 13 | ✅ 8 | ✅ 7 | $0.34 |
| Haiku, again after the MELSEC timer change (Claude Code 2.1.206) / 三菱タイマー変更後に再評価 | ✅ 6 | ✅ 7 | ✅ 40 | ✅ 7 | ✅ 8 | $0.66 |

Every passing run also kept the existing behavior (regression scenario) and left no check errors.

合格したものはすべて、既存の動作も壊さず、`check` のエラーもありませんでした。

### What the runs taught us / 評価から直したこと

A tool-call trace of Haiku showed where it stumbled; each became a fix in rungkit:

Haiku のツール呼び出し記録から、つまずいた所を rungkit 側で直しました。

| Stumble / つまずき | Fix / 修正 |
|---|---|
| `run 3.5s` was refused / 小数の時間が弾かれた | decimal times accepted (`3.5s`, `T#1.5s`) / 小数の時間を受け付ける |
| `expect D0 == 1` was refused / `==` が弾かれた | `==` and `=` accepted / `==`・`=` を受け付ける |
| a trace of 58 KB overflowed the agent's context / 58KBの記録が多すぎた | only scans where a value changed, at most 200 per test / 値が変わったスキャンだけ、1テスト200件まで |
| an IEC type and address on a MELSEC device passed silently / 三菱のデバイスにIECの型・アドレスを書いても黙って通った | warning `W_SYMBOL_TYPE_IGNORED` / 警告を出す |
| a Japanese description put in the rung **label** made a file rungkit could not read back / ラベルに日本語の説明を入れ、読み戻せないファイルができた | labels are checked (`E_BAD_LABEL`, edits refused with a hint), and saving refuses text that does not parse / ラベルを検査し、読み戻せない内容は保存しない |

In the second Haiku run, t3 passed only at the turn limit. Its first circuit (SET and RST on the same rising edge in two rungs) was a real ladder bug that the simulator showed correctly, but Haiku misread the cause and tried several circuits before a working one. It also wrote `#` comments in a scenario, which are now accepted.

Haiku の2回目の評価では、t3 は手数の上限ぎりぎりで合格しました。最初の回路（同じ立上りで SET と RST を別のラングに書く）は実機でも打ち消し合うラダーの誤りで、シミュレーターは正しく示していましたが、Haiku が原因を読み違えて何度も作り直しました。また、シナリオに `#` でコメントを書いていたので、`#` もコメントとして受け付けるようにしました。

The harness itself needed two fixes for recent Claude Code on Windows: MCP config paths in `C:/…` form (a `/c/…` path made the server fail to start), and `ENABLE_TOOL_SEARCH=false` so the rungkit tools are loaded up front instead of behind a tool search while the server is still pending.

評価の仕組みも、最近の Claude Code（Windows）向けに2か所直しました。MCP の設定のパスを `C:/…` 形式にする（`/c/…` だとサーバーが起動しない）ことと、`ENABLE_TOOL_SEARCH=false` で rungkit のツールを最初から読み込ませる（サーバーの接続待ちの間にツール検索の陰に隠れないように）ことです。

Not covered yet: local models through Ollama (they need an MCP-capable client), and repeated runs to measure variance.

まだやっていないこと: Ollama などのローカルモデル（MCPに対応したクライアントが必要）、同じ課題を何度も回してばらつきを測ること。
