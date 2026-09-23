'use strict';

const state = {config: null, region: null, timers: [], clockTimers: [], refreshing: false};
const $ = selector => document.querySelector(selector);
const externalTypes = new Set(['image', 'weather', 'earthquake', 'fire', 'system']);

function clearTimers() {
  for (const timer of state.timers) clearInterval(timer);
  for (const timer of state.clockTimers) clearInterval(timer);
  state.timers = [];
  state.clockTimers = [];
}

function esc(value) {
  return String(value ?? '').replace(/[&<>"']/g, char => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[char]));
}

async function init() {
  const response = await fetch('/api/dashboard', {cache: 'no-store'});
  if (!response.ok) throw new Error(`Dashboard configuration: HTTP ${response.status}`);
  state.config = await response.json();
  $('#app-title').textContent = state.config.title;

  const select = $('#region-select');
  select.innerHTML = '';
  for (const [id, region] of Object.entries(state.config.regions)) {
    const option = document.createElement('option');
    option.value = id;
    option.textContent = region.label;
    select.appendChild(option);
  }

  const saved = localStorage.getItem('sigwatch.region');
  state.region = saved && state.config.regions[saved] ? saved : state.config.default_region;
  select.value = state.region;
  select.addEventListener('change', () => {
    state.region = select.value;
    localStorage.setItem('sigwatch.region', state.region);
    render();
  });

  $('#refresh-region').addEventListener('click', refreshCurrentRegion);
  render();
}

function render() {
  clearTimers();
  const renderedRegion = state.region;
  const dashboard = $('#dashboard');
  dashboard.innerHTML = '';
  dashboard.classList.toggle('maps-grid', renderedRegion === 'maps');
  dashboard.dataset.region = renderedRegion;

  const region = state.config.regions[renderedRegion];
  for (const widget of region.widgets) {
    const element = document.createElement('section');
    element.className = 'widget';
    element.id = `widget-${widget.id}`;
    element.style.gridColumn = `span ${Math.min(widget.width || 1, 4)}`;
    element.style.gridRow = `span ${Math.min(widget.height || 1, 4)}`;
    element.innerHTML = `<div class="widget-header"><span class="widget-title">${esc(widget.title)}</span><span class="widget-status" data-status>Ready</span></div><div class="widget-body" data-body></div>`;
    dashboard.appendChild(element);

    if (widget.type === 'clock') {
      renderClock(element, widget);
    } else if (widget.type === 'links') {
      renderLinks(element, widget);
    } else if (widget.type === 'camera') {
      renderCamera(element, widget);
    } else {
      refreshWidget(element, widget, renderedRegion);
      const ms = Math.max(5000, widget.refresh_ms || 60000);
      state.timers.push(setInterval(() => refreshWidget(element, widget, renderedRegion), ms));
    }
  }
  updateOverall();
}

async function refreshCurrentRegion() {
  if (state.refreshing) return;
  const regionID = state.region;
  const button = $('#refresh-region');
  state.refreshing = true;
  button.disabled = true;
  button.textContent = '↻ Refreshing…';

  try {
    const response = await fetch(`/api/region/${encodeURIComponent(regionID)}/refresh`, {
      method: 'POST',
      cache: 'no-store',
      headers: {'X-SigWatch-Action': 'refresh-region'}
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);

    // Pull the newly refreshed state into the browser. If the user switched
    // regions while the request was running, leave the new region alone.
    if (state.region === regionID) {
      const region = state.config.regions[regionID];
      await Promise.all(region.widgets.filter(w => externalTypes.has(w.type)).map(widget => {
        const element = document.getElementById(`widget-${widget.id}`);
        return element ? refreshWidget(element, widget, regionID) : Promise.resolve();
      }));
    }
  } catch (error) {
    $('#overall-health').textContent = `Refresh failed: ${error.message}`;
  } finally {
    state.refreshing = false;
    button.disabled = false;
    button.textContent = '↻ Refresh';
  }
}

function renderClock(element, widget) {
  const body = element.querySelector('[data-body]');
  body.innerHTML = '<div class="clock"><div class="clock-time"></div><div class="clock-zone"></div></div>';
  const tick = () => {
    try {
      const now = new Date();
      body.querySelector('.clock-time').textContent = new Intl.DateTimeFormat([], {
        timeZone: widget.timezone, hour: '2-digit', minute: '2-digit', second: '2-digit'
      }).format(now);
      body.querySelector('.clock-zone').textContent = widget.timezone;
      element.querySelector('[data-status]').textContent = 'Local clock';
    } catch (error) {
      body.innerHTML = `<div class="empty">${esc(error.message)}</div>`;
    }
  };
  tick();
  state.clockTimers.push(setInterval(tick, 1000));
}

function renderLinks(element, widget) {
  const body = element.querySelector('[data-body]');
  body.innerHTML = '<div class="links">' + (widget.links || []).map(link =>
    `<a href="${esc(link.url)}" target="_blank" rel="noopener noreferrer">${esc(link.label)}</a>`
  ).join('') + '</div>';
  element.querySelector('[data-status]').textContent = 'Quick links';
}

function renderCamera(element, widget) {
  const body = element.querySelector('[data-body]');
  body.classList.add('no-pad');
  if (!widget.embed_url) {
    body.innerHTML = '<div class="empty">Camera configuration unavailable</div>';
    element.querySelector('[data-status]').textContent = 'Camera unavailable';
    return;
  }
  const sourceLink = widget.source_url
    ? `<a class="camera-source" href="${esc(widget.source_url)}" target="_blank" rel="noopener noreferrer">Open source ↗</a>`
    : '';
  body.innerHTML = `<div class="camera-wrap"><iframe src="${esc(widget.embed_url)}" title="${esc(widget.title)}" loading="lazy" referrerpolicy="strict-origin-when-cross-origin" allow="autoplay; encrypted-media; picture-in-picture; fullscreen" allowfullscreen></iframe>${sourceLink}</div>`;
  element.querySelector('[data-status]').textContent = 'Live · YouTube';
}

async function refreshWidget(element, widget, regionID) {
  // A stale interval from a previous region may finish after the user switches.
  if (!document.body.contains(element) || state.region !== regionID) return;

  try {
    const response = await fetch(`/api/widget/${encodeURIComponent(regionID)}/${encodeURIComponent(widget.id)}`, {cache: 'no-store'});
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const result = await response.json();
    if (!document.body.contains(element) || state.region !== regionID) return;

    applyStatus(element, result);
    const body = element.querySelector('[data-body]');
    body.classList.toggle('no-pad', widget.type === 'image');

    if (result.status === 'expired' || result.status === 'unavailable') {
      body.innerHTML = `<div class="empty"><div><strong>No current data</strong><br>${result.last_success ? 'Last successful update: ' + new Date(result.last_success).toLocaleString() : 'No successful update yet'}${result.error ? '<br>' + esc(result.error) : ''}</div></div>`;
    } else if (widget.type === 'image') {
      body.innerHTML = `<div class="image-wrap enlargeable" tabindex="0" role="button" aria-label="Enlarge ${esc(widget.title)}"><img alt="${esc(widget.title)}" src="${esc(result.image_url)}"><div class="source-badge">Source: ${esc(result.source || 'Configured source')}</div></div>`;
      enableImageEnlarge(element, widget, result);
    } else if (widget.type === 'weather') {
      renderWeather(body, result.data);
    } else if (widget.type === 'earthquake') {
      renderEarthquakes(body, result.data, result.source);
    } else if (widget.type === 'fire') {
      renderFires(body, result.data, result.source);
    } else if (widget.type === 'system') {
      renderSystem(body, result.data);
    }
    updateOverall();
  } catch (error) {
    if (!document.body.contains(element) || state.region !== regionID) return;
    element.classList.remove('fresh', 'stale', 'expired');
    element.classList.add('unavailable');
    element.querySelector('[data-status]').textContent = 'Update error';
    element.querySelector('[data-body]').innerHTML = `<div class="empty">${esc(error.message)}</div>`;
    updateOverall();
  }
}

function ensureImageDialog() {
  let dialog = $('#image-dialog');
  if (dialog) return dialog;
  dialog = document.createElement('dialog');
  dialog.id = 'image-dialog';
  dialog.className = 'image-dialog';
  dialog.innerHTML = '<div class="image-dialog-shell"><div class="image-dialog-header"><strong data-dialog-title></strong><button type="button" data-dialog-close aria-label="Close enlarged image">×</button></div><div class="image-dialog-body"><img data-dialog-image alt=""></div><div class="image-dialog-footer" data-dialog-source></div></div>';
  document.body.appendChild(dialog);
  dialog.querySelector('[data-dialog-close]').addEventListener('click', () => dialog.close());
  dialog.addEventListener('click', event => { if (event.target === dialog) dialog.close(); });
  return dialog;
}

function openImageDialog(widget, result) {
  const dialog = ensureImageDialog();
  const image = dialog.querySelector('[data-dialog-image]');
  dialog.querySelector('[data-dialog-title]').textContent = widget.title;
  dialog.querySelector('[data-dialog-source]').textContent = `Source: ${result.source || 'Configured source'}`;
  image.src = result.image_url;
  image.alt = widget.title;
  if (dialog.open) dialog.close();
  dialog.showModal();
}

function enableImageEnlarge(element, widget, result) {
  const wrap = element.querySelector('.image-wrap');
  if (!wrap) return;
  const open = () => openImageDialog(widget, result);
  wrap.addEventListener('click', open);
  wrap.addEventListener('keydown', event => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      open();
    }
  });
}

function applyStatus(element, result) {
  element.classList.remove('fresh', 'stale', 'expired', 'unavailable');
  element.classList.add(result.status);
  let label = result.status === 'fresh' ? 'Fresh' : result.status === 'stale' ? 'STALE' : result.status === 'expired' ? 'EXPIRED' : 'Unavailable';
  if (result.last_success) label += ` · ${new Date(result.last_success).toLocaleTimeString()}`;
  element.querySelector('[data-status]').textContent = label;
}

function renderWeather(body, data) {
  if (!data) {
    body.innerHTML = '<div class="empty">No weather data</div>';
    return;
  }
  body.innerHTML = `<div class="weather-main"><div><div class="temperature">${Math.round(data.temperature)}${esc(data.temperature_unit)}</div><div class="weather-summary">${esc(data.summary)}</div></div><div>Feels ${Math.round(data.apparent_temperature)}${esc(data.temperature_unit)}</div></div><div class="metrics"><div class="metric"><small>Humidity</small>${Math.round(data.humidity)}%</div><div class="metric"><small>Wind</small>${Math.round(data.wind_speed)} ${esc(data.wind_speed_unit)}</div><div class="metric"><small>Direction</small>${Math.round(data.wind_direction)}°</div><div class="metric"><small>Precipitation</small>${data.precipitation} ${esc(data.precipitation_unit)}</div></div><div class="status-line">Source: Open-Meteo</div>`;
}

function renderEarthquakes(body, data, source) {
  if (!data || !Array.isArray(data.events)) {
    body.innerHTML = '<div class="empty">No earthquake data</div>';
    return;
  }
  if (data.events.length === 0) {
    body.innerHTML = `<div class="empty"><div>No M${Number(data.min_magnitude).toFixed(1)}+ earthquakes found in the last ${Number(data.window_hours)} hours within ${Math.round(Number(data.radius_km))} km.</div></div><div class="status-line">Source: ${esc(source || 'USGS')}</div>`;
    return;
  }

  const rows = data.events.map(event => {
    const magnitude = Number(event.magnitude);
    const depth = Number(event.depth_km);
    const time = new Date(event.time);
    const href = safeUSGSURL(event.url);
    const place = href
      ? `<a href="${esc(href)}" target="_blank" rel="noopener noreferrer">${esc(event.place || 'Unknown location')}</a>`
      : esc(event.place || 'Unknown location');
    return `<div class="quake-row"><div class="quake-mag">M${Number.isFinite(magnitude) ? magnitude.toFixed(1) : '?'}</div><div class="quake-detail"><strong>${place}</strong><span>${relativeAge(time)}${Number.isFinite(depth) ? ` · depth ${depth.toFixed(1)} km` : ''}</span></div></div>`;
  }).join('');

  body.innerHTML = `<div class="quake-list">${rows}</div><div class="status-line">${data.events.length} most recent · M${Number(data.min_magnitude).toFixed(1)}+ · last ${Number(data.window_hours)}h · Source: ${esc(source || 'USGS')}</div>`;
}

function renderFires(body, data, source) {
  if (!data || !Array.isArray(data.incidents)) {
    body.innerHTML = '<div class="empty">No wildfire data</div>';
    return;
  }

  const filters = [];
  if (data.named_only) filters.push('named/described only');
  const minAcres = Number(data.min_acres);
  if (Number.isFinite(minAcres) && minAcres > 0) filters.push(`${formatAcres(minAcres)}+ acres`);
  const filterText = filters.length ? ` · ${filters.join(' · ')}` : '';
  const mapURL = safeFireURL(data.official_map_url);
  const mapLink = mapURL ? ` · <a href="${esc(mapURL)}" target="_blank" rel="noopener noreferrer">Official map ↗</a>` : '';

  if (data.incidents.length === 0) {
    body.innerHTML = `<div class="empty"><div>No current wildfires matched within ${formatDistance(Number(data.radius_km))}${filters.length ? ` (${esc(filters.join(', '))})` : ''}.</div></div><div class="status-line">Source: ${esc(source || 'NIFC WFIGS')}${mapLink}</div>`;
    return;
  }

  const rows = data.incidents.map(fire => {
    const acres = Number(fire.acres);
    const contained = Number(fire.percent_contained);
    const distance = Number(fire.distance_km);
    const stateLabel = displayFireState(fire.state);
    const locationParts = [fire.city, fire.county ? `${fire.county} County` : '', stateLabel].filter(Boolean);
    const location = locationParts.length ? locationParts.join(' · ') : 'Location unavailable';
    const metrics = [];
    if (Number.isFinite(acres)) metrics.push(`${formatAcres(acres)} acres`);
    if (Number.isFinite(contained)) metrics.push(`${Math.round(contained)}% contained`);
    else metrics.push('Containment not reported');
    if (Number.isFinite(distance)) metrics.push(formatDistance(distance));
    if (fire.updated) metrics.push(`Updated ${relativeAge(new Date(fire.updated))}`);
    else if (fire.discovered) metrics.push(`Discovered ${relativeAge(new Date(fire.discovered))}`);

    const href = safeFireURL(fire.details_url);
    const titleText = esc(fire.name || 'Unnamed wildfire');
    const title = href
      ? `<a href="${esc(href)}" target="_blank" rel="noopener noreferrer">${titleText}</a>`
      : titleText;
    const incidentID = !fire.named && fire.incident_id
      ? `<span class="fire-id">Incident ${esc(fire.incident_id)}</span>`
      : '';

    return `<div class="fire-row"><div class="fire-marker" aria-hidden="true">▲</div><div class="fire-detail"><strong>${title}</strong>${incidentID}<span>${esc(location)}</span><span>${esc(metrics.join(' · '))}</span></div></div>`;
  }).join('');

  body.innerHTML = `<div class="fire-list">${rows}</div><div class="status-line">${data.incidents.length} nearest current wildfire${data.incidents.length === 1 ? '' : 's'} · within ${formatDistance(Number(data.radius_km))}${filterText} · Source: ${esc(source || 'NIFC WFIGS')}${mapLink}</div>`;
}

function displayFireState(raw) {
  const value = String(raw || '').trim();
  return value.startsWith('US-') ? value.slice(3) : value;
}

function safeFireURL(raw) {
  if (!raw) return null;
  try {
    const url = new URL(raw);
    if (url.protocol !== 'https:') return null;
    if (url.hostname === 'inciweb.wildfire.gov' || url.hostname === 'egp.wildfire.gov') return url.href;
  } catch (_) {}
  return null;
}

function formatAcres(value) {
  if (!Number.isFinite(value)) return '?';
  if (value >= 1000) return Math.round(value).toLocaleString();
  if (value >= 10) return Math.round(value).toString();
  return value.toFixed(1).replace(/\.0$/, '');
}

function formatDistance(km) {
  if (!Number.isFinite(km)) return 'distance unavailable';
  const miles = km * 0.621371;
  return `${Math.round(miles)} mi`;
}

function safeUSGSURL(raw) {
  if (!raw) return null;
  try {
    const url = new URL(raw);
    if (url.protocol === 'https:' && (url.hostname === 'earthquake.usgs.gov' || url.hostname.endsWith('.usgs.gov'))) return url.href;
  } catch (_) {}
  return null;
}

function relativeAge(date) {
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
  if (!Number.isFinite(seconds)) return 'time unavailable';
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h ago`;
}

function renderSystem(body, data) {
  if (!data) {
    body.innerHTML = '<div class="empty">No system data</div>';
    return;
  }
  body.innerHTML = `<div class="system-grid"><div class="metric"><small>Host</small>${esc(data.hostname)}</div><div class="metric"><small>Platform</small>${esc(data.goos)}/${esc(data.goarch)}</div><div class="metric"><small>Memory alloc.</small>${Number(data.memory_alloc_mb).toFixed(1)} MB</div><div class="metric"><small>Goroutines</small>${data.goroutines}</div><div class="metric"><small>Process uptime</small>${Math.floor(data.process_uptime_seconds / 60)} min</div><div class="metric"><small>Go</small>${esc(data.go_version)}</div></div>`;
}

function updateOverall() {
  const health = $('#overall-health');
  const expired = document.querySelectorAll('.widget.expired,.widget.unavailable').length;
  const stale = document.querySelectorAll('.widget.stale').length;
  if (!expired && !stale) {
    health.textContent = 'All sources normal';
    return;
  }
  health.textContent = `Attention: ${stale} stale, ${expired} unavailable`;
}

init().catch(error => {
  $('#dashboard').innerHTML = `<div class="empty">Startup error: ${esc(error.message)}</div>`;
  $('#overall-health').textContent = 'Startup error';
});
