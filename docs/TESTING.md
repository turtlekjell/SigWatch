# SigWatch Prototype Test Plan

This plan turns the SRS v0.2 acceptance evidence into a practical prototype test loop.

## 1. Desktop smoke test

```bash
go test ./...
go run ./cmd/sigwatch -config ./config.yaml
```

Open `http://127.0.0.1:8080/`. Confirm:

- Dark default theme loads.
- Weather, local clock, UTC clock, host status, and source links render.
- The page does not routinely reload.
- `/healthz` returns JSON with `status: ok`.

## 2. Configuration validation

```bash
go run ./cmd/sigwatch -config ./config.yaml -check
```

Then copy the config, introduce each of these defects one at a time, and verify startup fails with an actionable field path:

- Unknown key.
- Non-loopback `listen` address.
- Unknown clock timezone.
- Duplicate widget ID.
- Weather widget missing latitude/longitude.
- `warning_after >= expire_after`.

## 3. Region persistence

Add a second region to `config.yaml`, restart SigWatch, choose it in the selector, then reload the browser. The selected region should persist through browser local storage. Remove that region from the YAML and restart; the UI should fall back to `default_region`.

## 4. Independent refresh

Configure two external widgets with visibly different refresh intervals, such as system at `10s` and weather at `1m`. Watch the widget timestamps/statuses. They should update independently without a full-page reload.

## 5. Failure / stale / expired behavior

For a fast manual test, temporarily set:

```yaml
freshness:
  warning_after: "30s"
  expire_after: "1m"
```

Use an image widget pointed at a small permitted HTTP(S) image source. Let it load successfully, then make the source fail (for example, change the upstream service or disconnect Internet without restarting SigWatch).

Expected:

1. Last-known-good content remains initially.
2. At 30 seconds since the last successful update, the widget shows a red border and textual `STALE` state.
3. At 1 minute, old primary content is hidden and the widget shows unavailable plus the last successful timestamp.
4. Other widgets continue operating.

Restore normal 15m/1h thresholds after the test.

## 6. Loopback security check

From the SigWatch host:

```bash
curl http://127.0.0.1:8080/healthz
```

From another LAN host, connection to the SigWatch machine on port 8080 should fail under the default configuration because the service is bound only to loopback.

## 7. Cross-build

```bash
make cross
file dist/sigwatch-linux-arm64 dist/sigwatch-linux-amd64
```

## 8. Raspberry Pi prototype

After choosing the test Pi and Raspberry Pi OS release:

1. Copy the ARM64 binary and repository deployment files.
2. Install the systemd service using `scripts/install-pi.sh`.
3. Install the Chromium kiosk autostart entry appropriate to the desktop session.
4. Cold reboot.
5. Confirm SigWatch service and Chromium return without manual navigation.
6. Kill the SigWatch process and confirm systemd restarts it.
7. Reboot with Internet disconnected and confirm the dashboard still starts with network widgets unavailable/cached rather than crashing.
8. Record CPU, memory, temperature, display resolution, and Chromium responsiveness for the Pi-model decision.

## 9. GitHub CI

After pushing to GitHub, the included workflow runs formatting, `go vet`, unit tests, and Linux AMD64/ARM64 builds on every push and pull request.
