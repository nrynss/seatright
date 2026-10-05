/**
 * Live stage-4 diner probe for an applied seating repair and three upgrades.
 *
 * Talks to the real stage-1, stage-2, and stage-3 images and the current
 * stage-4 image named by the URL arguments. Manager preview and apply are
 * HTTP from this process. The page never calls replans, series, or explain.
 * Each upgrade exports the same source that just committed, imports those
 * unchanged bytes, and retries in the original document.
 *
 * Does not print tokens, passwords, idempotency keys, request bodies, or
 * export JSON. Reveal checks never call scrollIntoView. Does not call
 * POST /series/{id}/amend.
 *
 *   node scripts/s4-g2-live.mjs --stage1 URL --stage2 URL --stage3 URL --destination URL --evidence DIR
 */
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { chmod, mkdir, readFile, readdir, writeFile, appendFile, stat, unlink } from 'node:fs/promises';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { contrastGate } from '@nrynss/chaaya/testing';
import { chromium } from 'playwright';

const worktree = '/home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-grok';
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const axePath = join(worktree, 'stage-4/web/node_modules/axe-core/axe.min.js');
const SECRET = 'correct horse';
const thursday = '2027-06-17';
const sunday = '2027-06-13';
const pastThursday = '2026-09-24';
const provenImages = {
  src1: 'sha256:f22aa004d45b6cc324baa7c6c40b41cf70aeba5b10de6ca510bbb832bca7dea0',
  src2: 'sha256:4b801131c30aca4ed974ecd5a4bb01b67f9aea0aac05885ebdf00188fa600a60',
  src3: 'sha256:44400dbcef867f514e4b5506007bca6702eebb8d0027c2393bb5487736c74420',
};
// Source 3 is the image built from this worktree's stage-3 folder. That folder's
// git tree matches accepted commit e13272d90213be0914211ae6f6f0bfd5888900ff.
// The older S3-G2B image is retained and is not this source.
const source3Provenance = {
  tag: 'tablekeeper:s4-g2a-stage3',
  imageId: 'sha256:44400dbcef867f514e4b5506007bca6702eebb8d0027c2393bb5487736c74420',
  acceptedCommit: 'e13272d90213be0914211ae6f6f0bfd5888900ff',
  acceptedTree: 'c783f9e08522a04a62bb11f9a0e3c485d077684f',
  retainedPriorImage: 'sha256:b3340493160ec658b5fbb68716cde4698a8a0986ef3040bf77b7c53ae8679224',
};

const args = process.argv.slice(2);
function arg(name) {
  const index = args.indexOf(`--${name}`);
  return index >= 0 ? args[index + 1] : '';
}

const stage1 = arg('stage1');
const stage2 = arg('stage2');
const stage3 = arg('stage3');
const destination = arg('destination');
const evidence = arg('evidence');

if (!stage1 || !stage2 || !stage3 || !destination || !evidence) {
  console.error('usage: node scripts/s4-g2-live.mjs --stage1 URL --stage2 URL --stage3 URL --destination URL --evidence DIR');
  process.exit(2);
}

const shotDir = join(evidence, 'screens');
const videoDir = join(evidence, 'videos');
const privateDir = join(evidence, 'private');
const logPath = join(evidence, 'probe.log');
const report = {
  item: 'S4-G2A',
  stage1,
  stage2,
  stage3,
  destination,
  checks: {},
  shots: [],
  videos: [],
  offOrigin: [],
  consoleErrors: [],
  pageErrors: [],
  axe: [],
  layout: [],
  renderedContrast: [],
  reveal: [],
  paint: [],
  publicGets: [],
  upgrades: {},
  repair: {},
  nonClaims: [
    'recurring HTTP amend clock was not called',
    'strict native stage-4 reassigned import was not claimed',
    'modern stage-4 to stage-4 portability was not claimed',
    'full harness and stage acceptance remain later',
  ],
};
const failures = [];

function log(message) {
  const line = `${message}\n`;
  process.stdout.write(line);
  return appendFile(logPath, line);
}

function sha256(bytes) {
  return createHash('sha256').update(bytes).digest('hex');
}

function weekday(iso) {
  return ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat'][new Date(`${iso}T00:00:00Z`).getUTCDay()];
}

function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonical(value[key])]));
  }
  return value;
}

function sameJsonText(left, right) {
  return JSON.stringify(canonical(JSON.parse(left))) === JSON.stringify(canonical(JSON.parse(right)));
}

function errorCode(text) {
  try {
    return JSON.parse(text)?.error?.code ?? '';
  } catch {
    return '';
  }
}

function pass(id, detail) {
  report.checks[id] = { result: 'pass', detail };
  return log(`PASS ${id} ${detail}`);
}

function fail(message) {
  failures.push(message);
  throw new Error(message);
}

async function expectCheck(id, ok, detail) {
  if (!ok) fail(`${id} ${detail}`);
  await pass(id, detail);
}

function receiptSummary(text) {
  const value = JSON.parse(text);
  const terms = value.accepted_terms && typeof value.accepted_terms === 'object' ? value.accepted_terms : null;
  return {
    reference: typeof value.reference === 'string' ? value.reference : '',
    hasTableId: Object.prototype.hasOwnProperty.call(value, 'table_id'),
    hasTableIds: Object.prototype.hasOwnProperty.call(value, 'table_ids'),
    tableId: typeof value.table_id === 'string' ? value.table_id : '',
    tableIds: Array.isArray(value.table_ids) ? value.table_ids : null,
    hasRevision: Object.prototype.hasOwnProperty.call(value, 'revision'),
    hasAcceptedTerms: Object.prototype.hasOwnProperty.call(value, 'accepted_terms'),
    revision: Number.isInteger(value.revision) ? value.revision : null,
    policyVersion: terms && Number.isInteger(terms.policy_version) ? terms.policy_version : null,
    slot: terms?.slot_minutes ?? null,
    duration: terms?.reservation_duration_minutes ?? null,
    cutoff: terms?.cancellation_cutoff_minutes ?? null,
    capacities: terms?.capacities ?? null,
    termsHaveEffective: Boolean(terms && Object.prototype.hasOwnProperty.call(terms, 'effective_from')),
    party: value.party_size ?? null,
    starts: value.starts_at_local ?? '',
    ends: value.ends_at ?? '',
    status: value.status ?? '',
    bytes: Buffer.byteLength(text),
    sha256: sha256(text),
  };
}

function frozenEqual(before, after) {
  const keys = ['accepted_terms', 'starts_at_local', 'starts_at', 'ends_at', 'party_size', 'restaurant_id', 'reservation_id', 'reference', 'created_at'];
  const changed = [];
  for (const key of keys) {
    if (JSON.stringify(before[key]) !== JSON.stringify(after[key])) changed.push(key);
  }
  return changed;
}

function elapsedMinutes(starts, ends) {
  const delta = Date.parse(ends) - Date.parse(starts);
  return Number.isFinite(delta) ? delta / 60000 : null;
}

const hours = (day, opens, closes) => [{ weekday: day, opens, closes }];

function anker(extra = {}) {
  return {
    id: 'r_anker',
    name: 'Zum Anker',
    timezone: 'Europe/Berlin',
    slot_minutes: 30,
    reservation_duration_minutes: 90,
    cancellation_cutoff_minutes: 120,
    opening_hours: hours('thu', '18:00', '23:00'),
    tables: [
      { id: 't_1', label: '1', capacity: 2 },
      { id: 't_2', label: '2', capacity: 4 },
      { id: 't_3', label: '3', capacity: 4 },
    ],
    ...extra,
  };
}

function repairFixture() {
  return {
    users: [
      {
        id: 'u_mgr',
        email: 'mira.manager@example.com',
        password: SECRET,
        display_name: 'Mira',
      },
    ],
    restaurants: [
      anker({
        tables: [
          { id: 't_1', label: '1', capacity: 2 },
          { id: 't_2', label: '2', capacity: 4 },
          { id: 't_3', label: '3', capacity: 6 },
        ],
        combinable: [['t_1', 't_2'], ['t_2', 't_3']],
        manager_user_ids: ['u_mgr'],
      }),
      {
        id: 'r_nord',
        name: 'Nordlicht',
        timezone: 'America/New_York',
        slot_minutes: 30,
        reservation_duration_minutes: 90,
        cancellation_cutoff_minutes: 120,
        opening_hours: hours('thu', '17:00', '22:00'),
        tables: [
          { id: 't_window', label: 'Window', capacity: 2 },
          { id: 't_corner', label: 'Corner', capacity: 4 },
        ],
      },
      {
        id: 'r_hall',
        name: 'The Long Room',
        timezone: 'Europe/Berlin',
        slot_minutes: 30,
        reservation_duration_minutes: 90,
        cancellation_cutoff_minutes: 120,
        opening_hours: hours('thu', '18:00', '22:00'),
        tables: [
          { id: 't_alcove', label: 'Window alcove', capacity: 2 },
          { id: 't_garden', label: 'Garden corner', capacity: 4 },
          { id: 't_hearth', label: 'Hearth booth', capacity: 4 },
        ],
        combinable: [['t_alcove', 't_garden']],
      },
    ],
    reservations: [],
  };
}

function stage1Fixture() {
  return {
    users: [],
    restaurants: [
      {
        id: 'r_anker',
        name: 'Zum Anker',
        timezone: 'Europe/Berlin',
        slot_minutes: 30,
        reservation_duration_minutes: 90,
        cancellation_cutoff_minutes: 120,
        opening_hours: hours('thu', '18:00', '23:00'),
        tables: [
          { id: 't_1', label: '1', capacity: 2 },
          { id: 't_2', label: '2', capacity: 4 },
        ],
      },
    ],
    reservations: [],
  };
}

function stage2Fixture() {
  return {
    users: [],
    restaurants: [anker({ combinable: [['t_1', 't_2']] })],
    reservations: [],
  };
}

function denseRestaurant(step) {
  return {
    users: [],
    restaurants: [
      {
        id: 'r_dense',
        name: 'The Long Room',
        timezone: 'Europe/Berlin',
        slot_minutes: step,
        reservation_duration_minutes: 90,
        cancellation_cutoff_minutes: 120,
        opening_hours: hours('thu', '18:00', '23:00'),
        tables: [
          { id: 't_1', label: 'Window alcove', capacity: 2 },
          { id: 't_2', label: 'Garden corner', capacity: 4 },
          { id: 't_3', label: 'Hearth booth', capacity: 4 },
        ],
        combinable: [['t_1', 't_2'], ['t_2', 't_3']],
      },
    ],
    reservations: [],
  };
}

