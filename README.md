# SigWatch

SigWatch is a lightweight, configurable situational-awareness dashboard for Raspberry Pi kiosk displays and ordinary desktop use. It combines local and regional weather, maps, live cameras, earthquakes, wildfire information, tides, forecasts, clocks, links, and system status in one browser dashboard.

SigWatch uses a Go backend with server-embedded HTML/CSS/vanilla JavaScript, human-readable YAML configuration, loopback-only HTTP by default, independent widget refresh, persistent last-known-good caching, visible stale/expired states, and named regions.

## Current features

- YAML configuration with validation and environment-variable expansion.
- Loopback-only listener by default (`127.0.0.1:8080`).
- Named regions with sticky browser selection.
- Flexible CSS-grid layouts plus viewport-locked regions that fit a configured grid into one screen.
- `clock`, `links`, `image`, `weather`, `system`, `earthquake`, `fire`, `camera`, and `coastal` widgets.
- YouTube camera embeds with configurable autoplay, mute, and controls.
- USGS earthquake filtering by center point, radius, magnitude, age, and event count.
- NIFC/WFIGS wildfire filtering by center point, radius, acreage, naming, and event count.
- Coastal widget combining NOAA tide predictions with an NWS seven-day forecast.
- Independent external-widget refresh; no routine full-page reload.
- In-memory and persistent last-known-good caching for supported external widgets.
- Fresh/stale/expired states with configurable thresholds.
- Region-level manual refresh.
- Click-to-enlarge support for eligible image content.
- Four bundled themes: `default`, `light`, `midnight`, and `amber`.
- Header Settings panel for browser-local Theme and Default Region preferences plus stable-release update checking/install on supported Pi/systemd installs.
- systemd service and Raspberry Pi install helper.
- Linux ARM64/AMD64 and developer cross-build targets.
- Unit tests and GitHub CI.

## Quick start

Requires Go 1.23+.

```bash
git clone https://github.com/TurtleKjell/SigWatch.git
cd SigWatch
go run ./cmd/sigwatch -config ./config.yaml -check
go run ./cmd/sigwatch -config ./config.yaml
```

Open:

```text
http://127.0.0.1:8080/
```

Run the full local verification set with:

```bash
make verify
```

That runs unit tests, `go vet`, and configuration validation.

## Configuration

The primary configuration is `config.yaml`. `examples/config.yaml` is a smaller starter example.

A region can use the normal flowing grid or a viewport-locked grid. For example:

```yaml
local:
  label: "Local"
  layout: viewport
  columns: 4
  rows: 3
  widgets:
    # widgets here
```

With `layout: viewport`, SigWatch constrains the configured rows to the available browser viewport so a kiosk dashboard can remain on one page without scrolling.

Widget `width` and `height` values are grid spans.

For portability guidance and examples, see [`docs/REGIONS.md`](docs/REGIONS.md).

The checked-in configuration ships with seven regions:

- **Northwest - Seattle** — Pacific Northwest weather, coastal data, hazards, and two live camera examples.
- **Southwest - Huntington Beach** — the original 4 x 3 local dashboard, including its two validated live camera tiles.
- **Midwest - Kansas City** — Central Plains weather, hazards, and two local live camera examples.
- **Southeast - Miami** — Southeast weather, coastal/tide data, hazards, and two Miami live camera examples.
- **Northeast - New York City** — Northeast weather, coastal/tide data, hazards, and two New York live camera examples.
- **National / Radio** — national weather imagery plus solar/HAM/propagation information.
- **Prototype** — a flexible sandbox for developing and testing widgets.

The five geographic presets are examples, not hard-coded application behavior. Each can be duplicated and retargeted by editing coordinates, radar/GOES/NDFD sources, timezone, and (where applicable) NOAA tide station.

### Themes

SigWatch ships with four appearance-only themes:

- `default` — the original dark SigWatch theme.
- `light` — a bright neutral theme for daytime/office displays.
- `midnight` — a deeper navy/black dark theme for TVs and low-light rooms.
- `amber` — a warm terminal/radio-console inspired dark theme.

For the easiest per-display setup, use **⚙ Settings** in the dashboard header. Theme changes apply immediately and both Theme and Default Region are remembered in that browser using local storage. This does not rewrite `config.yaml`; YAML remains the system-wide fallback for new browser profiles or cleared browser data.

You can still select the system-wide fallback theme in YAML:

```yaml
theme: "midnight"
```

Themes change colors, borders, and shadows only; region layout, widget behavior, freshness states, and providers are unchanged. See [`docs/THEMES.md`](docs/THEMES.md).

Every tile in those presets is also just an example. A user can replace an earthquake tile with a different camera, alert, map, or future provider without changing the region system. Earthquakes are naturally more prominent in some parts of the country than others; the bundled East Coast/Southeast earthquake tiles are retained mainly to demonstrate that the USGS widget is geographically portable. Future hazard-focused presets such as hurricane/tropical-weather and tornado/severe-weather dashboards are good post-v1 candidates rather than v1 requirements.

Each geographic preset includes two YouTube camera examples. For the non-Huntington Beach presets, the bundled choices were selected from streams that were actively embedded/online during the v1 release audit. Public livestream IDs can still rotate or disappear over time, so the camera URLs are intentionally easy to replace in `config.yaml`.

