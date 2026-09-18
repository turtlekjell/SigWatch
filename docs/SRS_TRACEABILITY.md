# SRS v0.2 Prototype Traceability

Status meanings: **Implemented** = present in source and testable now; **Partial** = scaffolding or core behavior exists but target-hardware/provider verification remains; **Deferred/TBD** = intentionally not claimed complete.

| Area | Prototype status | Notes |
|---|---|---|
| FR-001/002 supervised service and restart | Partial | systemd unit included; Raspberry Pi execution still needs hardware validation. |
| FR-003/004 kiosk launch | Partial | Chromium autostart desktop entry included; desktop-session-specific Pi validation remains. |
| FR-005 localhost binding | Implemented | Config validation rejects non-loopback listeners. |
| FR-006 alternate config path | Implemented | `-config`. |
| FR-007 validation mode | Implemented | `-check`. |
| FR-010–014 responsive configurable layout/header | Implemented | CSS Grid, width/height spans, status header. |
| FR-015 enlarged widget view | Deferred | SHOULD, not yet implemented. |
| FR-016 input modes | Partial | Native browser mouse/touch/keyboard behavior; dedicated enlarged-view controls not present. |
| FR-020–026 YAML configuration | Implemented | Self-contained YAML-subset parser, strict key validation, environment-variable expansion. GUI editor remains deferred as specified. |
| FR-030/031 widget IDs/config | Implemented | Stable type and unique per-region ID validation. |
| FR-032 image widget | Implemented | Server-side retrieval, image validation, 20 MiB bound, browser-supported rendering. |
| FR-033 clock widget | Implemented | IANA timezone validation and browser clock rendering. |
| FR-034 links widget | Implemented | HTTP(S) links. |
| FR-035 weather widget | Implemented prototype | Open-Meteo current-conditions provider. |
| FR-036 system status | Implemented prototype | Host/process metrics; Pi-specific health metrics can be expanded. |
| FR-037 modular widget framework | Partial | Provider interface + type-separated frontend rendering; more providers will exercise extensibility. |
| FR-040–044 independent refresh/failure isolation | Implemented core | Per-widget Go refresh goroutines, bounded HTTP timeout; synchronized-burst jitter not yet implemented. |
| FR-045–052 freshness/cache | Implemented runtime | Last-known-good retained in memory; stale/expired transitions and per-widget override supported. Persistence across process reboot is not yet implemented. |
| FR-060–064 browser communication/security | Implemented | Local HTTP, per-widget API polling, no framework, no arbitrary filesystem route. |
| FR-070–075 themes | Implemented core | Default theme contract is separate; missing themes fail startup. Second real theme not yet bundled. |
| FR-080–084 regions | Implemented | Named regions, selector, localStorage persistence, fallback to default. |
| UI-001–007 glanceability/status | Implemented core | Responsive kiosk-oriented layout and textual + visual freshness states; target display validation remains. |
| PI-001 | TBD | Pi model/OS requires prototype hardware testing. |
| PI-002–006 | Partial | Service/kiosk artifacts present; hardware reboot/power restoration test remains. |
| PI-007 | Partial | No database; refresh state is memory-resident. Disk-write measurement remains. |
| SEC-001/002 | Implemented | Loopback enforced. |
| SEC-004–008 | Implemented core | No shell execution config, server-side source retrieval, no telemetry, no example secrets. |
| LIC-001/003–007 | Partial | Clean implementation; user-configured sources remain subject to their terms. Third-party dependency count is zero. |
| LIC-002 project license | TBD | SRS explicitly leaves license selection open before public release. |
| NFR-001–012 | Implemented/Partial | Core separation, no Node/database, tests and journald-friendly stdout logging are present; long-duration/Pi performance requires real-world testing. |
