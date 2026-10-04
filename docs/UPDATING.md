# Updating SigWatch on Raspberry Pi / Linux

SigWatch v1 is designed to use an explicit, administrator-invoked update command rather than unattended background updates.

After a normal Pi installation, the helper is installed as:

```bash
/usr/local/sbin/sigwatch-update
```


## Settings-page update

On Raspberry Pi/systemd installations created by `scripts/install-pi.sh`, the dashboard Settings panel provides **Software Update** controls:

1. **Check for Updates** queries the public GitHub repository tags and compares the newest stable `vX.Y.Z` tag with the running binary version.
2. **Install Update** appears only when a newer stable release exists and the privileged update helper is installed.
3. The browser sends a fixed `install latest stable` request; it cannot supply shell commands, alternate URLs, or arbitrary target versions.
4. A root-owned `sigwatch-update.path` / `sigwatch-update.service` pair notices the request and runs `/usr/local/sbin/sigwatch-update-runner`.
5. The runner invokes the existing rollback-capable `/usr/local/sbin/sigwatch-update`.
6. The browser polls local update status and reloads after the new SigWatch service becomes healthy.

The SigWatch HTTP service itself continues to run as the unprivileged `sigwatch` account. Desktop/development runs without the systemd helper can still **check** stable tags, but installation from the Settings page is disabled.

## Normal update

```bash
sudo sigwatch-update
```

The updater:

1. Reads the newest stable `vX.Y.Z` Git tag from the public SigWatch GitHub repository.
2. Compares it with the installed binary version.
3. Checks out that exact release tag into a temporary directory.
4. Builds the release locally with the version embedded in the binary.
5. Validates the existing `/etc/sigwatch/config.yaml` with the new binary.
6. Backs up the currently installed binary and systemd service definition.
7. Installs the new binary/service definition and restarts `sigwatch.service`.
8. Verifies that systemd reports the service active and `/healthz` responds.
9. Automatically restores the prior binary/service definition if the new version does not become healthy.

The updater **never replaces `/etc/sigwatch/config.yaml`**. A release's current example configuration is copied to `/opt/sigwatch/config.example.yaml` for comparison only.

## Check without updating

```bash
sigwatch-update --check
```

## Install a specific stable version

```bash
sudo sigwatch-update --version 1.0.1
```

or:

```bash
sudo sigwatch-update --version v1.0.1
```

## Reinstall the current release

```bash
sudo sigwatch-update --force
```

## Requirements

The source-based v1 updater requires `git`, Go, `curl`, and systemd on the target machine. Raspberry Pi OS Bookworm installations used to build/install SigWatch already satisfy the intended environment once those development tools are installed.

Only stable semantic-version tags matching `vX.Y.Z` are considered by the default update check. Development commits on `main` are deliberately ignored.

## Why releases instead of `git pull main`?

A dashboard appliance should update to a known release, not an arbitrary development commit. Stable Git tags provide a repeatable source snapshot and give the updater a clear target version. Future releases may switch to signed/prebuilt GitHub release artifacts while preserving the same `sigwatch-update` user interface.
