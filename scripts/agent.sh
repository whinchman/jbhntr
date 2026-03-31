#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

SERVICE_NAME="${AGENT_SERVICE:-agent}"
AGENT_TYPE=""
AGENTS_DIR="$PROJECT_DIR/agents"

usage() {
    echo "Usage: $(basename "$0") [OPTIONS]"
    echo ""
    echo "Launch Claude Code autonomously inside the sandboxed container."
    echo ""
    echo "Options:"
    echo "  -t, --type TYPE      Agent type (default: coder)"
    echo "  -p, --prompt TEXT    Custom prompt (overrides default workflow)"
    echo "  -s, --service NAME   Docker Compose service name (default: agent)"
    echo "  --list-types         List available agent types"
    echo "  -h, --help           Show this help"
    echo ""
    echo "Agent Types:"
    echo "  manager        Orchestrates work, decomposes features, reviews/merges"
    echo "  architect      Researches technologies, designs architecture, writes plans"
    echo "  coder          Implements features with TDD (default)"
    echo "  code-reviewer  Reviews completed code for bugs and security issues"
    echo "  designer       UI/UX, design tokens, styling, Figma integration"
    echo "  automation     CI/CD, Dockerfiles, build scripts, deployment"
    echo "  qa             Test strategy, test suites, code review"
    echo ""
    echo "Environment:"
    echo "  AGENT_SERVICE        Override default service name"
    echo "  AGENT_MEMORY         Container memory limit (default: 8G)"
    echo "  AGENT_CPUS           Container CPU limit (default: 4)"
    echo ""
    echo "Examples:"
    echo "  $(basename "$0")                                    # Default coder workflow"
    echo "  $(basename "$0") -t manager                         # Launch manager agent"
    echo "  $(basename "$0") -t qa                              # Launch QA agent"
    echo "  $(basename "$0") -p 'Fix the login bug in src/auth' # Custom task"
    echo "  $(basename "$0") --list-types                       # Show available types"
}

list_types() {
    echo "Available agent types:"
    echo ""
    for dir in "$AGENTS_DIR"/*/; do
        type_name="$(basename "$dir")"
        [ "$type_name" = "_base" ] && continue
        [ -f "$dir/CLAUDE.md" ] || continue
        echo "  $type_name"
    done
}

# Default prompts per agent type
get_default_prompt() {
    case "$1" in
        manager)
            echo "Follow the Manager Workflow in CLAUDE.md. Read the backlog and create task assignments."
            ;;
        coder)
            echo "Follow the Coder Workflow in CLAUDE.md. Find your next pending task and implement it."
            ;;
        designer)
            echo "Follow the Designer Workflow in CLAUDE.md. Find your next pending task and implement it."
            ;;
        automation)
            echo "Follow the Automation Workflow in CLAUDE.md. Find your next pending task and implement it."
            ;;
        architect)
            echo "Follow the Architect Workflow in CLAUDE.md. Find your next pending task, research the problem, and produce a detailed implementation plan."
            ;;
        code-reviewer)
            echo "Follow the Code Reviewer Workflow in CLAUDE.md. Find your next pending review task and review the code for bugs."
            ;;
        qa)
            echo "Follow the QA Workflow in CLAUDE.md. Find your next pending task and implement it."
            ;;
        *)
            echo "Follow the workflow in CLAUDE.md."
            ;;
    esac
}

PROMPT=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -t|--type)
            AGENT_TYPE="$2"
            shift 2
            ;;
        -p|--prompt)
            PROMPT="$2"
            shift 2
            ;;
        -s|--service)
            SERVICE_NAME="$2"
            shift 2
            ;;
        --list-types)
            list_types
            exit 0
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

# Backwards compatibility: if agents/ directory doesn't exist, use legacy behavior
if [ ! -d "$AGENTS_DIR" ]; then
    PROMPT="${PROMPT:-Follow the Autonomous Workflow in CLAUDE.md. Start from Step 0 (pre-flight checks) and work through features in the backlog.}"

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
    exit 0
fi

# Agent type system
AGENT_TYPE="${AGENT_TYPE:-coder}"

# Validate agent type
if [ ! -f "$AGENTS_DIR/$AGENT_TYPE/CLAUDE.md" ]; then
    echo "Error: Unknown agent type '$AGENT_TYPE'"
    echo ""
    list_types
    exit 1
fi

# Assemble CLAUDE.md: concatenate _base + type-specific instructions
ASSEMBLED_CLAUDE=$(mktemp /tmp/claude-agent-XXXXXX.md)
trap "rm -f '$ASSEMBLED_CLAUDE'" EXIT

if [ -f "$AGENTS_DIR/_base/CLAUDE.md" ]; then
    cat "$AGENTS_DIR/_base/CLAUDE.md" >> "$ASSEMBLED_CLAUDE"
    echo "" >> "$ASSEMBLED_CLAUDE"
    echo "---" >> "$ASSEMBLED_CLAUDE"
    echo "" >> "$ASSEMBLED_CLAUDE"
fi
cat "$AGENTS_DIR/$AGENT_TYPE/CLAUDE.md" >> "$ASSEMBLED_CLAUDE"

# Set default prompt for agent type if none provided
PROMPT="${PROMPT:-$(get_default_prompt "$AGENT_TYPE")}"

echo "=== Launching Claude Code (autonomous, sandboxed) ==="
echo "Agent Type: $AGENT_TYPE"
echo "Service: $SERVICE_NAME"
echo "Prompt: $PROMPT"
echo ""

# Build the image if needed
docker compose build "$SERVICE_NAME" 2>/dev/null || true

# Launch with assembled CLAUDE.md mounted over the workspace copy
docker compose run --rm -v "$ASSEMBLED_CLAUDE:/workspace/CLAUDE.md:ro" -e "AGENT_TYPE=$AGENT_TYPE" "$SERVICE_NAME" claude --dangerously-skip-permissions -p "$PROMPT"
