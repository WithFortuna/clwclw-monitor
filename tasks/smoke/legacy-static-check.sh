#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "[smoke] root: $ROOT"

files=(
  "agent/clw-agent.js"
  "Claude-Code-Remote/setup.js"
  "Claude-Code-Remote/setup-telegram.sh"
  "Claude-Code-Remote/claude-hook-notify.js"
  "Claude-Code-Remote/claude-remote.js"
  "Claude-Code-Remote/diagnose-automation.js"
  "Claude-Code-Remote/src/core/coordinator-client.js"
  "Claude-Code-Remote/src/channels/telegram/webhook.js"
  "Claude-Code-Remote/src/channels/line/webhook.js"
  "Claude-Code-Remote/src/relay/command-relay.js"
)

echo "[smoke] file existence"
for f in "${files[@]}"; do
  if [[ ! -f "$ROOT/$f" ]]; then
    echo "❌ missing: $f"
    exit 1
  fi
  echo "✅ $f"
done

echo ""
echo "[smoke] node syntax check (all .js under Claude-Code-Remote + agent wrapper)"
node --check "$ROOT/agent/clw-agent.js"

while IFS= read -r -d '' f; do
  node --check "$f"
done < <(
  find "$ROOT/Claude-Code-Remote" \
    -type d \( -name node_modules -o -name .git \) -prune -o \
    -type f -name "*.js" -print0
)

echo ""
echo "✅ static smoke check passed"
