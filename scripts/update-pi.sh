#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${SIGWATCH_REPO_URL:-https://github.com/TurtleKjell/SigWatch.git}"
BIN="/opt/sigwatch/sigwatch"
CONFIG="/etc/sigwatch/config.yaml"
UNIT="/etc/systemd/system/sigwatch.service"
SERVICE="sigwatch.service"
BACKUP_DIR="/var/lib/sigwatch/backups"
CHECK_ONLY=0
FORCE=0
REQUESTED_VERSION=""

usage() {
  cat <<'USAGE'
Usage: sigwatch-update [options]

Update an installed SigWatch Raspberry Pi/Linux service from a stable Git tag.
The installed /etc/sigwatch/config.yaml is validated but never overwritten.

Options:
  --check              Show installed and latest stable versions; make no changes.
  --version VERSION    Install a specific stable version, e.g. 1.0.1 or v1.0.1.
  --repo URL           Override the Git repository URL.
  --force              Reinstall even when the requested version is already installed.
  -h, --help           Show this help.

Environment:
  SIGWATCH_REPO_URL    Alternate repository URL (same effect as --repo).
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)
      CHECK_ONLY=1
      shift
      ;;
    --version)
      [[ $# -ge 2 ]] || { echo "--version requires a value" >&2; exit 2; }
      REQUESTED_VERSION="$2"
      shift 2
      ;;
    --repo)
      [[ $# -ge 2 ]] || { echo "--repo requires a value" >&2; exit 2; }
      REPO_URL="$2"
      shift 2
      ;;
    --force)
      FORCE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

for cmd in git awk sort tail sed; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Missing required command: $cmd" >&2; exit 1; }
done

installed_version="unknown"
if [[ -x "$BIN" ]]; then
  installed_version="$($BIN -version 2>/dev/null | awk 'NR==1 {print $2}' || true)"
  [[ -n "$installed_version" ]] || installed_version="unknown"
fi

stable_tags() {
  git ls-remote --refs --tags "$REPO_URL" 'v*' 2>/dev/null \
    | awk -F/ '{print $3}' \
    | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
    | sort -V
}

if [[ -n "$REQUESTED_VERSION" ]]; then
  target_tag="$REQUESTED_VERSION"
  [[ "$target_tag" == v* ]] || target_tag="v$target_tag"
  if ! stable_tags | grep -Fxq "$target_tag"; then
    echo "Stable release tag not found: $target_tag" >&2
    exit 1
  fi
else
  target_tag="$(stable_tags | tail -n 1 || true)"
  if [[ -z "$target_tag" ]]; then
    echo "No stable SigWatch release tags were found in $REPO_URL" >&2
    exit 1
  fi
fi

target_version="${target_tag#v}"
printf 'Installed: %s\nLatest/target: %s\nRepository: %s\n' "$installed_version" "$target_version" "$REPO_URL"

if [[ $CHECK_ONLY -eq 1 ]]; then
  exit 0
fi

if [[ $EUID -ne 0 ]]; then
  echo "Run updates with sudo: sudo sigwatch-update" >&2
  exit 1
fi

for cmd in go curl systemctl install mktemp cp date; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Missing required command: $cmd" >&2; exit 1; }
done

if [[ ! -x "$BIN" || ! -f "$CONFIG" ]]; then
  echo "SigWatch does not appear to be installed under /opt/sigwatch and /etc/sigwatch." >&2
  echo "Run scripts/install-pi.sh first." >&2
  exit 1
fi

if [[ "$installed_version" == "$target_version" && $FORCE -ne 1 ]]; then
  echo "SigWatch is already at $target_version. Nothing to do."
  exit 0
fi

install -d -m 0755 "$BACKUP_DIR"
workdir="$(mktemp -d /var/lib/sigwatch/update.XXXXXX)"
trap 'rm -rf "$workdir"' EXIT
src="$workdir/src"
newbin="$workdir/sigwatch.new"

echo "Fetching $target_tag ..."
git clone --quiet --depth 1 --branch "$target_tag" "$REPO_URL" "$src"

release_file_version="$(tr -d '[:space:]' < "$src/VERSION")"
if [[ "$release_file_version" != "$target_version" ]]; then
  echo "Release mismatch: tag $target_tag contains VERSION=$release_file_version" >&2
  exit 1
fi

echo "Building SigWatch $target_version ..."
(
  cd "$src"
  go build -trimpath -ldflags "-s -w -X main.version=$target_version" -o "$newbin" ./cmd/sigwatch
)

echo "Validating existing configuration ..."
"$newbin" -config "$CONFIG" -check

stamp="$(date +%Y%m%d-%H%M%S)"
safe_current="$(printf '%s' "$installed_version" | sed 's/[^A-Za-z0-9._-]/_/g')"
backup_bin="$BACKUP_DIR/sigwatch-${safe_current}-${stamp}"
backup_unit=""
cp -p "$BIN" "$backup_bin"
if [[ -f "$UNIT" ]]; then
  backup_unit="$BACKUP_DIR/sigwatch.service-${stamp}"
  cp -p "$UNIT" "$backup_unit"
fi

rollback() {
  echo "Update failed; restoring the previous SigWatch binary ..." >&2
  install -m 0755 "$backup_bin" "$BIN"
  if [[ -n "$backup_unit" && -f "$backup_unit" ]]; then
    install -m 0644 "$backup_unit" "$UNIT"
  fi
  systemctl daemon-reload || true
  systemctl restart "$SERVICE" || true
}

# Install the new executable and service definition. The user configuration is
# intentionally left untouched.
install -m 0755 "$newbin" "$BIN"
install -m 0644 "$src/deploy/systemd/sigwatch.service" "$UNIT"
systemctl daemon-reload

if ! systemctl restart "$SERVICE"; then
  rollback
  exit 1
fi

health_url="http://127.0.0.1:8080/healthz"
listen_value="$(awk '/^listen:[[:space:]]*/ {sub(/^listen:[[:space:]]*/, ""); gsub(/[\"\047]/, ""); print; exit}' "$CONFIG")"
if [[ "$listen_value" =~ ^(127\.0\.0\.1|localhost):[0-9]+$ ]]; then
  health_url="http://${listen_value}/healthz"
fi

healthy=0
for _ in {1..15}; do
  if systemctl is-active --quiet "$SERVICE" && curl -fsS "$health_url" >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 1
done

if [[ $healthy -ne 1 ]]; then
  echo "SigWatch did not become healthy at $health_url." >&2
  rollback
  exit 1
fi

# Only replace updater components after the new SigWatch service has proven healthy.
install -m 0755 "$src/scripts/update-pi.sh" /usr/local/sbin/sigwatch-update
install -m 0755 "$src/scripts/update-runner.sh" /usr/local/sbin/sigwatch-update-runner
install -m 0644 "$src/deploy/systemd/sigwatch-update.service" /etc/systemd/system/sigwatch-update.service
install -m 0644 "$src/deploy/systemd/sigwatch-update.path" /etc/systemd/system/sigwatch-update.path
install -m 0644 "$src/VERSION" /opt/sigwatch/VERSION
install -m 0644 "$src/config.yaml" /opt/sigwatch/config.example.yaml
systemctl daemon-reload
systemctl enable --now sigwatch-update.path

# Keep the three most recent binary backups.
mapfile -t old_backups < <(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'sigwatch-*' -printf '%T@ %p\n' 2>/dev/null | sort -nr | awk 'NR>3 {$1=""; sub(/^ /, ""); print}')
if [[ ${#old_backups[@]} -gt 0 ]]; then
  rm -f -- "${old_backups[@]}"
fi

echo "SigWatch updated successfully: $installed_version -> $target_version"
echo "Configuration preserved: $CONFIG"
echo "Backup: $backup_bin"
echo "Health: $health_url"
