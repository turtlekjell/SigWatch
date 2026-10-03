# Themes

SigWatch themes are intentionally presentation-only. They do not change providers, refresh behavior, region layout, widget sizing, caching, stale/expired semantics, or application logic.

## Bundled themes

### `default`

The original SigWatch dark theme. This remains the default so existing installations keep their current appearance after upgrading.

### `light`

A light neutral theme intended for bright rooms, offices, and displays where a dark dashboard is less desirable.

### `midnight`

A deeper navy/black theme with higher visual contrast for televisions, OLED displays, and low-light rooms.

### `amber`

A dark warm-toned theme inspired by classic terminals and radio consoles. It remains a general-purpose theme rather than a novelty skin.

## Selecting a theme

The easiest way to choose a theme for a particular display is the **⚙ Settings** button in the dashboard header. The selected theme is stored in that browser's `localStorage` and applies immediately without restarting SigWatch.

The root YAML setting remains the system-wide fallback:

```yaml
theme: "light"
```

A browser-local choice overrides that fallback only for that browser profile. Clearing browser storage restores the YAML-configured theme. If the YAML-configured theme is not installed, SigWatch still rejects startup with an actionable error rather than silently falling back.

The Settings panel also exposes **Default Region** as a browser-local preference. Saving it switches the current display to that region and remembers it for future starts; normal region switching remains sticky as before.

## Theme contract

A theme is a directory under `internal/app/web/themes/<name>/` containing `theme.css`. The shared layout and widget CSS remain in `internal/app/web/static/base.css`.

Bundled themes define the same CSS custom properties:

```text
--bg
--panel
--surface
--text
--muted
--accent
--border
--warning
--danger
--radius
--shadow
```

New themes should preserve readable contrast and must keep stale/error states visibly distinguishable without relying on color alone.

A future Unicorn theme can use the same contract without changing Go provider logic or widget implementations.
