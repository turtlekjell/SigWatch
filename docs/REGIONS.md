# Regions and portability

SigWatch regions are configuration profiles. The application does not hard-code a fixed list of cities, states, or geographic areas.

A region can mix structured providers, remote images, cameras, clocks, links, and system information. Most geographic portability work therefore belongs in `config.yaml`, not Go source code.

## Viewport regions

For kiosk dashboards intended to fit on one screen, use:

```yaml
local:
  label: "Local"
  layout: viewport
  columns: 4
  rows: 3
  widgets:
    # 12 one-cell widgets, or another combination totaling the grid
```

`width` and `height` remain grid spans. A `4 x 3` viewport region is useful for a twelve-tile dashboard.

## Providers that are already geographically portable

### Weather

Change `latitude` and `longitude`.

### Earthquake

Change the center point, radius, magnitude threshold, age window, and event count as desired.

Example:

```yaml
- id: "regional-earthquakes"
  type: "earthquake"
  title: "Regional Earthquakes"
  latitude: 40.7128
  longitude: -74.0060
  max_radius_km: 300
  min_magnitude: 2.5
  hours: 24
  max_events: 4
  refresh: "2m"
  width: 1
  height: 1
```

### Fire

The NIFC/WFIGS provider is national rather than California-specific. For an eastern U.S. region, change the center point and radius. You may also want a smaller `min_acres` value than a western-region configuration.

Example:

```yaml
- id: "regional-fires"
  type: "fire"
  title: "Regional Wildfires"
  latitude: 40.7128
  longitude: -74.0060
  max_radius_km: 300
  max_events: 4
  named_only: true
  min_acres: 1
  refresh: "5m"
  width: 1
  height: 1
```

WFIGS/IRWIN coverage depends on what participating agencies report, so SigWatch should not be treated as a complete list of every small local fire.

### Coastal

Change:

- forecast latitude / longitude
- IANA timezone
- NOAA tide station ID

The widget code itself does not depend on Huntington Beach.

### Camera

Replace the YouTube URL with another supported YouTube video/live URL.

## Sources that normally need regional replacement

Image widgets are intentionally generic. A region using a California radar, GOES sector, precipitation map, or lightning image will need different URLs when moved to another part of the country.

Examples include:

- NWS radar site loops
- GOES-East vs GOES-West sectors
- regional precipitation products
- regional lightning images
- state/region-specific maps

These are configuration changes rather than new SigWatch provider code.


## Bundled National / Radio region

The v1 configuration includes a `national` viewport region using a 4 x 3 grid. It combines:

1. CONUS radar
2. National forecast
3. U.S. weather warnings
4. GOES-East CONUS satellite
5. U.S. lightning
6. HAMQSL solar/day-night map
7. HAMQSL HF/VHF conditions
8. Current solar image
9. HAMQSL solar trend
10. UTC clock
11. SigWatch host status
12. Radio/space-weather links

The National / Radio region is deliberately not tied to the Local region's coordinates. It provides a useful national overview while demonstrating that weather imagery, radio information, local utility widgets, and links can coexist in the same viewport layout.

## Bundled U.S. regional presets

SigWatch v1 includes five city-centered U.S. examples using the same provider framework:

| Region | Anchor city | Radar | GOES sector | NDFD sector | Coastal station |
| --- | --- | --- | --- | --- | --- |
| Northwest | Seattle, WA | KATX | GOES-18 Pacific Northwest (`pnw`) | Pacific Northwest | Seattle `9447130` |
| Southwest | Huntington Beach, CA | KSOX | GOES-18 Pacific Southwest (`psw`) | Pacific Southwest | Newport Beach `9410580` |
| Midwest | Kansas City, MO | KEAX | GOES-19 Upper Mississippi Valley (`umv`) | Central Plains | N/A |
| Southeast | Miami, FL | KAMX | GOES-19 Southeast (`se`) | Southeast | Virginia Key `8723214` |
| Northeast | New York City, NY | KOKX | GOES-19 Northeast (`ne`) | Northeast | The Battery `8518750` |

Weather, earthquake, and fire widgets are centered on the listed city coordinates. The regional image widgets use the closest practical bundled radar, GOES, and NDFD sectors rather than pretending the city itself is the center of those upstream products.

The Southwest preset retains two Huntington Beach YouTube camera tiles because those streams were specifically validated during development. The other city presets intentionally use durable clock/source-link utility tiles rather than shipping webcam IDs that may disappear or rotate without notice.

The Midwest preset is inland, so its eighth tile uses SigWatch host status instead of the coastal/tides widget.

## Recommended bundled regions for v1

The repository now ships seven regions:

1. **Northwest - Seattle**
2. **Southwest - Huntington Beach**
3. **Midwest - Kansas City**
4. **Southeast - Miami**
5. **Northeast - New York City**
6. **National / Radio**
7. **Prototype**

The five city presets are meant to make a fresh install immediately useful while still demonstrating that regions are configuration rather than application code. Users can delete presets they do not need or copy one as the starting point for another city.