async function reset(base, fixture) {
  const response = await fetch(`${base}/_test/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify(fixture),
  });
  if (response.status !== 204) fail(`reset status ${response.status}`);
}

async function waitHealth(base) {
  const started = Date.now();
  let last = 'none';
  while (Date.now() - started < 30000) {
    try {
      const response = await fetch(`${base}/health`);
      last = String(response.status);
      if (response.status === 200) {
        const text = await response.text();
        const contentType = response.headers.get('content-type') ?? '';
        const body = JSON.parse(text);
        if (body.status !== 'ok') fail(`health body status ${body.status}`);
        if (text !== '{"status":"ok"}') fail(`health body bytes ${text.length}`);
        if (contentType !== 'application/json; charset=utf-8') fail(`health content-type ${contentType}`);
        return { contentType, body: text, sha256: sha256(text) };
      }
    } catch (error) {
      last = error instanceof Error ? error.message : 'error';
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  fail(`health not ready (${last})`);
}

function randomKey() {
  return [...crypto.getRandomValues(new Uint8Array(16))].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

async function api(base, path, { method = 'GET', token = '', key = '', body = undefined, rawBody = undefined } = {}) {
  const headers = { Accept: 'application/json' };
  if (body !== undefined || rawBody !== undefined) headers['Content-Type'] = 'application/json; charset=utf-8';
  if (token) headers.Authorization = `Bearer ${token}`;
  if (key) headers['Idempotency-Key'] = key;
  const response = await fetch(`${base}${path}`, {
    method,
    headers,
    body: rawBody !== undefined ? rawBody : body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await response.text();
  return { status: response.status, text };
}

async function signup(base, email, displayName) {
  const result = await api(base, '/auth/signup', {
    method: 'POST',
    body: { email, password: SECRET, display_name: displayName },
  });
  if (result.status !== 201) fail(`signup ${result.status} ${errorCode(result.text)}`);
  return JSON.parse(result.text).token;
}

function inspectContainer(name) {
  const raw = execFileSync('docker', ['inspect', name], { encoding: 'utf8' });
  const info = JSON.parse(raw)[0];
  return {
    name: info.Name,
    id: info.Id,
    image: info.Config?.Image,
    imageId: info.Image,
    portEnv: (info.Config?.Env ?? []).find((item) => item.startsWith('PORT=')) ?? '',
    ports: info.NetworkSettings?.Ports ?? {},
    status: info.State?.Status,
    pid: info.State?.Pid ?? 0,
    nanoCpus: info.HostConfig?.NanoCpus ?? 0,
    memory: info.HostConfig?.Memory ?? 0,
  };
}

function hostPortOf(url) {
  const parsed = new URL(url);
  if (!parsed.port) fail(`${url} has no explicit port`);
  return parsed.port;
}

function containerOnHostPort(hostPort) {
  const ids = execFileSync('docker', ['ps', '-q', '--filter', `publish=${hostPort}`], { encoding: 'utf8' })
    .trim()
    .split('\n')
    .filter(Boolean);
  if (ids.length !== 1) fail(`publish ${hostPort} matched ${ids.length} containers`);
  const info = inspectContainer(ids[0]);
  const published = Object.values(info.ports)
    .flat()
    .filter(Boolean)
    .some((binding) => binding.HostPort === hostPort);
  if (!published) fail(`${info.name} does not publish ${hostPort}`);
  return info;
}

function isApi(pathname) {
  return (
    pathname === '/health' ||
    pathname.startsWith('/restaurants') ||
    pathname.startsWith('/availability') ||
    pathname.startsWith('/reservations') ||
    pathname.startsWith('/auth/') ||
    pathname.startsWith('/reservation-moves') ||
    pathname.startsWith('/series') ||
    pathname.startsWith('/_test/')
  );
}

function noteUrl(url) {
  if (url.hostname !== '127.0.0.1' && url.hostname !== 'localhost') {
    report.offOrigin.push(`${url.origin}${url.pathname}`);
  }
}

function isPublicGet(pathname) {
  return (
    pathname === '/restaurants' ||
    pathname === '/availability' ||
    /^\/restaurants\/[^/]+$/.test(pathname) ||
    /^\/restaurants\/[^/]+\/policies$/.test(pathname)
  );
}

async function installRouter(page, state) {
  await page.route('**/*', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    noteUrl(url);
    if (isApi(url.pathname)) {
      state.apiPaths.push(`${request.method()} ${url.pathname}${url.search}`);
    }
    if (request.method() === 'GET' && isPublicGet(url.pathname)) {
      const row = {
        path: `${url.pathname}${url.search}`,
        hasAuthorization: Boolean(request.headers().authorization),
      };
      state.publicGets.push(row);
      if (url.pathname === '/availability') state.availabilityParties.push(url.searchParams.get('party_size'));
    }
    if (state.hold && state.hold(url, request)) {
      await state.holdGate;
    }
    if (state.failAvailability && request.method() === 'GET' && url.pathname === '/availability') {
      state.failAvailability = false;
      await route.abort('failed');
      return;
    }
    if (state.dropNextReservation && request.method() === 'POST' && url.pathname === '/reservations') {
      state.dropNextReservation = false;
      const fetched = await route.fetch();
      const fresh = await fetched.text();
      state.dropped = {
        status: fetched.status(),
        text: fresh,
        key: request.headers()['idempotency-key'] ?? '',
        body: request.postData() ?? '',
        servedToBrowser: false,
        fetchedUrl: request.url(),
        method: request.method(),
      };
      await route.abort('failed');
      return;
    }
    const target = state.forward && isApi(url.pathname)
      ? new URL(`${url.pathname}${url.search}`, state.forward).toString()
      : '';
    const watchReservation = state.captureReservation && request.method() === 'POST' && url.pathname === '/reservations';
    const watchLookup = state.captureLookup && request.method() === 'GET' && /^\/reservations\/[^/]+$/.test(url.pathname);
    if (target || watchReservation || watchLookup) {
      const fetched = target ? await route.fetch({ url: target }) : await route.fetch();
      const headers = { ...fetched.headers() };
      delete headers['content-encoding'];
      delete headers['content-length'];
      const fresh = await fetched.text();
      if (watchReservation) {
        state.reservations.push({
          status: fetched.status(),
          text: fresh,
          key: request.headers()['idempotency-key'] ?? '',
          body: request.postData() ?? '',
          fetchedUrl: target || request.url(),
          method: request.method(),
          cachedBodyReused: false,
        });
      }
      if (watchLookup) {
        state.lookups.push({ status: fetched.status(), text: fresh, fetchedUrl: target || request.url() });
      }
      await route.fulfill({ status: fetched.status(), headers, body: fresh });
      return;
    }
    await route.continue();
  });
}

async function launch(browser, { width, height, video, reduced = false, theme = 'light' }) {
  const context = await browser.newContext({
    viewport: { width, height },
    reducedMotion: reduced ? 'reduce' : 'no-preference',
    colorScheme: 'dark',
    recordVideo: video ? { dir: videoDir, size: { width, height } } : undefined,
    serviceWorkers: 'block',
  });
  await context.addInitScript((mode) => {
    localStorage.setItem('chaaya-theme', mode);
  }, theme);
  const page = await context.newPage();
  page.setDefaultTimeout(12000);
  page.on('pageerror', (error) => report.pageErrors.push(String(error.message ?? error)));
  page.on('console', (message) => {
    if (message.type() === 'error') report.consoleErrors.push(message.text());
  });
  page.on('request', (request) => {
    try {
      noteUrl(new URL(request.url()));
    } catch {
      // Ignore unparsable browser URLs.
    }
  });
  return { context, page };
}

function freshState() {
  return {
    reservations: [],
    lookups: [],
    publicGets: [],
    availabilityParties: [],
    apiPaths: [],
    dropNextReservation: false,
    failAvailability: false,
    captureReservation: false,
    captureLookup: false,
    forward: '',
    dropped: null,
    hold: null,
    holdGate: null,
  };
}

async function pageBox(page) {
  return page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
}

async function assertNoPageOverflow(page, label) {
  const box = await pageBox(page);
  const overflow = box.scrollWidth > box.clientWidth + 1;
  report.layout.push({ label, ...box, overflow });
  if (overflow) failures.push(`${label} horizontal overflow ${box.scrollWidth}>${box.clientWidth}`);
}

async function measureSettled(page, label) {
  const measured = await page.evaluate(() => {
    const plateOutside = [];
    for (const text of document.querySelectorAll('.plate-label')) {
      const group = text.closest('.table-on-plan');
      const top = group?.querySelector('.table-top');
      if (!top || typeof text.getBBox !== 'function' || typeof top.getBBox !== 'function') continue;
      const ink = text.getBBox();
      const box = top.getBBox();
      const corners = [
        [ink.x, ink.y],
        [ink.x + ink.width, ink.y],
        [ink.x, ink.y + ink.height],
        [ink.x + ink.width, ink.y + ink.height],
      ];
      let outside = false;
      if (top.tagName.toLowerCase() === 'circle') {
        const cx = Number(top.getAttribute('cx'));
        const cy = Number(top.getAttribute('cy'));
        const radius = Number(top.getAttribute('r')) + 0.8;
        outside = corners.some(([x, y]) => (x - cx) ** 2 + (y - cy) ** 2 > radius ** 2);
      } else {
        outside = corners.some(
          ([x, y]) => x < box.x - 0.8 || y < box.y - 0.8 || x > box.x + box.width + 0.8 || y > box.y + box.height + 0.8,
        );
      }
      if (outside) plateOutside.push(text.textContent ?? '');
    }
    const wordOutside = [];
    for (const word of document.querySelectorAll('.state-word')) {
      const cell = word.closest('button');
      if (!cell) continue;
      const ink = word.getBoundingClientRect();
      const bounds = cell.getBoundingClientRect();
      if (
        ink.width > 0 &&
        (ink.left < bounds.left - 1 || ink.top < bounds.top - 1 || ink.right > bounds.right + 1 || ink.bottom > bounds.bottom + 1)
      ) {
        wordOutside.push(word.textContent ?? '');
      }
    }
    return { plateOutside, wordOutside };
  });
  report.layout.push({ label, ...measured });
  if (measured.plateOutside.length || measured.wordOutside.length) {
    failures.push(`${label} plate ${measured.plateOutside.length} word ${measured.wordOutside.length}`);
  }
}

async function runAxe(page, label) {
  await page.addScriptTag({ path: axePath });
  const violations = await page.evaluate(async () => {
    const results = await window.axe.run(document, {
      resultTypes: ['violations'],
      rules: {
        'color-contrast': { enabled: false },
        region: { enabled: false },
      },
    });
    return results.violations
      .filter((item) => item.impact === 'serious' || item.impact === 'critical')
      .map((item) => ({
        id: item.id,
        impact: item.impact,
        nodes: item.nodes.slice(0, 3).map((node) => node.target?.join(' ')),
      }));
  });
  const titles = await page.locator('[title]').count();
  const dead = await page.evaluate(() => [...document.querySelectorAll('button, a[href], input, select, textarea')].filter((element) => {
    if (element.hasAttribute('disabled')) return false;
    return element.getAttribute('tabindex') === '-1';
  }).length);
  report.axe.push({ label, violations, titles, dead, disabledRules: ['color-contrast', 'region'] });
  if (violations.length || titles || dead) {
    failures.push(`${label} axe ${violations.map((item) => item.id).join(',') || 'title-or-focus'}`);
  }
}

function parseCssColor(input) {
  const match = String(input).match(/rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)(?:[,\s/]+([\d.]+%?))?\s*\)/i);
  if (!match) return null;
  let alpha = match[4] === undefined ? 1 : Number(String(match[4]).replace('%', ''));
  if (String(match[4] ?? '').endsWith('%')) alpha /= 100;
  return { r: Number(match[1]), g: Number(match[2]), b: Number(match[3]), a: alpha };
}

function channel(value) {
  const unit = value / 255;
  return unit <= 0.04045 ? unit / 12.92 : ((unit + 0.055) / 1.055) ** 2.4;
}

function contrastRatio(foreground, background) {
  const fg = parseCssColor(foreground);
  const bg = parseCssColor(background);
  if (!fg || !bg || fg.a === 0) return null;
  const first = 0.2126 * channel(fg.r) + 0.7152 * channel(fg.g) + 0.0722 * channel(fg.b);
  const second = 0.2126 * channel(bg.r) + 0.7152 * channel(bg.g) + 0.0722 * channel(bg.b);
  const lighter = Math.max(first, second);
  const darker = Math.min(first, second);
  return (lighter + 0.05) / (darker + 0.05);
}

function hexOf(color) {
  const parsed = parseCssColor(color);
  if (!parsed) return String(color);
  return `#${[parsed.r, parsed.g, parsed.b].map((part) => Math.round(part).toString(16).padStart(2, '0')).join('')}`.toUpperCase();
}

async function readPaint(page, testId) {
  return page.evaluate((id) => {
    const cell = document.querySelector(`[data-testid="${id}"]`);
    if (!cell) return null;
    const word = cell.querySelector('.state-word');
    if (!word) return null;
    const style = getComputedStyle(word);
    let node = word;
    let background = '';
    while (node) {
      const painted = getComputedStyle(node).backgroundColor;
      const match = painted.match(/rgba?\(\s*[\d.]+[,\s]+[\d.]+[,\s]+[\d.]+(?:[,\s/]+([\d.]+))?\s*\)/);
      const alpha = match && match[1] !== undefined ? Number(match[1]) : match ? 1 : 0;
      if (painted && painted !== 'transparent' && alpha !== 0) {
        background = painted;
        break;
      }
      node = node.parentElement;
    }
    return {
      word: (word.textContent || '').trim(),
      color: style.color,
      background,
      cellBackground: getComputedStyle(cell).backgroundColor,
      hidden: style.display === 'none' || style.visibility === 'hidden' || Number.parseFloat(style.fontSize) === 0,
      available: cell.getAttribute('data-available'),
      selected: cell.getAttribute('data-selected'),
      fontSize: style.fontSize,
      fontWeight: style.fontWeight,
    };
  }, testId);
}

async function measureRenderedContrast(page, samples, label) {
  const saved = page.viewportSize();
  for (const width of [1280, 375]) {
    await page.setViewportSize({ width, height: width === 375 ? 812 : 900 });
    for (const theme of ['light', 'dark', 'system']) {
      await page.getByTestId(`theme-${theme}`).click();
      await page.waitForTimeout(120);
      for (const sample of samples) {
        const paint = await readPaint(page, sample.testId);
        if (!paint) fail(`missing painted cell ${sample.testId}`);
        if (paint.word !== sample.word) fail(`${sample.testId} word ${paint.word} expected ${sample.word}`);
        if (paint.hidden) fail(`${sample.testId} state word is hidden`);
        if (paint.available !== sample.available || paint.selected !== sample.selected) {
          fail(`${sample.testId} available=${paint.available} selected=${paint.selected}`);
        }
        const ratio = contrastRatio(paint.color, paint.background);
        const row = {
          label,
          role: sample.role,
          testId: sample.testId,
          width,
          theme,
          word: paint.word,
          foreground: hexOf(paint.color),
          background: hexOf(paint.background),
          ratio: ratio === null ? null : Math.round(ratio * 1000) / 1000,
          available: paint.available,
          selected: paint.selected,
        };
        row.ratioText = row.ratio === null ? 'none' : row.ratio.toFixed(3);
        report.renderedContrast.push(row);
        await log(`CONTRAST ${row.role} ${width} ${theme} ${row.word} ${row.foreground} on ${row.background} ${row.ratioText}:1`);
        if (row.ratio === null || row.ratio < 4.5) fail(`${row.role} ${width} ${theme} contrast ${row.ratioText}`);
      }
    }
  }
  await page.setViewportSize(saved ?? { width: 1280, height: 900 });
  await page.getByTestId('theme-light').click();
}

function runContrastGate() {
  const pairsSource = readFileSync(new URL('../src/lib/pairs.ts', import.meta.url), 'utf8');
  const pairs = [...pairsSource.matchAll(/\['([a-z0-9-]+)', '([a-z0-9-]+)'\]/g)].map((match) => [match[1], match[2]]);
  const themeCss = readFileSync(new URL('../src/theme.css', import.meta.url), 'utf8');
  contrastGate(themeCss, pairs);
  report.contrastGate = { pairs: pairs.length, result: 'pass' };
}

async function captureQuartet(page, name, settleMs = 400) {
  if (settleMs) await page.waitForTimeout(settleMs);
  for (const width of [1280, 375]) {
    await page.setViewportSize({ width, height: width === 375 ? 812 : 900 });
    for (const theme of ['light', 'dark', 'system']) {
      await page.getByTestId(`theme-${theme}`).click();
      await page.waitForTimeout(180);
      const file = join(shotDir, `${name}-${width}-${theme}.png`);
      await page.screenshot({ path: file, fullPage: true });
      report.shots.push(file);
      await assertNoPageOverflow(page, `${name}-${width}-${theme}`);
    }
  }
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.getByTestId('theme-light').click();
}

async function signupUi(page, base, email, displayName) {
  await page.goto(`${base}/signup`);
  await page.getByTestId('signup-email').fill(email);
  await page.getByTestId('signup-password').fill(SECRET);
  await page.getByTestId('signup-display-name').fill(displayName);
  await page.getByTestId('signup-submit').click();
  await page.getByTestId('current-user').waitFor();
  const name = (await page.getByTestId('current-user').innerText()).trim();
  if (displayName && !name.includes(displayName)) fail(`signed-in name missing ${displayName}`);
}

async function loginUi(page, base, email) {
  await page.goto(`${base}/login`);
  await page.getByTestId('login-email').fill(email);
  await page.getByTestId('login-password').fill(SECRET);
  await page.getByTestId('login-submit').click();
  await page.getByTestId('current-user').waitFor();
}

async function search(page, restaurant, date, party) {
  const select = page.getByTestId('restaurant-select');
  await select.waitFor();
  await select.locator(`option[value="${restaurant}"]`).waitFor({ state: 'attached' });
  await select.selectOption(restaurant);
  await page.getByTestId('date-input').fill(date);
  await page.getByTestId('party-size-input').fill(String(party));
  await page.getByTestId('search-button').click();
}

async function waitSearch(page) {
  await page.waitForFunction(
    () => document.querySelector('[data-testid="availability-grid"], [data-testid="no-slots"], [data-testid="search-error"]'),
    null,
    { timeout: 12000 },
  );
}

function clockOf(startsAtLocal) {
  return startsAtLocal.slice(11, 16);
}

async function clickAndReadSelected(page, testId) {
  const locator = page.getByTestId(testId);
  await locator.click();
  const selected = await locator.getAttribute('data-selected');
  const running = await locator.evaluate((element) => element.getAnimations({ subtree: true }).filter((item) => item.playState === 'running').length);
  return { selected, running };
}

async function bookFromForm(page, state) {
  const before = state.reservations.length;
  await page.getByTestId('booking-submit').click();
  const started = Date.now();
  while (state.reservations.length === before && Date.now() - started < 8000) {
    await page.waitForTimeout(40);
  }
  if (state.reservations.length === before) fail('booking request was not observed');
  return state.reservations[state.reservations.length - 1];
}

async function finishVideo(context, page, name) {
  const video = page.video();
  await context.close();
  if (!video) return;
  const target = join(videoDir, name);
  const raw = await video.path();
  await video.saveAs(target);
  if (raw && raw !== target) await unlink(raw).catch(() => undefined);
  report.videos.push(target);
  await log(`VIDEO ${name}`);
}

async function navLabels(page) {
  return page.getByRole('navigation', { name: 'Primary' }).evaluate((nav) =>
    [...nav.querySelectorAll('a')].map((link) => (link.textContent || '').trim()),
  );
}

async function assertSessionChrome(page, { signedIn, display }) {
  const labels = await navLabels(page);
  const signIn = labels.includes('Sign in');
  const create = labels.includes('Create account');
  if (!labels.includes('Search') || !labels.includes('Look up')) fail(`primary nav ${labels.join('|')}`);
  if (signedIn) {
    if (signIn || create) fail(`signed-in nav still has auth links ${labels.join('|')}`);
    if ((await page.getByTestId('current-user').count()) !== 1) fail('current-user missing while signed in');
    if ((await page.getByTestId('logout-button').count()) !== 1) fail('logout-button missing while signed in');
    if (display !== undefined) {
      const text = (await page.getByTestId('current-user').innerText()).trim();
      if (text !== display) fail(`current-user ${JSON.stringify(text)}`);
    }
  } else if (!signIn || !create) {
    fail(`signed-out nav ${labels.join('|')}`);
  } else if ((await page.getByTestId('current-user').count()) !== 0) {
    fail('current-user present while signed out');
  }
}

async function knownScroll(page) {
  await page.evaluate(() => window.scrollTo(0, 0));
}

async function settledScroll(page) {
  let last = await page.evaluate(() => window.scrollY);
  let stableFor = 0;
  const step = 50;
  for (let elapsed = 0; elapsed < 2500; elapsed += step) {
    await page.waitForTimeout(step);
    const y = await page.evaluate(() => window.scrollY);
    if (Math.abs(y - last) < 0.5) stableFor += step;
    else stableFor = 0;
    last = y;
    if (elapsed >= 300 && stableFor >= 300) return y;
  }
  return last;
}

async function readBox(page, selector) {
  return page.evaluate((sel) => {
    const node = document.querySelector(sel);
    const viewH = window.innerHeight;
    const viewW = window.innerWidth;
    if (!node) return { present: false, scrollY: window.scrollY, viewH, viewW };
    const rect = node.getBoundingClientRect();
    const visible = Math.max(0, Math.min(rect.bottom, viewH) - Math.max(rect.top, 0));
    return {
      present: true,
      scrollY: window.scrollY,
      viewH,
      viewW,
      top: Math.round(rect.top * 10) / 10,
      bottom: Math.round(rect.bottom * 10) / 10,
      visible: Math.round(visible * 10) / 10,
    };
  }, selector);
}

async function assertRevealed(page, label, selector, minVisible) {
  const box = await readBox(page, selector);
  report.reveal.push({ label, ...box });
  await log(`REVEAL ${label} present=${box.present} scrollY=${box.scrollY} top=${box.top} visible=${box.visible}`);
  if (!box.present) fail(`${label} missing`);
  if (box.visible < minVisible || box.top >= box.viewH - 8) fail(`${label} not revealed visible=${box.visible} top=${box.top}`);
  return box;
}

async function measurePaint(page, label) {
  const paint = await page.evaluate(() => {
    const svg = document.querySelector('svg.room-scene');
    if (!svg) return { missing: true };
    const nodes = [...svg.querySelectorAll('*')];
    const indexOf = (selector) => nodes.findIndex((node) => node.matches(selector));
    const link = indexOf('.pair-link');
    const top = indexOf('.table-top');
    const plate = indexOf('.plate-label');
    const badge = indexOf('.pair-badge');
    const planNames = [...svg.querySelectorAll('.plan-name')].map((node) => (node.textContent || '').trim());
    const plates = [...svg.querySelectorAll('.plate-label')].map((node) => ({
      text: (node.textContent || '').trim(),
      font: Number(node.getAttribute('font-size')),
    }));
    const seatLines = [...svg.querySelectorAll('.plan-seats')].map((node) => ({
      text: (node.textContent || '').trim(),
      font: Number(node.getAttribute('font-size')),
    }));
    const duplicatePlates = plates.map((item) => item.text).filter((text, index, all) => all.indexOf(text) !== index);
    return { link, top, plate, badge, planNames, plates, seatLines, duplicatePlates };
  });
  report.paint.push({ label, ...paint });
  await log(`PAINT ${label} order link=${paint.link} top=${paint.top} plate=${paint.plate} badge=${paint.badge}`);
  if (paint.missing) fail(`${label} floor missing`);
  if (!(paint.link >= 0 && paint.top > paint.link && paint.plate > paint.top && paint.badge > paint.plate)) {
    fail(`${label} paint order ${paint.link}/${paint.top}/${paint.plate}/${paint.badge}`);
  }
  if ((paint.planNames || []).length !== 0) fail(`${label} rendered plan-name`);
  if ((paint.duplicatePlates || []).length !== 0) fail(`${label} duplicate plate labels`);
  for (const plate of paint.plates || []) {
    if (!plate.text || plate.font < 13) fail(`${label} plate ${plate.text} ${plate.font}`);
  }
  for (const line of paint.seatLines || []) {
    if (line.font < 13 || !/seat/.test(line.text)) fail(`${label} seat line ${line.text}`);
  }
  return paint;
}

async function documentIdentity(page) {
  return page.evaluate(() => ({
    mark: window.__s4g2Mark ?? '',
    doc: document.documentElement.dataset.s4g2Doc ?? '',
    nav: performance.getEntriesByType('navigation').length,
    href: location.href,
  }));
}

async function sessionToken(page) {
  const token = await page.evaluate(() => JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}').token ?? '');
  if (!token) fail('session token missing');
  return token;
}

async function sessionUserId(page) {
  const userId = await page.evaluate(() => JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}').userId ?? '');
  if (!userId) fail('session user id missing');
  return userId;
}

async function availabilityOf(party, date = thursday) {
  const result = await api(destination, `/availability?restaurant_id=r_anker&date=${date}&party_size=${party}`);
  if (result.status !== 200) fail(`availability ${result.status} ${errorCode(result.text)}`);
  return JSON.parse(result.text);
}

async function explained(party, date = thursday) {
  const result = await api(destination, `/availability?restaurant_id=r_anker&date=${date}&party_size=${party}&explain=true`);
  if (result.status !== 200) fail(`explain ${result.status} ${errorCode(result.text)}`);
  return JSON.parse(result.text);
}

function ruleHolds(slot, tableId, rule) {
  const row = (slot.explain ?? []).find((item) => item.table_id === tableId);
  const found = (row?.rules ?? []).find((item) => item.rule === rule);
  return found ? Boolean(found.holds) : null;
}

async function mirrorGrid(page, availability) {
  const mismatches = [];
  const singles = ['t_1', 't_2', 't_3'];
  const pairs = ['t_1+t_2', 't_2+t_3'];
  for (const slot of availability.slots ?? []) {
    const time = clockOf(slot.starts_at_local);
    const open = new Set(slot.available_table_ids ?? []);
    for (const id of singles) {
      const got = await page.getByTestId(`slot-${id}-${time}`).getAttribute('data-available');
      const want = open.has(id) ? 'true' : 'false';
      if (got !== want) mismatches.push(`slot-${id}-${time} ${got}!=${want}`);
    }
    for (const key of pairs) {
      const eligible = (slot.available_options ?? []).some((option) => (option.table_ids ?? []).join('+') === key);
      const got = await page.getByTestId(`slot-${key}-${time}`).getAttribute('data-available');
      const want = eligible ? 'true' : 'false';
      if (got !== want) mismatches.push(`slot-${key}-${time} ${got}!=${want}`);
    }
  }
  return mismatches;
}

async function mirrorFloor(page, slot) {
  const mismatches = [];
  const time = clockOf(slot.starts_at_local);
  const open = new Set(slot.available_table_ids ?? []);
  for (const id of ['t_1', 't_2', 't_3']) {
    const got = await page.getByTestId(`plan-${id}`).getAttribute('data-available');
    const want = open.has(id) ? 'true' : 'false';
    if (got !== want) mismatches.push(`plan-${id} ${got}!=${want} at ${time}`);
  }
  for (const key of ['t_1+t_2', 't_2+t_3']) {
    const eligible = (slot.available_options ?? []).some((option) => (option.table_ids ?? []).join('+') === key);
    const got = await page.getByTestId(`plan-${key}`).getAttribute('data-available');
    const want = eligible ? 'true' : 'false';
    if (got !== want) mismatches.push(`plan-${key} ${got}!=${want} at ${time}`);
  }
  return mismatches;
}

async function restaurantRevision() {
  const result = await api(destination, '/_test/export');
  if (result.status !== 200) fail(`revision export ${result.status}`);
  const value = JSON.parse(result.text)?.state?.restaurant_revisions?.r_anker ?? 0;
  return Number(value);
}

async function previewClosure(token, from, to) {
  return api(destination, '/restaurants/r_anker/replans', {
    method: 'POST',
    token,
    key: randomKey(),
    body: { table_id: 't_2', from, to },
  });
}

async function applyPlan(token, planId) {
  return api(destination, `/restaurants/r_anker/replans/${encodeURIComponent(planId)}/apply`, {
    method: 'POST',
    token,
    key: randomKey(),
    body: {},
  });
}

function assignmentSummary(text) {
  const value = JSON.parse(text);
  return {
    statusKeys: Object.keys(value).sort(),
    planId: typeof value.plan_id === 'string' ? value.plan_id : '',
    restaurantRevision: value.restaurant_revision ?? null,
    movedCount: value.moved_count ?? null,
    unusedSeats: value.unused_seats ?? null,
    assignments: Array.isArray(value.assignments)
      ? value.assignments.map((item) => ({
        reference: item.reference ?? '',
        tableIds: item.table_ids ?? null,
        changed: item.changed ?? null,
      }))
      : null,
    closureTable: value.closure?.table_id ?? '',
  };
}

async function historyOf(token, reference) {
  const result = await api(destination, `/reservations/${encodeURIComponent(reference)}/history`, { token });
  if (result.status !== 200) fail(`history ${result.status} ${errorCode(result.text)}`);
  return JSON.parse(result.text);
}

function historyDigest(payload) {
  return (payload.entries ?? []).map((entry) => ({
    seq: entry.seq ?? null,
    event: entry.event ?? '',
    revision: entry.revision ?? null,
    hasPlan: Object.prototype.hasOwnProperty.call(entry, 'plan_id'),
    planPresent: typeof entry.plan_id === 'string' && entry.plan_id.length > 0,
    changes: (entry.changes ?? []).map((change) => ({
      field: change.field ?? '',
      from: change.from ?? null,
      to: change.to ?? null,
    })),
    policyVersion: entry.accepted_terms?.policy_version ?? null,
    duration: entry.accepted_terms?.reservation_duration_minutes ?? null,
  }));
}

async function ownerMatches(reference, userId) {
  const exported = await fetch(`${destination}/_test/export`);
  const bytes = Buffer.from(await exported.arrayBuffer());
  if (exported.status !== 200) fail(`owner export ${exported.status}`);
  const file = join(privateDir, `owner-${reference}.json`);
  await writeFile(file, bytes);
  await chmod(file, 0o600);
  const state = JSON.parse(bytes.toString('utf8')).state ?? {};
  const reservations = state.reservations;
  let found = null;
  if (Array.isArray(reservations)) found = reservations.find((item) => item && item.reference === reference) ?? null;
  else if (reservations && typeof reservations === 'object') {
    found = Object.values(reservations).find((item) => item && item.reference === reference) ?? null;
  }
  return Boolean(found && found.user_id === userId);
}

async function assertRepairMove(token, reference, planId, beforeText) {
  const before = JSON.parse(beforeText);
  const current = await api(destination, `/reservations/${encodeURIComponent(reference)}`, { token });
  if (current.status !== 200) fail(`current reservation ${current.status} ${errorCode(current.text)}`);
  const after = JSON.parse(current.text);
  const drifted = frozenEqual(before, after);
  if (drifted.length) fail(`frozen fields changed ${drifted.join(',')}`);
  if (after.revision !== before.revision + 1) fail(`revision ${before.revision} -> ${after.revision}`);
  if (JSON.stringify(after.table_ids) !== '["t_3"]' || after.table_id !== 't_3') fail('current tables were not t_3');
  if (elapsedMinutes(after.starts_at, after.ends_at) !== 90) fail(`duration ${after.starts_at} ${after.ends_at}`);
  if (after.accepted_terms?.policy_version !== 0 || after.accepted_terms?.reservation_duration_minutes !== 90) {
    fail('accepted terms were not policy 0 duration 90');
  }
  const history = await historyOf(token, reference);
  const entries = history.entries ?? [];
  if (entries.length !== 2) fail(`history length ${entries.length}`);
  const created = entries[0];
  const moved = entries[1];
  if (created.event !== 'created' || created.seq !== 1) fail('history prefix is not the created entry');
  const createdFields = (created.changes ?? []).map((change) => change.field).join(',');
  if (createdFields !== 'table_ids,starts_at_local,party_size') fail(`created fields ${createdFields}`);
  if (JSON.stringify(created.changes?.[0]?.to) !== '["t_1","t_2"]') fail('created table set was not the pair');
  if (moved.event !== 'reassigned' || moved.seq !== 2 || moved.revision !== before.revision + 1 || moved.plan_id !== planId) {
    fail('reassigned entry did not match the plan');
  }
  if ((moved.changes ?? []).length !== 1 || moved.changes[0].field !== 'table_ids') fail('reassigned change was not table_ids');
  if (JSON.stringify(moved.changes[0].from) !== '["t_1","t_2"]' || JSON.stringify(moved.changes[0].to) !== '["t_3"]') {
    fail('reassigned from/to was not the pair then t_3');
  }
  if (JSON.stringify(created.accepted_terms) !== JSON.stringify(moved.accepted_terms)) fail('history terms diverged');
  return { current: receiptSummary(current.text), history: historyDigest(history) };
}

async function runRepair(browser) {
  await reset(destination, repairFixture());
  if ((await restaurantRevision()) !== 0) fail('restaurant revision did not start at 0');
  const managerLogin = await api(destination, '/auth/login', {
    method: 'POST',
    body: { email: 'mira.manager@example.com', password: SECRET },
  });
  if (managerLogin.status !== 200) fail(`manager login ${managerLogin.status} ${errorCode(managerLogin.text)}`);
  const manager = JSON.parse(managerLogin.text);
  if (manager.user_id !== 'u_mgr' || manager.display_name !== 'Mira') fail('manager identity differed');

  const state = freshState();
  state.captureReservation = true;
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true });
  await installRouter(page, state);

  let releaseCatalog;
  state.holdGate = new Promise((resolve) => {
    releaseCatalog = resolve;
  });
  state.hold = (url) => url.pathname === '/restaurants';
  await page.goto(`${destination}/`);
  await page.getByTestId('loading-state').waitFor();
  await captureQuartet(page, 'loading', 0);
  await runAxe(page, 'loading');
  state.hold = null;
  releaseCatalog();
  await page.getByTestId('restaurant-select').waitFor();
  await pass('R115-loading', 'loading-state is visible before the catalogue resolves');

  const optionValues = await page.getByTestId('restaurant-select').locator('option').evaluateAll((nodes) => nodes.map((node) => node.value));
  if (!optionValues.includes('r_anker') || !optionValues.includes('r_nord') || !optionValues.includes('r_hall')) {
    fail(`restaurant options ${optionValues.join(',')}`);
  }

  await knownScroll(page);
  await search(page, 'r_anker', sunday, 6);
  await waitSearch(page);
  if ((await page.getByTestId('date-input').inputValue()) !== sunday) fail('date input did not keep the civil date');
  if ((await page.getByTestId('no-slots').count()) !== 1 || (await page.getByTestId('availability-grid').count()) !== 0) {
    fail('closed day did not replace the grid');
  }
  await settledScroll(page);
  await assertRevealed(page, 'empty', '[data-testid="no-slots"]', 40);
  await captureQuartet(page, 'empty');
  await runAxe(page, 'empty');
  await pass('R144-empty', 'no-slots replaces the grid on a closed day');

  state.failAvailability = true;
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 6);
  await page.getByTestId('search-error').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'error', '[data-testid="search-error"]', 24);
  await captureQuartet(page, 'error');
  await runAxe(page, 'error');
  await pass('R115-error', 'search-error after a lost availability response');

  await knownScroll(page);
  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  if ((await page.getByTestId('party-size-input').inputValue()) !== '6') fail('party input was not the searched number');
  const beforeForm = await page.getByTestId('booking-form').count();
  await page.getByTestId('slot-t_1-19:00').click();
  await page.waitForTimeout(120);
  if ((await page.getByTestId('booking-form').count()) !== beforeForm) fail('unavailable cell opened a form');
  await page.getByTestId('slot-t_1+t_2-19:00').click();
  await page.getByTestId('auth-error').waitFor();
  if ((await page.getByTestId('booking-form').count()) !== 0) fail('signed-out click opened a form');
  await captureQuartet(page, 'auth-error');
  await pass('R148-R149', 'unavailable cell is inert and a signed-out pair asks for sign-in');

  await page.goto(`${destination}/login`);
  await page.getByTestId('login-email').fill('ada.s4g2@example.com');
  await page.getByTestId('login-password').fill('not-the-password');
  await page.getByTestId('login-submit').click();
  await page.getByTestId('auth-error').waitFor();
  await captureQuartet(page, 'auth-bad-login');
  await runAxe(page, 'auth-bad-login');
  if ((await page.getByTestId('current-user').count()) !== 0) fail('bad login signed in');
  await pass('R138', 'auth-error is present only after a refused login');

  await signupUi(page, destination, 'ada.s4g2@example.com', 'Ada');
  for (const path of ['/', '/lookup', '/login', '/signup']) {
    await page.goto(`${destination}${path}`);
    await assertSessionChrome(page, { signedIn: true, display: 'Ada' });
  }
  await page.goto(`${destination}/`);
  await page.getByTestId('restaurant-select').waitFor();
  await pass('R137-R139', 'signup hooks and Ada on every signed-in screen');

  await knownScroll(page);
  const signedGets = state.publicGets.length;
  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  await settledScroll(page);
  const gridReveal = await assertRevealed(page, 'party-6-grid', '.grid-caption', 24);
  const floorReveal = await readBox(page, '[data-testid="floor-plan"]');
  if (floorReveal.bottom <= 0 || gridReveal.top < 48) fail('search reveal hid the floor or pinned the grid');
  const shownClocks = await page.evaluate(() => [...document.querySelectorAll('[data-testid^="slot-t_1-"]')].map((node) => (node.getAttribute('data-testid') || '').slice('slot-t_1-'.length)));
  if (shownClocks.join(',') !== '18:00,18:30,19:00,19:30,20:00,20:30,21:00,21:30') fail(`grid clocks ${shownClocks.join(',')}`);
  const cellCount = await page.locator('[data-testid="availability-grid"] [data-testid^="slot-"]').count();
  if (cellCount !== 40) fail(`repair grid cells ${cellCount}`);
  const signedPublic = state.publicGets.slice(signedGets);
  if (!signedPublic.some((row) => row.path.startsWith('/availability') && row.hasAuthorization === false)) {
    fail('signed-in availability request carried a bearer');
  }
  if (signedPublic.some((row) => row.hasAuthorization)) fail('a public GET carried a bearer');
  if (signedPublic.some((row) => row.path.includes('explain='))) fail('the page requested explain');
  report.publicGets = signedPublic.map((row) => ({ path: row.path.split('?')[0], hasAuthorization: row.hasAuthorization }));

  const selected = await clickAndReadSelected(page, 'slot-t_1+t_2-19:00');
  if (selected.selected !== 'true') fail(`pair selection ${selected.selected} running ${selected.running}`);
  report.selection = { selected: selected.selected, runningAnimations: selected.running };
  await page.getByTestId('booking-form').waitFor();
  const summary = await page.getByTestId('booking-summary').innerText();
  if (!summary.includes('Table 1') || !summary.includes('Table 2') || !summary.includes('7:00 PM') || !summary.includes('19:00')) {
    fail(`summary ${summary}`);
  }
  if ((await page.getByTestId('booking-party-size').inputValue()) !== '6') fail('prefilled party was not 6');
  await settledScroll(page);
  await assertRevealed(page, 'pair-form', '[data-testid="booking-form"]', 80);
  await measurePaint(page, 'party-6-pair');
  await measureRenderedContrast(page, [
    { role: 'selected-pair', testId: 'slot-t_1+t_2-19:00', word: 'Held', available: 'true', selected: 'true' },
    { role: 'unavailable-single', testId: 'slot-t_1-19:00', word: 'Taken', available: 'false', selected: 'false' },
    { role: 'available-single', testId: 'slot-t_3-19:00', word: 'Free', available: 'true', selected: 'false' },
  ], 'party-6');
  await measureSettled(page, 'pair-selected-1280');
  await page.setViewportSize({ width: 375, height: 812 });
  await page.waitForTimeout(200);
  await measureSettled(page, 'pair-selected-375');
  await page.setViewportSize({ width: 1280, height: 900 });
  await captureQuartet(page, 'pair-selected', 200);
  await pass('R114-R188', 'pair selection is true immediately and the summary names both tables');

  const created = await bookFromForm(page, state);
  if (created.status !== 201) fail(`pair create ${created.status} ${errorCode(created.text)}`);
  const createdSummary = receiptSummary(created.text);
  const createdBody = JSON.parse(created.body);
  if (createdBody.table_id || createdBody.table_ids?.join('+') !== 't_1+t_2' || createdBody.party_size !== 6) {
    fail('pair request was not table_ids only');
  }
  if (createdSummary.revision !== 1 || createdSummary.policyVersion !== 0 || createdSummary.duration !== 90 || createdSummary.tableIds?.join('+') !== 't_1+t_2') {
    fail(`create receipt revision ${createdSummary.revision} policy ${createdSummary.policyVersion}`);
  }
  await page.getByTestId('confirmation-reference').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'confirmed', '[data-testid="confirmation"]', 60);
  const pairRef = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (pairRef !== createdSummary.reference) fail('confirmation was not the server reference');
  const tables = await page.getByTestId('confirmation-tables').innerText();
  const details = await page.getByTestId('confirmation-details').innerText();
  if (tables !== 'Table 1 and Table 2') fail(`confirmation tables ${tables}`);
  if (!details.includes('Zum Anker') || !details.includes('Table 1') || !details.includes('7:00 PM') || !details.includes('19:00')) {
    fail(`confirmation details ${details}`);
  }
  if ((await page.getByTestId('booking-form').count()) !== 1) fail('confirmation removed the form');
  const userId = await sessionUserId(page);
  const ownerMatch = await ownerMatches(pairRef, userId);
  if (!ownerMatch) fail('exported owner did not match the session');
  report.repair.ownerMatch = true;
  await pass('R155-R189', `live pair ${pairRef} confirmed on Table 1 and Table 2`);

  const diner = await sessionToken(page);
  const beforeRevision = await restaurantRevision();
  if (beforeRevision !== 1) fail(`revision after first booking ${beforeRevision}`);
  const preview = await previewClosure(tokenOf(manager), '2027-06-17T19:30:00+02:00', '2027-06-17T20:00:00+02:00');
  if (preview.status !== 201) fail(`preview ${preview.status} ${errorCode(preview.text)}`);
  const previewView = assignmentSummary(preview.text);
  if ((await restaurantRevision()) !== beforeRevision) fail('preview incremented the restaurant revision');
  if (previewView.restaurantRevision !== beforeRevision || previewView.movedCount !== 1 || previewView.unusedSeats !== 0) {
    fail(`preview counts revision ${previewView.restaurantRevision} moved ${previewView.movedCount} unused ${previewView.unusedSeats}`);
  }
  if (previewView.assignments?.length !== 1 || previewView.assignments[0].reference !== pairRef || previewView.assignments[0].changed !== true) {
    fail('preview assignment was not the pair booking marked changed');
  }
  if (JSON.stringify(previewView.assignments[0].tableIds) !== '["t_3"]') fail('preview did not assign t_3');
  const historyBefore = await historyOf(diner, pairRef);
  const applied = await applyPlan(tokenOf(manager), previewView.planId);
  if (applied.status !== 201) fail(`apply ${applied.status} ${errorCode(applied.text)}`);
  const appliedView = JSON.parse(applied.text);
  if (appliedView.restaurant_revision !== beforeRevision + 1 || appliedView.plan_id !== previewView.planId) {
    fail(`apply revision ${appliedView.restaurant_revision} expected ${beforeRevision + 1}`);
  }
  if ((await restaurantRevision()) !== beforeRevision + 1) fail('apply did not increment the restaurant revision once');
  const reservations = appliedView.reservations ?? [];
  if (reservations.length !== 1 || reservations[0].reference !== pairRef || JSON.stringify(reservations[0].table_ids) !== '["t_3"]') {
    fail('apply reservations were not the moved booking');
  }
  const moved = await assertRepairMove(diner, pairRef, previewView.planId, created.text);
  if (JSON.stringify(historyBefore.entries) !== JSON.stringify((await historyOf(diner, pairRef)).entries.slice(0, historyBefore.entries.length))) {
    fail('history prefix changed');
  }
  const confirmationAfter = await page.getByTestId('confirmation-tables').innerText();
  const referenceAfter = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (confirmationAfter !== 'Table 1 and Table 2' || referenceAfter !== pairRef) fail('confirmation changed after the repair');
  await captureQuartet(page, 'confirmed', 300);
  await runAxe(page, 'confirmed');
  report.repair.first = {
    reference: pairRef,
    previewRevision: previewView.restaurantRevision,
    applyRevision: appliedView.restaurant_revision,
    movedCount: previewView.movedCount,
    unusedSeats: previewView.unusedSeats,
    assigned: previewView.assignments[0].tableIds,
    current: moved.current,
    history: moved.history,
    receiptSha256: createdSummary.sha256,
  };
  await pass('R327-R346', `repair moved ${pairRef} to Table 3 and kept the original confirmation`);

  state.captureLookup = true;
  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await page.getByTestId('lookup-reference-input').fill(pairRef);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-detail').waitFor();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed') fail('repaired lookup status');
  const lookedTables = await page.getByTestId('reservation-tables').innerText();
  const lookedDetail = await page.getByTestId('reservation-detail').innerText();
  if (lookedTables !== 'Table 3') fail(`lookup tables ${lookedTables}`);
  if (!lookedDetail.includes('7:00 PM') || !lookedDetail.includes('19:00') || lookedDetail.includes('20:30') || lookedDetail.includes('Table 1')) {
    fail('lookup did not paint the original start on Table 3');
  }
  const looked = state.lookups.at(-1);
  const lookedSummary = looked ? receiptSummary(looked.text) : null;
  if (!looked || looked.status !== 200 || lookedSummary.tableIds?.join('+') !== 't_3' || lookedSummary.revision !== 2) {
    fail(`lookup json ${looked?.status ?? 'missing'} revision ${lookedSummary?.revision ?? 'none'}`);
  }
  if (elapsedMinutes(JSON.parse(looked.text).starts_at, JSON.parse(looked.text).ends_at) !== 90) fail('lookup record duration was not 90 minutes');
  await captureQuartet(page, 'lookup-repaired');
  await pass('R306-lookup', 'lookup renders Table 3 at 7:00 PM while the record end stays 90 minutes later');

  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Search' }).click();
  await page.getByTestId('restaurant-select').waitFor();
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  const live = await availabilityOf(6);
  const gridMismatches = await mirrorGrid(page, live);
  const floorMismatches = await mirrorFloor(page, live.slots[0]);
  if (gridMismatches.length || floorMismatches.length) fail(`mirror ${[...gridMismatches, ...floorMismatches].join('; ')}`);
  if (clockOf(live.slots[0].starts_at_local) !== '18:00') fail('floor was not compared to the first slot');
  const explainedLive = await explained(6);
  const byClock = Object.fromEntries((explainedLive.slots ?? []).map((slot) => [clockOf(slot.starts_at_local), slot]));
  const overlap = {
    '18:00': ruleHolds(byClock['18:00'], 't_2', 'no_overlap'),
    '18:30': ruleHolds(byClock['18:30'], 't_2', 'no_overlap'),
    '19:00': ruleHolds(byClock['19:00'], 't_2', 'no_overlap'),
    '20:00': ruleHolds(byClock['20:00'], 't_2', 'no_overlap'),
  };
  report.repair.overlap = overlap;
  if (overlap['18:00'] !== true || overlap['20:00'] !== true || overlap['18:30'] !== false || overlap['19:00'] !== false) {
    fail(`closure overlap ${JSON.stringify(overlap)}`);
  }
  const inert = await page.getByTestId('slot-t_2-19:00').getAttribute('data-available');
  const forms = await page.getByTestId('booking-form').count();
  await page.getByTestId('slot-t_2-19:00').click();
  await page.waitForTimeout(120);
  if (inert !== 'false' || (await page.getByTestId('booking-form').count()) !== forms) fail('closed cell was not inert');
  await pass('R306-R343', 'grid and floor mirror availability and the short closure blocks only overlapping slots');

  await page.getByTestId('slot-t_1+t_2-20:30').click();
  await page.getByTestId('booking-form').waitFor();
  await page.getByTestId('booking-party-size').fill('5');
  const rivalToken = await signup(destination, 'nia.s4g2@example.com', 'Nia');
  const rival = await api(destination, '/reservations', {
    method: 'POST',
    token: rivalToken,
    key: randomKey(),
    body: { restaurant_id: 'r_anker', table_ids: ['t_1', 't_2'], starts_at_local: `${thursday}T20:30`, party_size: 6 },
  });
  if (rival.status !== 201) fail(`rival create ${rival.status} ${errorCode(rival.text)}`);
  const rivalRef = JSON.parse(rival.text).reference;
  const partiesBefore = state.availabilityParties.length;
  const refused = await bookFromForm(page, state);
  if (refused.status !== 409 || errorCode(refused.text) !== 'table_unavailable') fail(`rival response ${refused.status} ${errorCode(refused.text)}`);
  await page.getByTestId('booking-error').waitFor();
  if ((await page.getByTestId('booking-party-size').inputValue()) !== '5') fail('conflict cleared the edited party');
  if ((await page.getByTestId('booking-form').count()) !== 1) fail('conflict closed the form');
  if ((await page.getByTestId('confirmation').count()) !== 0 || (await page.getByTestId('booking-uncertain').count()) !== 0) {
    fail('rival 409 showed confirmation or uncertainty');
  }
  await page.waitForFunction(
    () => document.querySelector('[data-testid="slot-t_1+t_2-20:30"]')?.getAttribute('data-available') === 'false',
    null,
    { timeout: 8000 },
  );
  const refreshParties = state.availabilityParties.slice(partiesBefore);
  if (!refreshParties.includes('6')) fail(`refresh parties ${refreshParties.join(',')}`);
  await captureQuartet(page, 'booking-error');
  await runAxe(page, 'refused');
  const cancelledRival = await api(destination, `/reservations/${encodeURIComponent(rivalRef)}/cancel`, {
    method: 'POST',
    token: rivalToken,
  });
  if (cancelledRival.status !== 200) fail(`rival cancel ${cancelledRival.status} ${errorCode(cancelledRival.text)}`);
  await pass('R127-R128-R129', 'rival 409 keeps the edited party and refreshes the searched party');

  const staleFrom = '2027-06-17T21:30:00+02:00';
  const staleTo = '2027-06-17T22:00:00+02:00';
  const stalePreview = await previewClosure(tokenOf(manager), staleFrom, staleTo);
  if (stalePreview.status !== 201) fail(`stale preview ${stalePreview.status} ${errorCode(stalePreview.text)}`);
  const staleView = assignmentSummary(stalePreview.text);
  if ((staleView.assignments ?? []).length !== 0 || staleView.movedCount !== 0) fail('pre-booking preview was not empty');
  const revisionAtStale = await restaurantRevision();
  if (staleView.restaurantRevision !== revisionAtStale) fail('stale preview revision drifted');
  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  if ((await page.getByTestId('slot-t_1+t_2-21:30').getAttribute('data-available')) !== 'true') fail('drop target was not free');
  await page.getByTestId('slot-t_1+t_2-21:30').click();
  await page.getByTestId('booking-summary').waitFor();
  if ((await page.getByTestId('booking-party-size').inputValue()) !== '6') fail('new selection did not restore the searched party');
  state.dropNextReservation = true;
  const droppedClick = page.getByTestId('booking-submit').click();
  await page.getByTestId('booking-uncertain').waitFor();
  await droppedClick;
  if (!state.dropped || state.dropped.status !== 201 || state.dropped.servedToBrowser !== false || state.dropped.method !== 'POST') {
    fail(`drop commit ${state.dropped?.status ?? 'missing'} ${errorCode(state.dropped?.text ?? '')}`);
  }
  const uncertainText = (await page.getByTestId('booking-uncertain').innerText()).trim();
  if (!uncertainText || (await page.getByTestId('booking-error').count()) !== 0 || (await page.getByTestId('confirmation').count()) !== 0) {
    fail('lost response was not uncertain alone');
  }
  await captureQuartet(page, 'uncertain', 200);
  await runAxe(page, 'uncertain');
  const droppedSummary = receiptSummary(state.dropped.text);
  const stored = await api(destination, '/reservations', {
    method: 'POST',
    token: diner,
    key: state.dropped.key,
    body: JSON.parse(state.dropped.body),
  });
  if (stored.status !== 200 || stored.text !== state.dropped.text) fail(`stored receipt ${stored.status}`);
  if ((await restaurantRevision()) !== revisionAtStale + 1) fail('withheld booking did not increment the restaurant revision');
  const staleApply = await applyPlan(tokenOf(manager), staleView.planId);
  if (staleApply.status !== 409 || errorCode(staleApply.text) !== 'stale_plan') {
    fail(`stale apply ${staleApply.status} ${errorCode(staleApply.text)}`);
  }
  if ((await restaurantRevision()) !== revisionAtStale + 1) fail('stale apply changed the restaurant revision');
  const stillPair = await api(destination, `/reservations/${encodeURIComponent(droppedSummary.reference)}`, { token: diner });
  const stillSummary = receiptSummary(stillPair.text);
  if (stillPair.status !== 200 || stillSummary.tableIds?.join('+') !== 't_1+t_2' || stillSummary.revision !== 1) {
    fail('stale apply changed the withheld booking');
  }
  const fresh = await previewClosure(tokenOf(manager), staleFrom, staleTo);
  if (fresh.status !== 201) fail(`fresh preview ${fresh.status} ${errorCode(fresh.text)}`);
  const freshView = assignmentSummary(fresh.text);
  if (freshView.restaurantRevision !== revisionAtStale + 1 || freshView.movedCount !== 1 || freshView.unusedSeats !== 0) {
    fail(`fresh preview revision ${freshView.restaurantRevision} moved ${freshView.movedCount}`);
  }
  if (freshView.assignments?.length !== 1 || freshView.assignments[0].reference !== droppedSummary.reference || JSON.stringify(freshView.assignments[0].tableIds) !== '["t_3"]' || freshView.assignments[0].changed !== true) {
    fail('fresh preview did not move the withheld booking to t_3');
  }
  const freshApply = await applyPlan(tokenOf(manager), freshView.planId);
  if (freshApply.status !== 201 || JSON.parse(freshApply.text).restaurant_revision !== revisionAtStale + 2) {
    fail(`fresh apply ${freshApply.status} ${errorCode(freshApply.text)}`);
  }
  const listBeforeRetry = await api(destination, '/reservations', { token: diner });
  const countBeforeRetry = JSON.parse(listBeforeRetry.text).reservations?.length ?? -1;
  const recovered = await bookFromForm(page, state);
  if (recovered.status !== 200 || recovered.method !== 'POST' || recovered.key !== state.dropped.key || recovered.cachedBodyReused !== false) {
    fail(`recovery ${recovered.status} ${recovered.method}`);
  }
  if (!sameJsonText(recovered.body, state.dropped.body) || recovered.text !== state.dropped.text) fail('recovery key, body, or receipt differed');
  if (!recovered.fetchedUrl.startsWith(destination) || new URL(recovered.fetchedUrl).pathname !== '/reservations') {
    fail('recovery was not a live reservation POST');
  }
  const listAfterRetry = await api(destination, '/reservations', { token: diner });
  const countAfterRetry = JSON.parse(listAfterRetry.text).reservations?.length ?? -1;
  if (countBeforeRetry !== countAfterRetry) fail(`retry changed the reservation count ${countBeforeRetry} -> ${countAfterRetry}`);
  await page.getByTestId('confirmation-reference').waitFor();
  const recoveredRef = (await page.getByTestId('confirmation-reference').innerText()).trim();
  const recoveredTables = await page.getByTestId('confirmation-tables').innerText();
  const recoveredDetails = await page.getByTestId('confirmation-details').innerText();
  if (recoveredRef !== droppedSummary.reference || recoveredTables !== 'Table 1 and Table 2') fail('retry confirmation was not the original pair receipt');
  if (!recoveredDetails.includes('9:30 PM') || !recoveredDetails.includes('21:30') || (await page.getByTestId('booking-uncertain').count()) !== 0) {
    fail('retry confirmation lost the original clock or left uncertainty');
  }
  await captureQuartet(page, 'uncertain-confirmed', 300);
  const repairedLate = await assertRepairMove(diner, droppedSummary.reference, freshView.planId, state.dropped.text);
  report.repair.uncertain = {
    reference: droppedSummary.reference,
    stalePlan: 'stale_plan',
    retryStatus: recovered.status,
    byteIdentical: true,
    cachedBodyReused: false,
    countBeforeRetry,
    countAfterRetry,
    current: repairedLate.current,
    history: repairedLate.history,
  };
  await pass('R130-R133', `lost commit recovered ${droppedSummary.reference} as the original pair receipt`);

  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await page.getByTestId('lookup-reference-input').fill(droppedSummary.reference);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-detail').waitFor();
  const lateTables = await page.getByTestId('reservation-tables').innerText();
  const lateDetail = await page.getByTestId('reservation-detail').innerText();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed' || lateTables !== 'Table 3') {
    fail(`late lookup ${lateTables}`);
  }
  if (!lateDetail.includes('9:30 PM') || !lateDetail.includes('21:30') || lateDetail.includes('Table 1') || lateDetail.includes('23:00')) {
    fail('late lookup did not paint the repaired table at the original start');
  }
  await captureQuartet(page, 'lookup-repaired-late');
  await pass('R306-current', 'lookup after the withheld repair shows Table 3 at 9:30 PM');

  await page.getByTestId('lookup-reference-input').fill(pairRef);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-cancel-button').waitFor();
  await page.getByTestId('reservation-cancel-button').click();
  await page.waitForFunction(() => document.querySelector('[data-testid="reservation-status"]')?.textContent?.trim() === 'cancelled');
  if ((await page.getByTestId('reservation-cancel-button').count()) !== 0) fail('cancel button remained');
  await captureQuartet(page, 'lookup-cancelled');
  await pass('R159', 'owner cancel removes the button and shows cancelled');

  const past = await api(destination, '/reservations', {
    method: 'POST',
    token: diner,
    key: randomKey(),
    body: { restaurant_id: 'r_anker', table_id: 't_1', starts_at_local: `${pastThursday}T18:00`, party_size: 2 },
  });
  if (past.status !== 201) fail(`past booking ${past.status} ${errorCode(past.text)}`);
  const pastRef = JSON.parse(past.text).reference;
  await page.getByTestId('lookup-reference-input').fill(pastRef);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-status').waitFor();
  await page.getByTestId('reservation-cancel-button').click();
  await page.getByTestId('reservation-error').waitFor();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed') fail('cutoff changed the status');
  if ((await page.getByTestId('reservation-cancel-button').count()) !== 1) fail('cutoff removed the button');
  await captureQuartet(page, 'lookup-cutoff');
  await pass('R160-cutoff', 'past booking stays confirmed when the accepted cutoff has passed');

  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Search' }).click();
  let releaseAnker;
  let ankerSeen;
  const ankerGate = new Promise((resolve) => {
    releaseAnker = resolve;
  });
  const ankerStarted = new Promise((resolve) => {
    ankerSeen = resolve;
  });
  state.holdGate = ankerGate;
  state.hold = (url) => {
    const hit = url.pathname === '/restaurants/r_anker' || url.searchParams.get('restaurant_id') === 'r_anker';
    if (hit) ankerSeen();
    return hit;
  };
  await search(page, 'r_anker', thursday, 2);
  await ankerStarted;
  await search(page, 'r_nord', thursday, 2);
  await page.getByTestId('slot-t_window-17:00').waitFor();
  if ((await page.getByTestId('slot-t_1-18:00').count()) !== 0) fail('search B showed search A early');
  const scrollBeforeLate = await settledScroll(page);
  state.hold = null;
  releaseAnker();
  await page.waitForTimeout(700);
  const scrollAfterLate = await settledScroll(page);
  if (Math.abs(scrollAfterLate - scrollBeforeLate) > 40) fail(`late search moved scroll ${scrollBeforeLate} -> ${scrollAfterLate}`);
  if ((await page.locator('h1.display').innerText()) !== 'Nordlicht') fail('late search replaced the heading');
  if ((await page.getByTestId('slot-t_1-18:00').count()) !== 0) fail('late search restored A');
  await pass('R125', 'the later search wins when the earlier response arrives last');

  await knownScroll(page);
  await search(page, 'r_hall', thursday, 2);
  await waitSearch(page);
  await settledScroll(page);
  await measurePaint(page, 'long-labels');
  await page.getByTestId('plan-t_garden').focus();
  await page.keyboard.press('Enter');
  await page.getByTestId('booking-form').waitFor();
  await page.getByTestId('plan-t_alcove+t_garden').focus();
  await page.keyboard.press('Enter');
  const keyboardSummary = await page.getByTestId('booking-summary').innerText();
  if (!keyboardSummary.includes('Window alcove') || !keyboardSummary.includes('Garden corner')) fail(`keyboard summary ${keyboardSummary}`);
  await captureQuartet(page, 'long-labels', 200);
  await pass('R111-R196', 'floor labels and keyboard pair selection use table names');

  const missingLabels = await page.evaluate(() => [...document.querySelectorAll('input, select, textarea')].filter((element) => {
    if (element.getAttribute('aria-label') || element.getAttribute('aria-labelledby')) return false;
    return !(element.id && document.querySelector(`label[for="${CSS.escape(element.id)}"]`));
  }).length);
  if (missingLabels !== 0) fail(`${missingLabels} controls have no label`);
  const focusStyle = await page.evaluate(() => {
    const button = document.querySelector('[data-testid="search-button"]');
    button.focus({ focusVisible: true });
    const style = getComputedStyle(button);
    return { outline: style.outlineStyle, width: style.outlineWidth };
  });
  if (focusStyle.outline === 'none' || focusStyle.width === '0px') fail('search control has no visible focus');
  await pass('R118', 'controls have labels and a visible focus ring');

  const forbidden = state.apiPaths.filter((item) => item.includes('/replans') || item.includes('/series') || item.includes('explain='));
  report.forbiddenUi = forbidden;
  if (forbidden.length) fail(`page called ${forbidden.join('; ')}`);
  await pass('R305', 'the diner page did not call replans, series, or explain');

  await page.getByTestId('logout-button').click();
  await assertSessionChrome(page, { signedIn: false });
  await pass('R140', 'logout clears the signed-in name');
  await finishVideo(context, page, 'main-desktop.webm');
}