## Widget overview

### Image

Displays an externally retrieved image or animated image. The Go backend fetches and caches the source.

Typical uses include radar, satellite, precipitation, warnings, propagation maps, and lightning imagery.

### Weather

Structured local weather using latitude/longitude and `imperial` or `metric` units.

### Earthquake

Uses USGS earthquake data and can filter by:

- latitude / longitude
- radius
- minimum magnitude
- age window
- maximum displayed events

### Fire

Uses current NIFC/WFIGS wildfire incident data and can filter by:

- latitude / longitude
- radius
- minimum acreage
- named incidents only
- maximum displayed incidents

The provider is not California-specific; a different region normally requires only different coordinates, radius, and display preferences.

### Camera

Supports common YouTube video/live URL formats. Video is delivered directly from YouTube to the browser rather than proxied through SigWatch.

### Coastal

Combines two independent sources:

- NOAA tide predictions for a configured tide station
- National Weather Service seven-day forecast for configured coordinates

The tide and weather halves retain independent last-known-good data so a temporary failure of one source does not erase the other.

### Clock, links, and system

Local/non-network utility widgets for time zones, shortcuts, and basic host/process status.

## Refresh, cache, and freshness

Each external widget refreshes independently.

Freshness is measured from the last successful source update, not the most recent attempt.

- **Fresh**: before `warning_after`.
- **Stale**: last-known-good content remains visible with a textual/visual stale indication.
- **Expired**: old primary content is no longer presented as current; SigWatch shows an unavailable state and the last successful update time.
- **Never loaded**: unavailable until the first successful fetch.

Supported external widgets keep last-known-good content in memory and on disk when a cache directory is configured. The Raspberry Pi systemd service uses `/var/lib/sigwatch/cache`.

## Raspberry Pi OS Bookworm

SigWatch has been exercised on Raspberry Pi OS Bookworm as a local systemd service. The backend can start automatically at boot even if you choose to launch Chromium manually.

On the Pi, clone the repository and build:

```bash
git clone https://github.com/TurtleKjell/SigWatch.git
cd SigWatch
make build
./sigwatch -config ./config.yaml -check
```

Install and enable the service:

```bash
sudo ./scripts/install-pi.sh ./sigwatch ./config.yaml
```

The installer places the application under `/opt/sigwatch`, the configuration under `/etc/sigwatch`, the persistent cache under `/var/lib/sigwatch`, installs `/usr/local/sbin/sigwatch-update`, and enables `sigwatch.service` so the backend starts after reboot.

Check it with:

```bash
systemctl status sigwatch --no-pager
curl http://127.0.0.1:8080/healthz
```

To open the dashboard manually in Chromium kiosk mode:

```bash
chromium --kiosk http://127.0.0.1:8080/
```

Browser autostart is intentionally left as an optional deployment choice because Raspberry Pi desktop/session behavior can vary. A prototype desktop entry remains under `deploy/kiosk/` for users who want to adapt it.

### Updating an installed Pi

On an installation made with `scripts/install-pi.sh`, open **⚙ Settings → Software Update** and choose **Check for Updates**. When a newer stable `vX.Y.Z` release exists, **Install Update** requests the same rollback-capable updater used by the command line. The browser reconnects after `sigwatch.service` restarts.

The command-line equivalents remain available:

```bash
sigwatch-update --check
sudo sigwatch-update
```

The web UI cannot provide arbitrary commands, repository URLs, or version strings. It can only request the latest stable release. A root-owned systemd path/service performs the privileged install while the SigWatch web process remains unprivileged. The updater validates the existing `/etc/sigwatch/config.yaml`, backs up the current binary, restarts the service, checks `/healthz`, and rolls back automatically if the new service does not become healthy. It does **not** overwrite the user's configuration. See [`docs/UPDATING.md`](docs/UPDATING.md).

## Build targets

Build locally:

```bash
make build
```

Cross-build Linux ARM64 and AMD64:

```bash
make cross
```

Developer builds for Apple Silicon macOS and Windows AMD64:

```bash
make dev-cross
```

## Security posture

SigWatch rejects non-loopback listen addresses in the current baseline. Remote/LAN administration is intentionally outside the v1 design.

External sources are treated as untrusted input and fetched with timeouts/size limits where applicable. Camera embeds are restricted to supported YouTube URL forms rather than arbitrary iframe URLs.

Example configuration must not contain private credentials or secrets.

## Data sources and attribution

The checked-in configuration demonstrates several public sources, including NOAA/NWS, NOAA/NESDIS/STAR, USGS, NIFC/WFIGS, NOAA Tides & Currents, HAMQSL/N0NBH, and selected third-party video/propagation providers. Each integration has its own terms, attribution requirements, rate limits, and availability characteristics.

See [`docs/SOURCES.md`](docs/SOURCES.md) for the bundled-source inventory and refresh notes.

SigWatch does not replace authoritative emergency channels and does not guarantee that every upstream incident or warning is complete or current.

## License

SigWatch is released under the MIT License. See [`LICENSE`](LICENSE).

## Project status

The current 0.1.x series is the release-candidate phase for the first stable v1.0 release. The core application and Local dashboard are usable; remaining work is focused on repository cleanup, source/configuration cleanup, documentation, and final release verification rather than major new features.
