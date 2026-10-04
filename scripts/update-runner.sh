#!/usr/bin/env bash
set -euo pipefail

UPDATE_DIR=/var/lib/sigwatch/update
REQUEST="$UPDATE_DIR/request"
STATUS="$UPDATE_DIR/status.json"
LOG="$UPDATE_DIR/update.log"
UPDATER=/usr/local/sbin/sigwatch-update

install -d -o sigwatch -g sigwatch -m 0755 "$UPDATE_DIR"
action="$(tr -d '[:space:]' < "$REQUEST" 2>/dev/null || true)"
rm -f "$REQUEST"

write_status() {
  local state="$1" message="$2" version="${3:-}"
  local now
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  # Values written here are controlled by this script/release metadata only.
  printf '{"state":"%s","message":"%s","version":"%s","updated_at":"%s"}\n' \
    "$state" "$message" "$version" "$now" > "$STATUS"
  chmod 0644 "$STATUS"
}

if [[ "$action" != "install" ]]; then
  write_status failed "Unknown update request"
  exit 1
fi

if [[ ! -x "$UPDATER" ]]; then
  write_status failed "Updater is not installed"
  exit 1
fi

write_status installing "Installing latest stable release"
if "$UPDATER" >"$LOG" 2>&1; then
  version="unknown"
  if [[ -f /opt/sigwatch/VERSION ]]; then
    version="$(tr -d '[:space:]' < /opt/sigwatch/VERSION)"
  fi
  write_status complete "Update installed successfully" "$version"
else
  write_status failed "Update failed; previous version was restored if needed"
  exit 1
fi
