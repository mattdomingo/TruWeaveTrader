#!/usr/bin/env bash
set -euo pipefail

# Restart automation: stop any previous run, clear stale state, start fresh

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$ROOT_DIR/alpaca-tui"
STATE_FILE="$HOME/.alpaca-tui/automation_state.json"

info() { printf "[INFO] %s\n" "$*"; }
warn() { printf "[WARN] %s\n" "$*"; }

# Build the binary if missing
if [[ ! -x "$BINARY" ]]; then
  info "Building alpaca-tui binary..."
  (cd "$ROOT_DIR" && go build -o alpaca-tui .)
fi

# Try to read PID from state file
if [[ -f "$STATE_FILE" ]]; then
  PID="$(awk -F: '/"pid"/ {gsub(/[ ,]/,"",$2); print $2; exit}' "$STATE_FILE" || true)"
  if [[ -n "${PID:-}" && "$PID" =~ ^[0-9]+$ ]]; then
    if ps -p "$PID" >/dev/null 2>&1; then
      info "Stopping previous automation process PID $PID..."
      kill -TERM "$PID" || true
      # Wait up to 15s for graceful exit
      for _ in $(seq 1 15); do
        if ! ps -p "$PID" >/dev/null 2>&1; then
          break
        fi
        sleep 1
      done
      if ps -p "$PID" >/dev/null 2>&1; then
        warn "Process still running, sending SIGKILL..."
        kill -KILL "$PID" || true
      fi
    else
      warn "State file PID $PID not running (stale)."
    fi
  else
    warn "Could not parse PID from $STATE_FILE (stale or malformed)."
  fi
  info "Removing stale state file..."
  rm -f "$STATE_FILE" || true
fi

# Ensure strategies are enabled unless explicitly overridden
export STRATEGIES_ENABLED="${STRATEGIES_ENABLED:-true}"

# Optional: run detached by setting DETACH=1
if [[ "${DETACH:-0}" == "1" ]]; then
  LOG_FILE="${LOG_FILE:-$ROOT_DIR/automation.log}"
  info "Starting automation in background (logs: $LOG_FILE)..."
  nohup "$BINARY" auto start >"$LOG_FILE" 2>&1 &
  echo $! >"$ROOT_DIR/.automation.pid"
  info "Started PID $(cat "$ROOT_DIR/.automation.pid")"
else
  info "Starting automation in foreground... (Ctrl+C to stop)"
  exec "$BINARY" auto start
fi


