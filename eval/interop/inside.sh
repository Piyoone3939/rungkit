#!/bin/sh
# Runs inside the container. /work/in holds rungkit's exports, results go to /work/out.
#   D: compile rungkit's Structured Text with matiec (the compiler OpenPLC and Beremiz use)
#   C: build rungkit's PLCopen XML with Beremiz (XML -> ST by Beremiz -> matiec -> C)
mkdir -p /work/out
cfg='
CONFIGURATION config
  RESOURCE resource1 ON PLC
    TASK task0(INTERVAL := T#10ms, PRIORITY := 0);
    PROGRAM instance0 WITH task0 : Main;
  END_RESOURCE
END_CONFIGURATION'
status=0
for n in $(ls /work/in/*.st | xargs -n1 basename | sed 's/\.st$//'); do
  { cat /work/in/$n.st; echo "$cfg"; } > /tmp/$n.st
  mkdir -p /tmp/c-$n
  if /opt/matiec/iec2c -I /opt/matiec/lib -T /tmp/c-$n /tmp/$n.st > /work/out/$n.matiec.log 2>&1; then
    echo "D  ST -> matiec       $n: OK"
  else
    echo "D  ST -> matiec       $n: FAILED"; head -5 /work/out/$n.matiec.log; status=1
  fi
done
cd /opt/beremiz
for n in $(ls /work/in/*.xml | xargs -n1 basename | sed 's/\.xml$//'); do
  P=/tmp/p-$n; rm -rf $P; mkdir -p $P
  printf '<?xml version="1.0" encoding="utf-8"?>\n<BeremizRoot xmlns:xsd="http://www.w3.org/2001/XMLSchema" URI_location="LOCAL://">\n</BeremizRoot>\n' > $P/beremiz.xml
  cp /work/in/$n.xml $P/plc.xml
  if python3 Beremiz_cli.py --project-home $P build > /work/out/$n.beremiz.log 2>&1 && ! grep -qi "error" /work/out/$n.beremiz.log; then
    echo "C  PLCopen -> Beremiz $n: OK"
  else
    echo "C  PLCopen -> Beremiz $n: FAILED"; grep -i "error" /work/out/$n.beremiz.log | head -5; status=1
  fi
  [ -f $P/build/plc.st ] && cp $P/build/plc.st /work/out/$n.beremiz.st
done
exit $status
