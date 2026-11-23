#!/bin/sh
# Grades every run in eval/agent/runs with hidden acceptance scenarios.
cd "$(dirname "$0")"
H=$(pwd); REPO=$(cd ../.. && pwd)
BIN="$H/rungkit"; [ -x "$BIN.exe" ] && BIN="$BIN.exe"
printf "%-8s %-4s %-6s %-6s %-7s %-5s %5s %5s %7s\n" model task accept regres comment check turns secs usd
for W in "$H"/runs/*; do
  [ -f "$W/result.json" ] || continue
  name=$(basename "$W"); m=${name%-*}; t=${name##*-}
  case $t in t1|t2|t4) f=conveyor.ldt;; t3) f=lamp.ldt;; t5) f=bug.ldt;; esac
  acc=FAIL; reg=-; com=-; chk=FAIL
  if [ -f "$W/$f" ]; then
    "$BIN" test -lang en "$W/$f" "$H/tasks/$t.accept" >/dev/null 2>&1 && acc=PASS
    "$BIN" check -lang en "$W/$f" >/dev/null 2>&1 && chk=ok
    case $t in t1|t4) "$BIN" test -lang en "$W/$f" "$REPO/examples/conveyor.ldtest" >/dev/null 2>&1 && reg=PASS || reg=FAIL;; esac
    case $t in t1) grep -q '^sym X3 .*"非常停止"' "$W/$f" && com=yes || com=no;; t4) grep -q '^sym Y2 .*"満杯ランプ"' "$W/$f" && com=yes || com=no;; esac
  else acc=NOFILE; fi
  turns=$(grep -o '"num_turns":[0-9]*' "$W/result.json" | head -1 | cut -d: -f2)
  ms=$(grep -o '"duration_ms":[0-9]*' "$W/result.json" | head -1 | cut -d: -f2)
  usd=$(grep -o '"total_cost_usd":[0-9.]*' "$W/result.json" | head -1 | cut -d: -f2)
  printf "%-8s %-4s %-6s %-6s %-7s %-5s %5s %5s %7.3f\n" "$m" "$t" $acc $reg $com $chk "${turns:-?}" "$(( ${ms:-0} / 1000 ))" "${usd:-0}"
done
