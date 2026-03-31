# ============================================================================
# JobHuntr Agent Container
# Go 1.24 + Chromium (for go-rod PDF gen) + Claude Code CLI
# ============================================================================

FROM ubuntu:noble AS base

USER root
SHELL ["/bin/bash", "-c"]
ENV DEBIAN_FRONTEND=noninteractive

# Core system packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    git-lfs \
    unzip \
    wget \
    zip \
    rsync \
    curl \
    jq \
    python3 \
    bash-completion \
    xvfb \
    libgl1 \
    libx11-6 \
    libxcursor1 \
    libxrandr2 \
    libxi6 \
    build-essential \
    gnupg \
    && rm -rf /var/lib/apt/lists/*

# Chromium via playwright's known-good build (avoids snap shim in Noble)
# go-rod will auto-download its own browser binary at first use, but we
# pre-install Google Chrome stable as a reliable fallback.
RUN curl -fsSL https://dl.google.com/linux/linux_signing_key.pub | gpg --dearmor -o /usr/share/keyrings/google-chrome.gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/google-chrome.gpg] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google-chrome.list \
    && apt-get update && apt-get install -y --no-install-recommends \
    google-chrome-stable \
    fonts-liberation \
    libnss3 \
    libatk-bridge2.0-0 \
    libdrm2 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libgbm1 \
    libasound2t64 \
    libpango-1.0-0 \
    libcairo2 \
    && rm -rf /var/lib/apt/lists/*

# Go 1.24
ARG GO_VERSION=1.24.2
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/home/agent/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Node.js 22 (required for Claude Code CLI)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Claude Code CLI
ARG CLAUDE_VERSION="latest"
RUN if [ "$CLAUDE_VERSION" = "latest" ]; then npm install -g @anthropic-ai/claude-code; else npm install -g @anthropic-ai/claude-code@${CLAUDE_VERSION}; fi

# Non-root user with parameterized UID/GID
ARG USER_UID=1000
ARG USER_GID=1000
ARG USERNAME=agent

RUN (getent group ${USER_GID} >/dev/null 2>&1 || groupadd --gid ${USER_GID} ${USERNAME}) \
    && (getent passwd ${USER_UID} >/dev/null 2>&1 && usermod -l ${USERNAME} -d /home/${USERNAME} -m $(getent passwd ${USER_UID} | cut -d: -f1) || useradd --uid ${USER_UID} --gid ${USER_GID} -m ${USERNAME}) \
    && mkdir -p /home/${USERNAME}/.cache /home/${USERNAME}/go \
    && chown -R ${USER_UID}:${USER_GID} /home/${USERNAME}

COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

USER ${USERNAME}
WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["bash"]
