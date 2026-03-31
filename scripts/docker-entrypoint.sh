#!/bin/bash
set -e

# Start Xvfb if DISPLAY is set and no X server is running
if [ -n "$DISPLAY" ] && ! xdpyinfo -display "$DISPLAY" >/dev/null 2>&1; then
    Xvfb "$DISPLAY" -screen 0 1920x1080x24 &
    sleep 1
fi

exec "$@"
