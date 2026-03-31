#!/bin/bash

# TekstoBot RPM Post-install Script
# Detects GPU support and configures Quadlets

QUADLET_DIR="/etc/containers/systemd"
TEMPLATE_DIR="/usr/share/tekstobot/deploy"

mkdir -p "$QUADLET_DIR"

# $1 == 1: Install
# $1 == 2: Upgrade
if [ "$1" = "1" ]; then
    echo "Initial installation detected. Configuring services..."
elif [ "$1" = "2" ]; then
    echo "Upgrade detected. Re-verifying configuration..."
fi

if [ -x /usr/bin/nvidia-ctk ] || [ -f /usr/lib64/libcuda.so ]; then
    echo "NVIDIA detected. Configuring Whisper for GPU (CUDA)..."
    cp "$TEMPLATE_DIR/whisper-gpu.container" "$QUADLET_DIR/whisper.container"
else
    echo "No NVIDIA GPU detected. Falling back to Whisper CPU mode..."
    cp "$TEMPLATE_DIR/whisper-cpu.container" "$QUADLET_DIR/whisper.container"
fi

# Set permissions
chmod 644 "$QUADLET_DIR/whisper.container"

# Always install the bot container as well
# (Removing old tekstobot container copy as it's now a native systemd unit)

# Ensure data directories exist for the native service and whisper container
mkdir -p /var/lib/tekstobot/data
mkdir -p /var/lib/tekstobot/whisper/cache
chmod -R 755 /var/lib/tekstobot

echo "Reloading systemd to activate Quadlets..."
systemctl daemon-reload

echo "TekstoBot Quadlets are now active in $QUADLET_DIR."
echo "Note: You must edit /etc/tekstobot.env to configure your database before starting."
