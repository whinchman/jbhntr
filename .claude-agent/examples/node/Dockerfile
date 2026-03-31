# ============================================================================
# Node/TypeScript Extension — Node.js is already in the base image.
# This Dockerfile just adds pnpm and project dependency installation.
# Usage: Copy this Dockerfile to your project root and rebuild
# ============================================================================

FROM ubuntu:noble AS base

USER root
SHELL ["/bin/bash", "-c"]
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git git-lfs unzip wget zip rsync curl jq python3 bash-completion \
    xvfb libgl1 libx11-6 libxcursor1 libxrandr2 libxi6 \
    && rm -rf /var/lib/apt/lists/*

# Node.js 22
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# pnpm (optional — remove if you use npm or yarn)
RUN npm install -g pnpm

# Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Non-root user
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
