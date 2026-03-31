# ============================================================================
# Godot Extension — Adds Godot 4.6.1 + Android SDK to the base agent image
# Usage: Copy this Dockerfile to your project root and update docker-compose.yml
# ============================================================================

# --- Stage 1: Base agent with Godot engine + export templates ---------------
FROM ubuntu:noble AS godot-base

USER root
SHELL ["/bin/bash", "-c"]
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git git-lfs unzip wget zip rsync curl jq python3 bash-completion \
    xvfb libgl1 libasound2t64 libpulse0 libx11-6 libxcursor1 libxrandr2 libxi6 fontconfig \
    && rm -rf /var/lib/apt/lists/*

ARG GODOT_VERSION="4.6.1"
ARG RELEASE_NAME="stable"
ARG GODOT_PLATFORM="linux.x86_64"

ENV GODOT_VERSION=${GODOT_VERSION}

RUN wget https://github.com/godotengine/godot-builds/releases/download/${GODOT_VERSION}-${RELEASE_NAME}/Godot_v${GODOT_VERSION}-${RELEASE_NAME}_${GODOT_PLATFORM}.zip \
    && wget https://github.com/godotengine/godot-builds/releases/download/${GODOT_VERSION}-${RELEASE_NAME}/Godot_v${GODOT_VERSION}-${RELEASE_NAME}_export_templates.tpz \
    && mkdir -p ~/.cache ~/.config/godot ~/.local/share/godot/export_templates/${GODOT_VERSION}.${RELEASE_NAME} \
    && unzip Godot_v${GODOT_VERSION}-${RELEASE_NAME}_${GODOT_PLATFORM}.zip \
    && mv Godot_v${GODOT_VERSION}-${RELEASE_NAME}_${GODOT_PLATFORM} /usr/local/bin/godot \
    && unzip Godot_v${GODOT_VERSION}-${RELEASE_NAME}_export_templates.tpz \
    && mv templates/* ~/.local/share/godot/export_templates/${GODOT_VERSION}.${RELEASE_NAME} \
    && rm -f *.zip *.tpz

RUN godot -v -e --quit --headless || true

# --- Stage 2: Android SDK/NDK for mobile exports ---------------------------
FROM godot-base AS android

RUN apt-get update && apt-get install -y --no-install-recommends \
    openjdk-17-jdk-headless adb \
    && rm -rf /var/lib/apt/lists/*

ENV ANDROID_HOME="/usr/lib/android-sdk"

RUN wget https://dl.google.com/android/repository/commandlinetools-linux-7583922_latest.zip \
    && unzip commandlinetools-linux-*_latest.zip -d cmdline-tools \
    && mv cmdline-tools $ANDROID_HOME/ \
    && rm -f commandlinetools-linux-*_latest.zip

ENV PATH="${ANDROID_HOME}/cmdline-tools/cmdline-tools/bin:${PATH}"

RUN yes | sdkmanager --licenses \
    && sdkmanager "platform-tools" "build-tools;33.0.2" "platforms;android-33" "cmdline-tools;latest" "cmake;3.22.1" "ndk;25.2.9519653"

RUN keytool -keyalg RSA -genkeypair -alias androiddebugkey -keypass android -keystore debug.keystore -storepass android -dname "CN=Android Debug,O=Android,C=US" -validity 9999 \
    && mv debug.keystore /root/debug.keystore

RUN echo '[gd_resource type="EditorSettings" format=3]' > ~/.config/godot/editor_settings-${GODOT_VERSION:0:3}.tres \
    && echo '[resource]' >> ~/.config/godot/editor_settings-${GODOT_VERSION:0:3}.tres \
    && echo 'export/android/java_sdk_path = "/usr/lib/jvm/java-17-openjdk-amd64"' >> ~/.config/godot/editor_settings-${GODOT_VERSION:0:3}.tres \
    && echo 'export/android/android_sdk_path = "/usr/lib/android-sdk"' >> ~/.config/godot/editor_settings-${GODOT_VERSION:0:3}.tres \
    && echo 'export/android/debug_keystore = "/root/debug.keystore"' >> ~/.config/godot/editor_settings-${GODOT_VERSION:0:3}.tres \
    && echo 'export/android/debug_keystore_user = "androiddebugkey"' >> ~/.config/godot/editor_settings-${GODOT_VERSION:0:3}.tres \
    && echo 'export/android/debug_keystore_pass = "android"' >> ~/.config/godot/editor_settings-${GODOT_VERSION:0:3}.tres

# --- Stage 3: Dev tools + Claude Code + non-root user ----------------------
FROM android AS dev

RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

ARG USER_UID=1000
ARG USER_GID=1000

RUN (getent group ${USER_GID} >/dev/null 2>&1 || groupadd --gid ${USER_GID} godot) \
    && (getent passwd ${USER_UID} >/dev/null 2>&1 && usermod -l godot -d /home/godot -m $(getent passwd ${USER_UID} | cut -d: -f1) || useradd --uid ${USER_UID} --gid ${USER_GID} -m godot) \
    && mkdir -p /home/godot/.cache /home/godot/.config/godot /home/godot/.local/share/godot \
    && cp -r /root/.local/share/godot/export_templates /home/godot/.local/share/godot/ \
    && cp -r /root/.config/godot/* /home/godot/.config/godot/ \
    && cp /root/debug.keystore /home/godot/debug.keystore \
    && sed -i 's|/root/debug.keystore|/home/godot/debug.keystore|g' /home/godot/.config/godot/editor_settings-${GODOT_VERSION:0:3}.tres \
    && chown -R $(getent passwd ${USER_UID} | cut -d: -f1) /home/godot

COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

USER godot
WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["bash"]