function tokenOf(manager) {
  return manager.token;
}

async function runBlankName(browser) {
  const context = await browser.newContext({ viewport: { width: 375, height: 812 }, colorScheme: 'dark' });
  await context.addInitScript(() => localStorage.setItem('chaaya-theme', 'dark'));
  const page = await context.newPage();
  page.on('pageerror', (error) => report.pageErrors.push(String(error.message ?? error)));
  await page.goto(`${destination}/signup`);
  await page.getByTestId('signup-email').fill('blank.s4g2@example.com');
  await page.getByTestId('signup-password').fill(SECRET);
  await page.getByTestId('signup-display-name').fill('');
  await page.getByTestId('signup-submit').click();
  await page.getByTestId('logout-button').waitFor();
  for (const path of ['/', '/lookup', '/login', '/signup']) {
    await page.goto(`${destination}${path}`);
    await assertSessionChrome(page, { signedIn: true, display: '' });
  }
  await page.getByTestId('logout-button').click();
  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Sign in' }).waitFor();
  await assertSessionChrome(page, { signedIn: false });
  await context.close();
  await pass('R139-blank', 'a blank display name stays signed in until logout');
}

async function runPhone(browser) {
  await reset(destination, repairFixture());
  const state = freshState();
  state.captureReservation = true;
  const { context, page } = await launch(browser, { width: 375, height: 812, video: true });
  await installRouter(page, state);
  await signupUi(page, destination, 'phone.s4g2@example.com', 'Ada');
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  await settledScroll(page);
  await page.getByTestId('slot-t_1+t_2-21:30').click();
  await page.getByTestId('booking-form').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'phone-form', '[data-testid="booking-form"]', 80);
  const booked = await bookFromForm(page, state);
  if (booked.status !== 201) fail(`phone create ${booked.status} ${errorCode(booked.text)}`);
  await page.getByTestId('confirmation-reference').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'phone-confirmation', '[data-testid="confirmation"]', 40);
  await assertNoPageOverflow(page, 'phone-confirmed');
  await finishVideo(context, page, 'main-phone.webm');
  await pass('R117-phone', 'phone width books a pair without page overflow');
}

