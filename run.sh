#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Source .env if present
if [ -f "${SCRIPT_DIR}/.env" ]; then
  set -a
  # shellcheck source=/dev/null
  source "${SCRIPT_DIR}/.env"
  set +a
fi

CONFIG="${SCRIPT_DIR}/config.yaml"
BINARY="${SCRIPT_DIR}/bin/jobhuntr"

# Check binary exists
if [ ! -f "$BINARY" ]; then
  echo "Binary not found at $BINARY — building..."
  cd "$SCRIPT_DIR"
  go build -o bin/jobhuntr ./cmd/jobhuntr
fi

# Bootstrap config from example if missing
if [ ! -f "$CONFIG" ]; then
  echo "No config.yaml found — copying from config.yaml.example"
  cp "${SCRIPT_DIR}/config.yaml.example" "$CONFIG"
fi

# Warn on missing env vars used in config
missing=()
[ -z "${ANTHROPIC_API_KEY:-}" ] && missing+=("ANTHROPIC_API_KEY")
[ -z "${SERPAPI_KEY:-}" ]       && missing+=("SERPAPI_KEY")
[ -z "${NTFY_TOPIC:-}" ]        && missing+=("NTFY_TOPIC")

if [ ${#missing[@]} -gt 0 ]; then
  echo "Warning: the following env vars are unset (set them or edit config.yaml):"
  for v in "${missing[@]}"; do echo "  $v"; done
fi

exec "$BINARY" --config "$CONFIG" "$@"
