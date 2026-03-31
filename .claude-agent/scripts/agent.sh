#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

SERVICE_NAME="${AGENT_SERVICE:-agent}"
DEFAULT_PROMPT="Follow the Autonomous Workflow in CLAUDE.md. Start from Step 0 (pre-flight checks) and work through features in the backlog."

usage() {
    echo "Usage: $(basename "$0") [OPTIONS]"
    echo ""
    echo "Launch Claude Code autonomously inside the sandboxed container."
    echo ""
    echo "Options:"
    echo "  -p, --prompt TEXT    Custom prompt (overrides default workflow)"
    echo "  -s, --service NAME   Docker Compose service name (default: agent)"
    echo "  -h, --help           Show this help"
    echo ""
    echo "Environment:"
    echo "  AGENT_SERVICE        Override default service name"
    echo "  AGENT_MEMORY         Container memory limit (default: 8G)"
    echo "  AGENT_CPUS           Container CPU limit (default: 4)"
    echo ""
    echo "Examples:"
    echo "  $(basename "$0")                                    # Default workflow"
    echo "  $(basename "$0") -p 'Fix the login bug in src/auth' # Custom task"
    echo "  $(basename "$0") -s my-service                      # Custom service"
}

PROMPT=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -p|--prompt)
            PROMPT="$2"
            shift 2
            ;;
        -s|--service)
            SERVICE_NAME="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            PROMPT="$1"
            shift
            ;;
    esac
done

PROMPT="${PROMPT:-$DEFAULT_PROMPT}"

# Ensure container is running
if ! docker compose ps --status running 2>/dev/null | grep -q "$SERVICE_NAME"; then
    echo "Starting container..."
    docker compose up -d "$SERVICE_NAME"
    sleep 2
fi

echo "=== Launching Claude Code (autonomous, sandboxed) ==="
echo "Service: $SERVICE_NAME"
echo "Prompt: $PROMPT"
echo ""

docker compose exec "$SERVICE_NAME" claude --dangerously-skip-permissions -p "$PROMPT"
