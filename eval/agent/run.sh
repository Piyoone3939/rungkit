#!/bin/sh
# Runs one task with Claude Code in headless mode, allowing only the rungkit MCP tools.
# usage: eval/agent/run.sh MODEL TASK      e.g. eval/agent/run.sh sonnet t1
# Needs: claude (Claude Code) on PATH. Uses your Claude Code account.
set -e
cd "$(dirname "$0")"
H=$(pwd); REPO=$(cd ../.. && pwd)
m=$1; t=$2; W="$H/runs/$m-$t"
[ -x "$H/rungkit" ] || [ -x "$H/rungkit.exe" ] || (cd "$REPO" && go build -o "$H/rungkit$( [ "$(go env GOOS)" = windows ] && echo .exe)" ./cmd/rungkit)
BIN="$H/rungkit"; [ -x "$BIN.exe" ] && BIN="$BIN.exe"
rm -rf "$W"; mkdir -p "$W"
cp "$REPO/examples/conveyor.ldt" "$H/tasks/bug.ldt" "$W/"
# Windows (Git Bash): claude is a Windows program and needs C:/... paths.
winpath() { if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf %s "$1"; fi; }
cat > "$W/mcp.json" <<JSON
{"mcpServers":{"rungkit":{"command":"$(winpath "$BIN")","args":["mcp","-lang","ja","-root","$(winpath "$W")"]}}}
JSON
cd "$W"
# Recent Claude Code defers MCP tools behind a tool search and may start while
# the server is still "pending"; load the tools up front so every run sees them.
ENABLE_TOOL_SEARCH=false claude -p "$(cat "$H/tasks/$t.prompt")" \
  --mcp-config "$W/mcp.json" --strict-mcp-config \
  --allowedTools "mcp__rungkit" \
  --disallowedTools "Bash,Edit,Write,Read,Glob,Grep,WebFetch,WebSearch,NotebookEdit,Agent,Task" \
  --model "$m" --max-turns 40 --output-format json < /dev/null > "$W/result.json"
