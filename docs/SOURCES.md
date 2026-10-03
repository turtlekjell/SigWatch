# Bundled source inventory

This document records the primary upstream sources used by the checked-in SigWatch configuration. User-created regions may use other sources and are responsible for their own terms and attribution requirements.

SigWatch is an aggregation/display tool. Upstream availability and update cadence are controlled by the source providers.

## NOAA / National Weather Service

Used for:

- NWS radar loops
- U.S. weather warnings image
- National Weather Prediction Center forecast image
- NWS structured seven-day forecast used by the coastal widget

The checked-in configuration keeps NOAA/NWS attribution visible on the relevant image widgets.

## NOAA / NESDIS / STAR

Used for GOES satellite imagery.

The National / Radio region uses GOES-East / GOES-19 CONUS GeoColor imagery. Regional satellite tiles remain city/region specific. Lightning imagery is handled separately through Blitzortung fixed maps rather than the GOES GLM overlays. Satellite availability may be interrupted by upstream maintenance; SigWatch's last-known-good cache/freshness handling is intended to make those interruptions visible without breaking the dashboard.

## Blitzortung.org

Used for the bundled lightning tiles. Blitzortung provides a national North America/USA fixed map plus regional fixed maps. SigWatch uses:

- National / Radio — `image_b_us.png`
- Northwest / Seattle — `image_b_us.png` (no dedicated Pacific Northwest fixed map is offered in the standard regional menu)
- Southwest / Huntington Beach — `image_b_ca.png`
- Midwest / Kansas City — `image_b_mn.png`
- Southeast / Miami — `image_b_fl.png`
- Northeast / New York City — `image_b_ny.png`

The fixed maps show recent lightning activity and identify Blitzortung.org contributors on the rendered image. Blitzortung states that images marked CC BY-SA may be included on other websites under that license. SigWatch therefore keeps Blitzortung contributor / CC BY-SA attribution visible in each widget. Blitzortung is a community project and is not an official warning service.

## NOAA Tides & Currents

Used by the coastal widget for high/low tide predictions. Bundled coastal presets use Seattle `9447130`, Newport Beach `9410580`, Virginia Key `8723214`, and The Battery `8518750`. A different coastal region should select an appropriate NOAA tide station and timezone.

## USGS

Used by the earthquake widget. SigWatch filters the USGS real-time feed locally by configured center point, radius, minimum magnitude, time window, and event count.

## NIFC / WFIGS / IRWIN

Used by the wildfire widget. The provider is national rather than California-specific. Incident completeness depends on what participating agencies report into the interagency system.

## HAMQSL.com / N0NBH

Used by the National / Radio region for:

- solar/day-night map (`solarmap.php`)
- HF/VHF conditions (`solar101vhf.php`)
- current solar image (`solarsun.php`)
- solar trend graph (`solargraph.php`)

HAMQSL asks users of its displayed data/panels to preserve source credit. SigWatch therefore identifies HAMQSL/N0NBH in the widget attribution. HAMQSL documents different update intervals for different measurements; the checked-in configuration intentionally refreshes these panels more slowly than ordinary radar/lightning imagery.

## YouTube camera streams

Each bundled geographic preset contains two YouTube camera examples. The Huntington Beach pair was validated during development; the Seattle, Kansas City, Miami, and New York replacements were refreshed during the v1 release audit from sources that were actively embedding or reporting the streams online.

Current examples include Seattle Space Needle and Waterfront views, Kansas City downtown and Zoo polar-bear views, PortMiami and Coral City, plus New York Times Square North and a rotating city-camera feed.

YouTube content is delivered directly to the browser; SigWatch does not proxy or persist the video stream. Stream IDs and embed permissions are controlled by the channel owner/provider and may change over time, so users should replace stale or unavailable cameras in `config.yaml`.

## Source maintenance

Before a stable release, verify that bundled image URLs still return current content. A broken or retired image source should be replaced in `config.yaml` rather than worked around by spoofing or bypassing provider restrictions.
