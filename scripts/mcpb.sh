#!/bin/sh
# Packs the release binaries as MCP Bundles (.mcpb) and writes server.json for
# the official MCP Registry.
#
# usage: scripts/mcpb.sh VERSION [DIST] [OUT]
#   VERSION  release version without "v" (e.g. 0.2.0)
#   DIST     GoReleaser output with rungkit_<os>_<arch>*/rungkit[.exe] (default: dist)
#   OUT      where to write the bundles and server.json (default: dist/mcpb)
#
# One bundle per OS and architecture (rungkit-mcp-<os>-<arch>.mcpb): the MCPB
# manifest selects by OS only. The registry requires "mcp" in the package URL,
# hence the file names.
set -eu
VERSION=$1
DIST=${2:-dist}
OUT=${3:-dist/mcpb}
REPO=https://github.com/Piyoone3939/rungkit
mkdir -p "$OUT"
OUT=$(cd "$OUT" && pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

packages=""
for dir in "$DIST"/rungkit_*_*; do
  [ -d "$dir" ] || continue
  name=$(basename "$dir")                      # rungkit_linux_amd64_v1
  os=$(echo "$name" | cut -d_ -f2)
  arch=$(echo "$name" | cut -d_ -f3)
  bin=rungkit; platform=$os
  [ "$os" = windows ] && bin=rungkit.exe && platform=win32
  [ -f "$dir/$bin" ] || continue

  work="$TMP/$os-$arch"
  mkdir -p "$work/server"
  cp "$dir/$bin" "$work/server/$bin"
  chmod 755 "$work/server/$bin"
  cat > "$work/manifest.json" <<JSON
{
  "manifest_version": "0.3",
  "name": "rungkit",
  "display_name": "rungkit",
  "version": "$VERSION",
  "description": "Edit PLC ladder logic (MELSEC, IEC 61131-3) and check it on a scan simulator.",
  "author": { "name": "The rungkit Authors", "url": "$REPO" },
  "homepage": "https://piyoone3939.github.io/rungkit/",
  "repository": { "type": "git", "url": "$REPO" },
  "license": "MIT OR Apache-2.0",
  "keywords": ["plc", "ladder", "melsec", "iec-61131-3", "simulator"],
  "server": {
    "type": "binary",
    "entry_point": "server/$bin",
    "mcp_config": { "command": "\${__dirname}/server/$bin", "args": ["mcp"] }
  },
  "compatibility": { "platforms": ["$platform"] }
}
JSON
  file="rungkit-mcp-$os-$arch.mcpb"
  rm -f "$OUT/$file"
  (cd "$work" && zip -q -X -r "$OUT/$file" manifest.json server)
  sum=$(sha256sum "$OUT/$file" | cut -d' ' -f1)
  entry=$(printf '    {\n      "registryType": "mcpb",\n      "identifier": "%s/releases/download/v%s/%s",\n      "fileSha256": "%s",\n      "transport": { "type": "stdio" }\n    }' "$REPO" "$VERSION" "$file" "$sum")
  if [ -z "$packages" ]; then packages=$entry; else packages=$(printf '%s,\n%s' "$packages" "$entry"); fi
  echo "packed $file"
done

[ -n "$packages" ] || { echo "no binaries found in $DIST" >&2; exit 1; }
cat > "$OUT/server.json" <<JSON
{
  "\$schema": "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json",
  "name": "io.github.Piyoone3939/rungkit",
  "title": "rungkit",
  "description": "PLC ladder logic for AI agents: edit over MCP, check on a scan simulator (MELSEC, IEC).",
  "repository": { "url": "$REPO", "source": "github" },
  "websiteUrl": "https://piyoone3939.github.io/rungkit/",
  "version": "$VERSION",
  "packages": [
$packages
  ]
}
JSON
echo "wrote $OUT/server.json"
