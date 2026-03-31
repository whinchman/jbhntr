# Extending the Dockerfile

The base Dockerfile includes Ubuntu, git, Node.js, and Claude Code CLI. To add your project's language runtime, test framework, or build tools, you replace the base Dockerfile with an extended one.

## Option 1: Use an example

Check `examples/` for a ready-made Dockerfile for your stack:

```
cp examples/python/Dockerfile .
docker compose build
```

## Option 2: Write your own

Start from the base and add your tools. Here's the pattern:

```dockerfile
FROM ubuntu:noble AS base

USER root
SHELL ["/bin/bash", "-c"]
ENV DEBIAN_FRONTEND=noninteractive

# --- System packages (keep the base set + add yours) ---
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git git-lfs unzip wget zip rsync curl jq python3 bash-completion \
    xvfb libgl1 libx11-6 libxcursor1 libxrandr2 libxi6 \
    # ↓↓↓ ADD YOUR PACKAGES HERE ↓↓↓
    your-language-runtime \
    your-build-tools \
    && rm -rf /var/lib/apt/lists/*

# --- Node.js 22 (required for Claude Code CLI — do not remove) ---
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# --- Claude Code CLI (do not remove) ---
RUN npm install -g @anthropic-ai/claude-code

# --- Your toolchain setup ---
# RUN curl ... install your language
# RUN pip install ... or cargo install ... or go install ...

# --- Non-root user (do not remove) ---
ARG USER_UID=1000
ARG USER_GID=1000
ARG USERNAME=agent

RUN (getent group ${USER_GID} >/dev/null 2>&1 || groupadd --gid ${USER_GID} ${USERNAME}) \
    && (getent passwd ${USER_UID} >/dev/null 2>&1 && usermod -l ${USERNAME} -d /home/${USERNAME} -m $(getent passwd ${USER_UID} | cut -d: -f1) || useradd --uid ${USER_UID} --gid ${USER_GID} -m ${USERNAME}) \
    && mkdir -p /home/${USERNAME}/.cache \
    && chown -R ${USER_UID}:${USER_GID} /home/${USERNAME}

COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

USER ${USERNAME}
WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["bash"]
```

### What to keep

These lines are required for the agent to function:

- **Node.js 22**: Claude Code CLI is a Node.js application
- **Claude Code CLI**: `npm install -g @anthropic-ai/claude-code`
- **Non-root user block**: The container should not run as root
- **Entrypoint**: `docker-entrypoint.sh` handles Xvfb startup
- **git**: The autonomous workflow uses git extensively

### What to add

Anything your project needs to build and test:

- Language runtimes (Python, Go, Java, Ruby, etc.)
- Package managers (pip, cargo, go modules, maven, etc.)
- Build tools (make, cmake, gradle, etc.)
- Test frameworks (if not installable via package manager)
- System libraries your code depends on

### Multi-stage builds

For heavy toolchains (like Godot + Android SDK), use multi-stage builds to keep the image organized. See `examples/godot/Dockerfile` for an example with three stages (base, android, dev).

## After modifying

Rebuild the container:

```
docker compose build
```

Then launch as usual:

```
scripts/agent.sh
```

## Verifying your setup

Test that your toolchain works inside the container:

```
docker compose run agent bash
```

Then inside the container:

```bash
# Verify Claude Code
claude --version

# Verify your language
python3 --version   # or: cargo --version, go version, etc.

# Verify your test runner
pytest --version     # or: cargo test --help, go test --help, etc.
```