async function runReduced(browser) {
  await reset(destination, repairFixture());
  const state = freshState();
  state.captureReservation = true;
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true, reduced: true });
  await installRouter(page, state);
  await signupUi(page, destination, 'motion.s4g2@example.com', 'Ada');
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  const cell = page.getByTestId('slot-t_1+t_2-18:00');
  await cell.focus();
  await page.keyboard.press('Enter');
  const selected = await cell.getAttribute('data-selected');
  const running = await cell.evaluate((element) => element.getAnimations({ subtree: true }).filter((item) => item.playState === 'running').length);
  report.reducedSelection = { selected, runningAnimations: running };
  if (selected !== 'true' || running !== 0) fail(`reduced selection ${selected} running ${running}`);
  await page.getByTestId('booking-form').waitFor();
  await page.getByTestId('booking-submit').focus();
  await page.keyboard.press('Enter');
  await page.getByTestId('confirmation-reference').waitFor();
  const reference = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (!reference || (await page.getByTestId('booking-form').count()) !== 1) fail('reduced confirmation was not shown with the form');
  await finishVideo(context, page, 'main-reduced-motion.webm');
  await pass('R113-R114', `reduced motion selected immediately (${running} running) and confirmed ${reference}`);
}

async function runDensity(browser) {
  for (const step of [15, 5]) {
    const expectCells = step === 15 ? 75 : 215;
    const expectSlots = step === 15 ? 15 : 43;
    await reset(destination, denseRestaurant(step));
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, colorScheme: 'dark' });
    await context.addInitScript(() => localStorage.setItem('chaaya-theme', 'light'));
    const page = await context.newPage();
    page.on('pageerror', (error) => report.pageErrors.push(String(error.message ?? error)));
    await page.goto(`${destination}/`);
    await page.getByTestId('restaurant-select').waitFor();
    await search(page, 'r_dense', thursday, 6);
    await waitSearch(page);
    const cells = await page.locator('[data-testid="availability-grid"] .cell').count();
    const slots = await page.evaluate(() => document.querySelectorAll('.colhead').length);
    if (cells !== expectCells || slots !== expectSlots) fail(`dense ${step} cells ${cells} slots ${slots}`);
    for (const width of [1280, 375]) {
      await page.setViewportSize({ width, height: width === 375 ? 812 : 900 });
      await page.waitForTimeout(150);
      const rail = await page.evaluate(() => {
        const wrap = document.querySelector('.matrix-wrap');
        const matrix = document.querySelector('.matrix');
        const root = document.documentElement;
        const matrixStyle = matrix ? getComputedStyle(matrix) : null;
        return {
          pageOverflow: root.scrollWidth > root.clientWidth + 1,
          railOverflow: Boolean(wrap && wrap.scrollWidth > wrap.clientWidth + 1),
          wrapScroll: wrap ? wrap.scrollWidth : 0,
          wrapClient: wrap ? wrap.clientWidth : 0,
          matrixDisplay: matrixStyle ? matrixStyle.display : '',
          matrixDirection: matrixStyle ? matrixStyle.flexDirection : '',
        };
      });
      report.layout.push({ label: `dense-${step}-${width}`, cells, slots, ...rail });
      await log(`DENSE ${step} ${width} cells=${cells} slots=${slots} page=${rail.pageOverflow} rail=${rail.railOverflow}`);
      if (rail.pageOverflow) fail(`dense ${step} ${width} page overflow`);
      if (width === 1280 && !rail.railOverflow) fail(`dense ${step} desktop grid has no internal rail`);
      if (width === 375 && (rail.matrixDisplay !== 'flex' || rail.matrixDirection !== 'column')) {
        fail(`dense ${step} phone matrix ${rail.matrixDisplay}/${rail.matrixDirection}`);
      }
    }
    await context.close();
  }
  await pass('R117-dense', '15 slots/75 cells and 43 slots/215 cells stay inside the page');
}

