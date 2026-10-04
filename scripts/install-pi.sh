#!/usr/bin/env bash
set -euo pipefail
if [[ $EUID -ne 0 ]]; then echo "Run with sudo." >&2; exit 1; fi
BIN=${1:-./sigwatch}
CONFIG=${2:-./examples/config.yaml}
id sigwatch &>/dev/null || useradd --system --home /var/lib/sigwatch --shell /usr/sbin/nologin sigwatch
install -d -o sigwatch -g sigwatch /opt/sigwatch /etc/sigwatch /var/lib/sigwatch /var/lib/sigwatch/update
install -m 0755 "$BIN" /opt/sigwatch/sigwatch
install -m 0640 -o root -g sigwatch "$CONFIG" /etc/sigwatch/config.yaml
install -m 0644 deploy/systemd/sigwatch.service /etc/systemd/system/sigwatch.service
install -m 0644 deploy/systemd/sigwatch-update.service /etc/systemd/system/sigwatch-update.service
install -m 0644 deploy/systemd/sigwatch-update.path /etc/systemd/system/sigwatch-update.path
if [[ -f VERSION ]]; then install -m 0644 VERSION /opt/sigwatch/VERSION; fi
if [[ -f config.yaml ]]; then install -m 0644 config.yaml /opt/sigwatch/config.example.yaml; fi
if [[ -f scripts/update-pi.sh ]]; then install -m 0755 scripts/update-pi.sh /usr/local/sbin/sigwatch-update; fi
if [[ -f scripts/update-runner.sh ]]; then install -m 0755 scripts/update-runner.sh /usr/local/sbin/sigwatch-update-runner; fi
systemctl daemon-reload
systemctl enable --now sigwatch.service sigwatch-update.path
echo "SigWatch service installed and enabled for reboot."
echo "Dashboard: http://127.0.0.1:8080/"
echo "Manual Chromium kiosk: chromium --kiosk http://127.0.0.1:8080/"
echo "Future stable-release updates: sudo sigwatch-update"
