#!/bin/sh
# Checks rungkit's exports against real tools in Docker (nothing is installed
# on the host). The first run builds the image, which takes a few minutes.
# usage: eval/interop/run.sh
set -e
cd "$(dirname "$0")"
H=$(pwd); REPO=$(cd ../.. && pwd)
docker build -q -t rungkit-interop "$H" >/dev/null
rm -rf "$H/work"; mkdir -p "$H/work/in"
for f in "$REPO/examples/conveyor.ldt" "$REPO/examples/verify/melsec-semantics.ldt" "$H/iec.ldt"; do
  n=$(basename "$f" .ldt)
  (cd "$REPO" && go run ./cmd/rungkit export -lang en -to st -name Main "$f") > "$H/work/in/$n.st"
  (cd "$REPO" && go run ./cmd/rungkit export -lang en -to plcopen -name Main "$f") > "$H/work/in/$n.xml"
done
# MSYS_NO_PATHCONV keeps Git Bash on Windows from rewriting the container paths.
MSYS_NO_PATHCONV=1 docker run --rm -v "$H/work:/work" -v "$H/inside.sh:/inside.sh:ro" rungkit-interop sh /inside.sh
