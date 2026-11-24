#!/bin/sh
# Builds the browser version into web/dist (serve that folder as static files).
# usage: web/build.sh [version]
set -e
cd "$(dirname "$0")/.."
D=web/dist
rm -rf "$D"; mkdir -p "$D/examples"
GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w -X main.version=${1:-dev}" -o "$D/rungkit.wasm" ./cmd/rungkit-web
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$D/"
cp web/index.html web/style.css web/app.js "$D/"
cp examples/conveyor.ldt examples/conveyor.ldtest examples/verify/melsec-semantics.ldt examples/verify/melsec-semantics.ldtest "$D/examples/"
cp examples/iec-motor.ldt "$D/examples/iec.ldt"
cp examples/iec-motor.ldtest "$D/examples/iec.ldtest"
echo "built $D ($(du -h "$D/rungkit.wasm" | cut -f1) wasm)"