async function runUpgrade(browser, { name, source, fixture, email, party, testId, kind }) {
  await reset(source, fixture);
  await reset(destination, { users: [], restaurants: [], reservations: [] });
  const state = freshState();
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true });
  await installRouter(page, state);
  await page.goto(`${source}/signup`);
  await page.getByTestId('signup-email').fill(email);
  await page.getByTestId('signup-password').fill(SECRET);
  await page.getByTestId('signup-display-name').fill('Ada');
  await page.getByTestId('signup-submit').click();
  await page.getByTestId('current-user').waitFor();
  await page.evaluate((doc) => {
    window.__s4g2Mark = 'alive';
    document.documentElement.dataset.s4g2Doc = doc;
  }, name);
  const before = await documentIdentity(page);
  await search(page, 'r_anker', thursday, party);
  await waitSearch(page);
  await page.getByTestId(testId).click();
  await page.getByTestId('booking-summary').waitFor();
  state.dropNextReservation = true;
  const submitted = page.getByTestId('booking-submit').click();
  await page.getByTestId('booking-uncertain').waitFor();
  await submitted;
  if (!state.dropped || state.dropped.status !== 201 || state.dropped.servedToBrowser !== false) {
    fail(`${name} source commit ${state.dropped?.status ?? 'missing'} ${errorCode(state.dropped?.text ?? '')}`);
  }
  if ((await page.getByTestId('booking-error').count()) !== 0 || (await page.getByTestId('confirmation').count()) !== 0) {
    fail(`${name} loss showed an error or a confirmation`);
  }
  const uncertainText = (await page.getByTestId('booking-uncertain').innerText()).trim();
  if (!uncertainText) fail(`${name} uncertainty was empty`);
  await captureQuartet(page, `${name}-uncertain`, 200);
  const committed = receiptSummary(state.dropped.text);
  const sent = JSON.parse(state.dropped.body);
  if (kind === 'stage1') {
    if (!committed.hasTableId || committed.hasTableIds || committed.hasRevision || committed.hasAcceptedTerms || committed.tableId !== 't_2') {
      fail(`${name} receipt was not a legacy singleton`);
    }
    if (sent.table_id !== 't_2' || sent.table_ids || sent.party_size !== party) fail(`${name} pending body changed`);
  } else if (kind === 'stage2') {
    if (committed.hasTableId || !committed.hasTableIds || committed.tableIds?.join('+') !== 't_1+t_2' || committed.hasRevision || committed.hasAcceptedTerms) {
      fail(`${name} receipt was not a legacy pair`);
    }
    if (sent.table_id || sent.table_ids?.join('+') !== 't_1+t_2' || sent.party_size !== party) fail(`${name} pending body changed`);
  } else if (kind === 'stage3') {
    if (committed.hasTableId || !committed.hasTableIds || committed.tableIds?.join('+') !== 't_1+t_2' || !committed.hasRevision || !committed.hasAcceptedTerms) {
      fail(`${name} receipt was not a modern pair`);
    }
    if (committed.revision !== 1 || committed.policyVersion !== 0 || committed.duration !== 90 || committed.termsHaveEffective) {
      fail(`${name} modern terms revision ${committed.revision} policy ${committed.policyVersion}`);
    }
    if (sent.table_id || sent.table_ids?.join('+') !== 't_1+t_2' || sent.party_size !== party) fail(`${name} pending body changed`);
  } else {
    fail(`${name} unknown kind`);
  }
  const exported = await fetch(`${source}/_test/export`);
  const exportBytes = Buffer.from(await exported.arrayBuffer());
  if (exported.status !== 200) fail(`${name} export ${exported.status}`);
  const exportJson = JSON.parse(exportBytes.toString('utf8'));
  if (exportJson.track !== 'tablekeeper' || exportJson.format_version !== 1 || !exportJson.state || typeof exportJson.state !== 'object') {
    fail(`${name} export envelope`);
  }
  const exportPath = join(privateDir, `${name}-export.json`);
  await writeFile(exportPath, exportBytes);
  await chmod(exportPath, 0o600);
  const readBack = await readFile(exportPath);
  const fileMode = (await stat(exportPath)).mode & 0o777;
  const dirMode = (await stat(privateDir)).mode & 0o777;
  if (!readBack.equals(exportBytes) || fileMode !== 0o600 || dirMode !== 0o700) {
    fail(`${name} private export mode file=${fileMode.toString(8)} dir=${dirMode.toString(8)}`);
  }
  const imported = await fetch(`${destination}/_test/import`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: exportBytes,
  });
  if (imported.status !== 204) fail(`${name} import ${imported.status}`);
  const midway = await documentIdentity(page);
  if (midway.mark !== 'alive' || midway.doc !== name || midway.nav !== before.nav || midway.href !== before.href) {
    fail(`${name} document changed before retry`);
  }
  state.forward = destination;
  state.captureReservation = true;
  await page.getByTestId('booking-submit').click();
  await page.getByTestId('confirmation-reference').waitFor();
  const retry = state.reservations.at(-1);
  if (!retry || retry.status !== 200 || retry.cachedBodyReused !== false || retry.method !== 'POST') fail(`${name} retry ${retry?.status ?? 'missing'}`);
  if (!retry.fetchedUrl.startsWith(destination)) fail(`${name} retry did not reach the destination`);
  if (retry.key !== state.dropped.key || !sameJsonText(retry.body, state.dropped.body) || retry.text !== state.dropped.text) {
    fail(`${name} retry key, body, or bytes differed`);
  }
  const shown = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (shown !== committed.reference) fail(`${name} confirmation was not the original reference`);
  if ((await page.getByTestId('booking-uncertain').count()) !== 0 || (await page.getByTestId('booking-error').count()) !== 0) {
    fail(`${name} recovery left an error state`);
  }
  await captureQuartet(page, `${name}-confirmed`, 300);
  const after = await documentIdentity(page);
  if (after.mark !== 'alive' || after.doc !== name || after.nav !== before.nav || new URL(page.url()).origin !== new URL(source).origin) {
    fail(`${name} retry left the original document`);
  }
  const nameText = (await page.getByTestId('current-user').innerText()).trim();
  if (!nameText.includes('Ada')) fail(`${name} display name was not retained`);
  const listed = await page.evaluate(async () => {
    const session = JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}');
    const response = await fetch('/reservations', {
      headers: { Accept: 'application/json', Authorization: `Bearer ${session.token}` },
    });
    const payload = await response.json();
    return {
      status: response.status,
      count: Array.isArray(payload.reservations) ? payload.reservations.length : -1,
      reference: payload.reservations?.[0]?.reference ?? '',
    };
  });
  if (listed.status !== 200 || listed.count !== 1 || listed.reference !== committed.reference) {
    fail(`${name} list ${listed.status} count ${listed.count}`);
  }
  const token = await sessionToken(page);
  const direct = await api(destination, `/reservations/${encodeURIComponent(committed.reference)}`, { token });
  if (direct.status !== 200) fail(`${name} destination lookup ${direct.status} ${errorCode(direct.text)}`);
  const directSummary = receiptSummary(direct.text);
  if (directSummary.reference !== committed.reference) fail(`${name} lookup reference differed`);
  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await page.getByTestId('lookup-reference-input').fill(committed.reference);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-detail').waitFor();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed') fail(`${name} lookup status`);
  const still = await documentIdentity(page);
  if (still.mark !== 'alive' || still.doc !== name || still.nav !== before.nav) fail(`${name} lookup reloaded the document`);
  await captureQuartet(page, `${name}-lookup`, 200);
  report.upgrades[name] = {
    kind,
    sourceCommit: state.dropped.status,
    browserServedCommit: false,
    exportStatus: exported.status,
    exportSha256: sha256(exportBytes),
    exportBytes: exportBytes.length,
    exportFileMode: fileMode.toString(8),
    privateDirMode: dirMode.toString(8),
    importStatus: imported.status,
    destinationRetry: retry.status,
    sourcePort: new URL(state.dropped.fetchedUrl).port,
    destinationPort: new URL(retry.fetchedUrl).port,
    documentHref: after.href,
    sameDocument: true,
    sameKey: true,
    sameBody: true,
    byteIdenticalReceipt: true,
    cachedBodyReused: false,
    receipt: committed,
    lookupStatus: direct.status,
    lookupReference: directSummary.reference,
    lookupHasRevision: directSummary.hasRevision,
    lookupHasTerms: directSummary.hasAcceptedTerms,
    oneRecord: listed.count === 1,
    displayNameRemained: true,
  };
  await pass(`R382-${kind}`, `${name} commit 201 lost, export 200, import 204, destination retry 200`);
  await finishVideo(context, page, `${name}-upgrade.webm`);
}

