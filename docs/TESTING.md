# SigWatch Release-Candidate Test Plan

This test plan covers the current 0.1.x release-candidate baseline.

## 1. Full local verification

```bash
make verify
```

Equivalent commands:

```bash
go test ./...
go vet ./...
go run ./cmd/sigwatch -config ./config.yaml -check
```

## 2. Desktop smoke test

```bash
go run ./cmd/sigwatch -config ./config.yaml
```

Open `http://127.0.0.1:8080/` and confirm:

- Default region loads.
- Northwest, Southwest, Midwest, Southeast, Northeast, National / Radio, and Prototype regions all switch cleanly.
- Region switching works and persists after browser reload.
- Local and National / Radio viewport regions fit their 4 x 3 grids without page scrolling at the intended landscape resolution.
- Image widgets render and eligible images can be enlarged/closed.
- Weather, earthquake, fire, coastal, camera, clock, links, and system widgets used by the checked-in configuration render without breaking unrelated widgets.
- `/healthz` returns an OK response.

## 3. Theme smoke test

For each bundled theme (`default`, `light`, `midnight`, `amber`):

1. Set the root `theme` value in `config.yaml`.
2. Run `go run ./cmd/sigwatch -config ./config.yaml -check`.
3. Start SigWatch and spot-check Local and National / Radio.
4. Confirm text, borders, freshness states, links, controls, images, cameras, and dialogs remain legible.
5. Restore the desired default theme when finished.

Automated tests also verify that all four bundled themes are embedded and accepted at startup.

## 4. Configuration validation

```bash
go run ./cmd/sigwatch -config ./config.yaml -check
```

When changing config parsing or validation, exercise representative failures such as:

- Unknown key.
- Non-loopback `listen` address.
- Unknown timezone.
- Duplicate widget ID.
- Missing required coordinates/provider settings.
- Invalid viewport rows/columns.
- `warning_after >= expire_after`.

## 5. Refresh isolation

Confirm externally refreshed widgets update independently and that the page does not routinely reload.

Use the region-level Refresh control and verify supported external widgets update without restarting camera embeds or breaking unrelated widgets.

## 6. Persistent cache and offline behavior

1. Start SigWatch online and let cacheable widgets load successfully.
2. Stop SigWatch.
3. Disconnect Internet access.
4. Restart SigWatch using the same cache directory.
5. Confirm last-known-good cacheable content is restored.
6. Confirm freshness indicators age into stale/expired states according to policy.
7. Confirm one failed provider does not prevent unrelated widgets or the server from running.

For a fast freshness test, temporarily use short thresholds, then restore normal values afterward.

## 7. Structured-provider checks

### Earthquake

Confirm center/radius, minimum magnitude, time window, event count, and event links behave as configured.

### Fire

Confirm radius, `named_only`, `min_acres`, event count, containment display, incident links, and empty/no-current-incident behavior.

### Coastal

Confirm tide predictions and seven-day forecast both render. Test or simulate a failure of one source and verify the other half can remain current/available independently.

### Camera

Confirm both bundled camera tiles in each geographic region render in the browser, and verify autoplay/mute/control settings behave as expected for the browser environment. Because third-party livestream IDs can rotate, a failed camera should be treated as a configuration/source-maintenance issue rather than a failure of unrelated widgets. Spot-check the two Huntington Beach camera tiles and the Seattle Space Needle example. A single unavailable public livestream must not break the rest of the region.

### National / Radio

Confirm the current national NOAA imagery loads, the HAMQSL panels are legible, UTC/system/link widgets render, and no retired Maps-only layout behavior is required. A single unavailable image source must not break the other eleven tiles.

## 8. Loopback security check

From the SigWatch host:

```bash
curl http://127.0.0.1:8080/healthz
```

From another LAN host, direct access to port 8080 should fail under the default loopback-only configuration.

## 9. Cross-build

```bash
make cross
file dist/sigwatch-linux-arm64 dist/sigwatch-linux-amd64
```

## 10. Raspberry Pi OS Bookworm

On the Pi:

1. Build SigWatch from a clean clone with `make build` and confirm `./sigwatch -version` matches `VERSION`.
2. Validate `config.yaml`.
3. Install with `sudo ./scripts/install-pi.sh ./sigwatch ./config.yaml`.
4. Confirm `systemctl status sigwatch --no-pager` reports the service active.
5. Reboot and confirm the SigWatch backend starts automatically.
6. Launch the dashboard manually with `chromium --kiosk http://127.0.0.1:8080/`.
7. Confirm the Local viewport fits the display and both video streams remain usable.
8. Confirm persistent cache survives a service/device restart.
9. Reboot without Internet and confirm SigWatch starts rather than crashing.

Automatic Chromium launch is optional for the v1 baseline and can be documented separately after a preferred Bookworm desktop-session method is settled.

### Pi update helper

After a tagged test release is available:

1. Run `sigwatch-update --check` and confirm the installed/target versions are reported without modification.
2. Run `sudo sigwatch-update --version <test-version>`.
3. Confirm `/etc/sigwatch/config.yaml` is byte-for-byte unchanged.
4. Confirm the new binary reports the expected version with `/opt/sigwatch/sigwatch -version`.
5. Confirm `sigwatch.service` is active and `/healthz` succeeds.
6. Test an intentionally invalid/incompatible config in a disposable environment and confirm the updater stops before replacing the installed binary.
7. Test a failed health check in a disposable environment and confirm rollback restores the previous binary.

### Settings-page updater

On a disposable/test Pi install:

1. Open **Settings → Software Update** and confirm the running version is displayed.
2. Select **Check for Updates** and confirm only stable `vX.Y.Z` tags are considered.
3. Confirm the Install button is hidden when no newer stable release exists.
4. With a newer test tag available, confirm Install requests the update and the browser survives/reconnects across the service restart.
5. Confirm `/etc/sigwatch/config.yaml` remains unchanged.
6. Confirm `/var/lib/sigwatch/update/status.json` reaches `complete` on success.
7. Confirm a forced failure reports `failed` and the rollback behavior from the command-line updater still restores the previous binary.
8. Confirm a desktop run without `-update-dir` can check releases but cannot request a privileged install.

## 11. GitHub CI

After pushing, confirm the included workflow passes formatting checks, `go vet`, unit tests, and Linux AMD64/ARM64 builds.

## Browser settings

Verify Theme and Default Region persistence through the Settings panel and confirm clearing localStorage restores YAML fallbacks.
