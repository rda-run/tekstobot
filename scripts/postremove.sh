#!/bin/bash

# TekstoBot RPM Post-remove Script
# Cleans up files and reloads systemd

QUADLET_DIR="/etc/containers/systemd"

echo "Cleaning up TekstoBot Quadlets..."
rm -f "$QUADLET_DIR/whisper.container"

echo "Reloading systemd..."
systemctl daemon-reload

echo "TekstoBot removal clean up finished."