function publicContainer(info) {
  return {
    name: info.name,
    id: info.id,
    image: info.image,
    imageId: info.imageId,
    portEnv: info.portEnv,
    status: info.status,
    pid: info.pid,
    nanoCpus: info.nanoCpus,
    memory: info.memory,
  };
}

async function main() {
  await mkdir(shotDir, { recursive: true });
  await mkdir(videoDir, { recursive: true });
  await mkdir(privateDir, { recursive: true });
  await chmod(privateDir, 0o700);
  await writeFile(logPath, `COMMAND node scripts/s4-g2-live.mjs --stage1 ${stage1} --stage2 ${stage2} --stage3 ${stage3} --destination ${destination} --evidence ${evidence}\nCWD ${worktree}/stage-4/web\nSTARTED ${new Date().toISOString()}\nCHROME ${chrome}\n`);
  const days = {
    thursday: weekday(thursday),
    sunday: weekday(sunday),
    pastThursday: weekday(pastThursday),
  };
  if (days.thursday !== 'thu' || days.sunday !== 'sun' || days.pastThursday !== 'thu') fail(`weekdays ${JSON.stringify(days)}`);
  const head = execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  const stageTrees = {
    stage1: execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD:stage-1'], { encoding: 'utf8' }).trim(),
    stage2: execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD:stage-2'], { encoding: 'utf8' }).trim(),
    stage3: execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD:stage-3'], { encoding: 'utf8' }).trim(),
    stage4: execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD:stage-4'], { encoding: 'utf8' }).trim(),
  };
  const acceptedStage3 = execFileSync('git', ['-C', worktree, 'rev-parse', `${source3Provenance.acceptedCommit}:stage-3`], { encoding: 'utf8' }).trim();
  report.head = head;
  report.stageTrees = stageTrees;
  report.source3Provenance = {
    tag: source3Provenance.tag,
    imageId: source3Provenance.imageId,
    acceptedCommit: source3Provenance.acceptedCommit,
    acceptedTree: source3Provenance.acceptedTree,
    gitStage3AtAcceptedCommit: acceptedStage3,
    gitStage3AtHead: stageTrees.stage3,
    retainedPriorImage: source3Provenance.retainedPriorImage,
  };
  report.startedAt = new Date().toISOString();
  await log(`HEAD ${head}`);
  await log(`TREES ${stageTrees.stage1} ${stageTrees.stage2} ${stageTrees.stage3} ${stageTrees.stage4}`);
  await log(`SRC3-PROVENANCE tag=${source3Provenance.tag} image=${source3Provenance.imageId} accepted=${source3Provenance.acceptedCommit} tree=${acceptedStage3} head-tree=${stageTrees.stage3}`);
  if (acceptedStage3 !== source3Provenance.acceptedTree || stageTrees.stage3 !== source3Provenance.acceptedTree) {
    fail(`stage-3 tree head ${stageTrees.stage3} accepted ${acceptedStage3}`);
  }
  runContrastGate();
  await pass('R110', `${report.contrastGate.pairs} token pairs pass contrastGate`);

  const health = {
    stage1: await waitHealth(stage1),
    stage2: await waitHealth(stage2),
    stage3: await waitHealth(stage3),
    destination: await waitHealth(destination),
  };
  report.health = health;
  const stage1Port = hostPortOf(stage1);
  const stage2Port = hostPortOf(stage2);
  const stage3Port = hostPortOf(stage3);
  const destinationPort = hostPortOf(destination);
  const source1 = containerOnHostPort(stage1Port);
  const source2 = containerOnHostPort(stage2Port);
  const source3 = containerOnHostPort(stage3Port);
  const dest = containerOnHostPort(destinationPort);
  report.containers = {
    source1: publicContainer(source1),
    source2: publicContainer(source2),
    source3: publicContainer(source3),
    destination: publicContainer(dest),
  };
  await log(`SRC1 name=${source1.name} image=${source1.image} id=${source1.id} imageId=${source1.imageId} pid=${source1.pid} cpu=${source1.nanoCpus} mem=${source1.memory}`);
  await log(`SRC2 name=${source2.name} image=${source2.image} id=${source2.id} imageId=${source2.imageId} pid=${source2.pid} cpu=${source2.nanoCpus} mem=${source2.memory}`);
  await log(`SRC3 name=${source3.name} image=${source3.image} id=${source3.id} imageId=${source3.imageId} pid=${source3.pid} cpu=${source3.nanoCpus} mem=${source3.memory}`);
  await log(`DST name=${dest.name} image=${dest.image} id=${dest.id} imageId=${dest.imageId} pid=${dest.pid} cpu=${dest.nanoCpus} mem=${dest.memory}`);
  const ids = [source1.id, source2.id, source3.id, dest.id];
  const imageIds = [source1.imageId, source2.imageId, source3.imageId, dest.imageId];
  const pids = [source1.pid, source2.pid, source3.pid, dest.pid];
  const limited = [source1, source2, source3, dest].every((item) => item.nanoCpus === 2000000000 && item.memory === 2147483648);
  await expectCheck(
    'containers',
    source1.imageId === provenImages.src1 &&
      source2.imageId === provenImages.src2 &&
      source3.imageId === provenImages.src3 &&
      source3.image === source3Provenance.tag &&
      source3.imageId === source3Provenance.imageId &&
      source3.imageId !== source3Provenance.retainedPriorImage &&
      dest.image === 'tablekeeper:s4-g2a-stage4' &&
      source1.portEnv === `PORT=${stage1Port}` &&
      source2.portEnv === `PORT=${stage2Port}` &&
      source3.portEnv === `PORT=${stage3Port}` &&
      dest.portEnv === `PORT=${destinationPort}` &&
      new Set(ids).size === 4 &&
      new Set(imageIds).size === 4 &&
      new Set(pids).size === 4 &&
      pids.every((pid) => pid > 0) &&
      limited &&
      source1.name.endsWith('tk-s4-g2a-src1') &&
      source2.name.endsWith('tk-s4-g2a-src2') &&
      source3.name.endsWith('tk-s4-g2a-src3') &&
      dest.name.endsWith('tk-s4-g2a-dst'),
    `four images on ${stage1Port}/${stage2Port}/${stage3Port}/${destinationPort}`,
  );

  const browser = await chromium.launch({ executablePath: chrome, headless: true });
  try {
    await runRepair(browser);
    await runBlankName(browser);
    await runReduced(browser);
    await runPhone(browser);
    await runDensity(browser);
    await runUpgrade(browser, {
      name: 'stage1',
      source: stage1,
      fixture: stage1Fixture(),
      email: 'ada.stage1@example.com',
      party: 2,
      testId: 'slot-t_2-19:00',
      kind: 'stage1',
    });
    await runUpgrade(browser, {
      name: 'stage2',
      source: stage2,
      fixture: stage2Fixture(),
      email: 'ada.stage2@example.com',
      party: 6,
      testId: 'slot-t_1+t_2-19:00',
      kind: 'stage2',
    });
    await runUpgrade(browser, {
      name: 'stage3',
      source: stage3,
      fixture: stage2Fixture(),
      email: 'ada.stage3@example.com',
      party: 6,
      testId: 'slot-t_1+t_2-19:00',
      kind: 'stage3',
    });
    const pairRows = report.renderedContrast.filter((row) => row.role === 'selected-pair' && row.word === 'Held');
    for (const row of pairRows) {
      const expected = row.theme === 'light' ? 7.611 : row.theme === 'dark' ? 9.915 : null;
      if (expected == null) continue;
      if (Math.abs(row.ratio - expected) > 0.02) fail(`${row.role} ${row.width} ${row.theme} ${row.ratio} expected ${expected}`);
    }
    if (pairRows.filter((row) => row.theme === 'light' || row.theme === 'dark').length < 4) fail('selected pair contrast samples were incomplete');
    const words = new Set(report.renderedContrast.map((row) => row.word));
    if (!words.has('Free') || !words.has('Held') || !words.has('Taken')) fail(`state words ${[...words].join(',')}`);
    await pass('R118-contrast', 'selected pair contrast is 7.611 in light and 9.915 in dark');
  } finally {
    await browser.close();
  }

  const noisy = report.pageErrors.filter((item) => !/Failed to fetch|NetworkError|aborted|favicon/i.test(item));
  const noisyConsole = report.consoleErrors.filter((item) => !/ERR_FAILED|Failed to fetch|net::|favicon|status of 401|status of 409|status of 422/i.test(item));
  report.unexpectedPageErrors = noisy;
  report.unexpectedConsole = noisyConsole;
  if (report.offOrigin.length) failures.push(`off-origin ${report.offOrigin.length}`);
  if (noisy.length) failures.push(`pageerror ${noisy.length}`);
  if (noisyConsole.length) failures.push(`console ${noisyConsole.length}`);
  for (const name of await readdir(videoDir).catch(() => [])) {
    if (name.startsWith('page@')) await unlink(join(videoDir, name)).catch(() => undefined);
  }
  report.checkCount = Object.keys(report.checks).length;
  report.shotCount = report.shots.length;
  report.videoCount = report.videos.length;
  report.endedAt = new Date().toISOString();
  const serialized = JSON.stringify(report, null, 2);
  if (serialized.includes(SECRET) || serialized.includes('Bearer ') || serialized.includes('Idempotency-Key')) {
    await log('FAIL public report contained a secret');
    process.exit(1);
  }
  if (failures.length) {
    await log(`FAIL ${failures.join(' | ')}`);
    report.result = 'fail';
    report.failures = failures;
    await writeFile(join(evidence, 'report.json'), JSON.stringify(report, null, 2));
    process.exit(1);
  }
  report.result = 'pass';
  await writeFile(join(evidence, 'report.json'), JSON.stringify(report, null, 2));
  await log(`PASS probe checks=${report.checkCount} shots=${report.shotCount} videos=${report.videoCount} ${report.endedAt}`);
}

const isMain = process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href;
if (isMain) {
  main().catch(async (error) => {
    const message = error instanceof Error ? error.message : String(error);
    await log(`FAIL ${message}`).catch(() => undefined);
    report.result = 'fail';
    report.failures = [...failures, message];
    report.endedAt = new Date().toISOString();
    await mkdir(evidence || '/tmp', { recursive: true }).catch(() => undefined);
    const serialized = JSON.stringify(report, null, 2);
    if (!serialized.includes(SECRET) && !serialized.includes('Bearer ')) {
      await writeFile(join(evidence, 'report.json'), serialized).catch(() => undefined);
    }
    process.exit(1);
  });
}
