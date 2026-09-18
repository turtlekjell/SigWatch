#!/usr/bin/env bash
set -euo pipefail
if [[ $EUID -ne 0 ]]; then echo "Run with sudo." >&2; exit 1; fi
BIN=${1:-./sigwatch}
CONFIG=${2:-./examples/config.yaml}
id sigwatch &>/dev/null || useradd --system --home /var/lib/sigwatch --shell /usr/sbin/nologin sigwatch
install -d -o sigwatch -g sigwatch /opt/sigwatch /etc/sigwatch /var/lib/sigwatch
install -m 0755 "$BIN" /opt/sigwatch/sigwatch
install -m 0640 -o root -g sigwatch "$CONFIG" /etc/sigwatch/config.yaml
install -m 0644 deploy/systemd/sigwatch.service /etc/systemd/system/sigwatch.service
systemctl daemon-reload
systemctl enable --now sigwatch.service
echo "SigWatch service installed. Kiosk autostart is documented separately in README.md."
