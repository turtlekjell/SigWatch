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
- Local, National / Radio, and Prototype regions all switch cleanly.
- Region switching works and persists after browser reload.
- Local and National / Radio viewport regions fit their 4 x 3 grids without page scrolling at the intended landscape resolution.
- Image widgets render and eligible images can be enlarged/closed.
- Weather, earthquake, fire, coastal, camera, clock, links, and system widgets used by the checked-in configuration render without breaking unrelated widgets.
- `/healthz` returns an OK response.

## 3. Configuration validation

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

## 4. Refresh isolation

Confirm externally refreshed widgets update independently and that the page does not routinely reload.

Use the region-level Refresh control and verify supported external widgets update without restarting camera embeds or breaking unrelated widgets.

## 5. Persistent cache and offline behavior

1. Start SigWatch online and let cacheable widgets load successfully.
2. Stop SigWatch.
3. Disconnect Internet access.
4. Restart SigWatch using the same cache directory.
5. Confirm last-known-good cacheable content is restored.
6. Confirm freshness indicators age into stale/expired states according to policy.
7. Confirm one failed provider does not prevent unrelated widgets or the server from running.

For a fast freshness test, temporarily use short thresholds, then restore normal values afterward.

## 6. Structured-provider checks

### Earthquake

Confirm center/radius, minimum magnitude, time window, event count, and event links behave as configured.

### Fire

Confirm radius, `named_only`, `min_acres`, event count, containment display, incident links, and empty/no-current-incident behavior.

### Coastal

Confirm tide predictions and seven-day forecast both render. Test or simulate a failure of one source and verify the other half can remain current/available independently.

### Camera

Confirm supported YouTube URL forms render in the browser and autoplay/mute/control settings behave as expected for the browser environment.

### National / Radio

Confirm the current national NOAA imagery loads, the HAMQSL panels are legible, UTC/system/link widgets render, and no retired Maps-only layout behavior is required. A single unavailable image source must not break the other eleven tiles.

## 7. Loopback security check

From the SigWatch host:

```bash
curl http://127.0.0.1:8080/healthz
```

From another LAN host, direct access to port 8080 should fail under the default loopback-only configuration.

## 8. Cross-build

```bash
make cross
file dist/sigwatch-linux-arm64 dist/sigwatch-linux-amd64
```

## 9. Raspberry Pi OS Bookworm

On the Pi:

1. Build SigWatch from a clean clone.
2. Validate `config.yaml`.
3. Install with `sudo ./scripts/install-pi.sh ./sigwatch ./config.yaml`.
4. Confirm `systemctl status sigwatch --no-pager` reports the service active.
5. Reboot and confirm the SigWatch backend starts automatically.
6. Launch the dashboard manually with `chromium --kiosk http://127.0.0.1:8080/`.
7. Confirm the Local viewport fits the display and both video streams remain usable.
8. Confirm persistent cache survives a service/device restart.
9. Reboot without Internet and confirm SigWatch starts rather than crashing.

Automatic Chromium launch is optional for the v1 baseline and can be documented separately after a preferred Bookworm desktop-session method is settled.

## 10. GitHub CI

After pushing, confirm the included workflow passes formatting checks, `go vet`, unit tests, and Linux AMD64/ARM64 builds.
