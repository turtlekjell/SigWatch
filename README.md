#SigWatch

SigWatch is a lightweight, configurable local/regional situational-awareness dashboard designed for Raspberry Pi kiosk deployments and ordinary desktop development.

This repository is the first implementation prototype based on SigWatch SRS v0.2 (2026-09-18). It uses a Go backend, server-embedded HTML/CSS/vanilla JavaScript, YAML configuration, loopback-only HTTP by default, independent widget refresh, last-known-good in-memory caching, and visible stale/expired states.

Prototype features

YAML config with strict unknown-field validation and environment-variable expansion.

Zero runtime/library dependencies beyond the Go standard library; the built-in config reader supports the documented MVP YAML subset (mappings, lists, scalars, comments) and intentionally rejects advanced YAML features.

Loopback-only listener validation.

Named regions with sticky browser selection.

CSS-grid widget layout with variable width/height.

Clock, links, image, weather, and basic local system widgets.

Independent external-widget refreshers in Go; no routine full-page reload.

Last-known-good data retained after update failure.

Default stale at 15 minutes / expired at 1 hour, globally configurable and per-widget overridable.

Expired widgets stop presenting old primary content.

Default theme separated from widget/provider logic.

Content Security Policy and server-side external source retrieval.

systemd unit, Chromium kiosk autostart example, Pi install helper.

Unit tests and Linux ARM64/AMD64 cross-build targets.

Run locally

Requires Go 1.23+.

go mod download
go run ./cmd/sigwatch -config ./config.yaml

Open http://127.0.0.1:8080/.

Validate config without starting the service:

go run ./cmd/sigwatch -config ./config.yaml -check

Run tests:

go test ./...

Configuration

Start from examples/config.yaml. Supported MVP/prototype widgets:

clock: requires an IANA timezone such as America/Los_Angeles or UTC.

links: contains link label/URL pairs.

image: requires an HTTP(S) url; the Go server retrieves the image so source URLs are not exposed to the browser.

weather: requires latitude, longitude, and optional units: imperial|metric; prototype provider is Open-Meteo.

system: basic SigWatch process/host metrics.

width and height are grid spans. External widgets accept refresh and optional per-widget freshness thresholds.

Freshness model

Freshness is measured from the last successful source update, not the last attempt.

Fresh: before warning_after.

Stale: retain last-known-good content, red border + textual STALE indicator.

Expired: after expire_after, hide old primary content and show unavailable plus last-success timestamp.

Never successfully loaded: unavailable.

Raspberry Pi deployment

The service unit is in deploy/systemd/sigwatch.service. A basic Chromium autostart desktop entry is in deploy/kiosk/sigwatch-kiosk.desktop.

Cross-build for Raspberry Pi/Linux:

make cross

Convenience developer builds for Apple Silicon macOS and Windows AMD64:

make dev-cross

On a Raspberry Pi, copy the ARM64 binary and repository deployment files, then:

sudo ./scripts/install-pi.sh ./dist/sigwatch-linux-arm64 ./examples/config.yaml

The exact supported Pi model/OS and desktop-session-specific kiosk installation remain prototype-test decisions, matching the SRS TBDs.

Security posture

The prototype rejects non-loopback listen addresses. Remote/LAN access is intentionally not part of the MVP. External sources are fetched server-side with response-size limits and timeouts. Example configuration contains no credentials.

Data sources and licensing

The optional prototype weather widget uses Open-Meteo. Other image/data sources are user-configured and must be reviewed for their own usage terms, attribution requirements, rate limits, and redistribution/content rights. The sample links point to authoritative public sites but are not endorsements.

Project license is intentionally not selected yet because the SRS marks the license as TBD before public release. Choose a license before making the GitHub repository public.

Known prototype gaps

No persistent last-known-good cache across process/device reboots yet.

No earthquake, wildfire, public-alert, radar-provider, or radio-specific provider yet; the SRS leaves the first public provider set to implementation selection.

System widget is intentionally basic and does not yet read Raspberry Pi temperature, load average, disk wear, or undervoltage flags.

Pi kiosk setup varies by Raspberry Pi OS release/desktop; the provided desktop entry is scaffolding rather than a fully tested OS image recipe.

No graphical configuration editor or hot reload (both post-MVP/deferred in the SRS).

Suggested next test loop

Run on macOS/Windows/Linux desktop using config.yaml.

Exercise config validation by intentionally breaking a field.

Test multi-region selection and reload persistence.

Add one permitted remote image source and test refresh/failure/stale behavior with short temporary thresholds (for example 30s/1m).

Cross-build ARM64 and deploy to the target Raspberry Pi.

Measure CPU/memory and settle the minimum supported Pi and baseline display resolution.

Included prototype sources

The checked-in prototype configuration includes the official NOAA/National Weather Service KSOX animated radar GIF as the image-widget example and labels its source in the UI. The image URL is still configuration, not hard-coded provider logic, so it can be replaced with another permitted source.
