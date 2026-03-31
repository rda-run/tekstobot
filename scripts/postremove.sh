#!/bin/bash

# TekstoBot RPM Post-remove Script
# Cleans up files and reloads systemd

# $1 == 0: Full uninstall
# $1 >= 1: Upgrade (skip cleanup)

if [ "$1" = "0" ]; then
    QUADLET_DIR="/etc/containers/systemd"
    echo "Uninstall detected. Cleaning up TekstoBot Quadlets..."
    rm -f "$QUADLET_DIR/whisper.container"
    
    echo "Reloading systemd..."
    systemctl daemon-reload
else
    echo "Upgrade detected. Skipping cleanup of Quadlets."
fi

echo "TekstoBot removal script finished."
