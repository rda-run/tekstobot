#!/usr/bin/env bash
# Runs before tekstobot starts: align whisper.service with TRANSCRIBER_BACKEND in /etc/tekstobot.env.
set -euo pipefail

ENV_FILE="/etc/tekstobot.env"
BACKEND="local"

if [[ -f "$ENV_FILE" ]]; then
	line="$(grep -E '^[[:space:]]*TRANSCRIBER_BACKEND[[:space:]]*=' "$ENV_FILE" 2>/dev/null | tail -n1 || true)"
	if [[ -n "${line}" ]]; then
		val="${line#*=}"
		val="${val%%#*}"
		val="$(echo "${val}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
		val="${val#\"}"
		val="${val%\"}"
		val="${val#\'}"
		val="${val%\'}"
		val="$(echo "${val}" | tr '[:upper:]' '[:lower:]')"
		if [[ -n "${val}" ]]; then
			BACKEND="${val}"
		fi
	fi
fi

case "${BACKEND}" in
cloudflare)
	systemctl stop whisper.service 2>/dev/null || true
	;;
*)
	systemctl start whisper.service
	;;
esac
