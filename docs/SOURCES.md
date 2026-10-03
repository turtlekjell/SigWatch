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

The National / Radio region uses GOES-East / GOES-19 CONUS GeoColor imagery. The five city presets use matched regional GeoColor and GLM lightning-over-GeoColor sectors: GOES-18 Pacific Northwest and Pacific Southwest, plus GOES-19 Upper Mississippi Valley, Southeast, and Northeast. Satellite image availability may be interrupted by upstream maintenance; SigWatch's last-known-good cache/freshness handling is intended to make those interruptions visible without breaking the dashboard.

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

## Blitzortung / LightningMaps

Used by the National / Radio region for the U.S. lightning overview. The five city presets use NOAA GOES GLM sector imagery for their local lightning tiles. Blitzortung/LightningMaps is a third-party community source rather than an official government warning service.

## YouTube camera streams

The Southwest - Huntington Beach region demonstrates supported YouTube live/video embeds. YouTube content is delivered directly to the browser; SigWatch does not proxy or persist the video stream. Stream IDs and availability are controlled by the channel owner/provider and may change over time. Other bundled city presets avoid hard-coding webcam IDs so the default configuration remains more durable.

## Source maintenance

Before a stable release, verify that bundled image URLs still return current content. A broken or retired image source should be replaced in `config.yaml` rather than worked around by spoofing or bypassing provider restrictions.
