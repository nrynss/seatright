/**
 * S3-G2A live probe.
 *
 * Talks to the real stage-1 image, the real stage-2 image, and the current
 * stage-3 image. Policy setup is HTTP against the stage-3 process. The two
 * upgrades export the same source that just committed, import those bytes,
 * and retry in the original document. Exits nonzero on failure.
 *
 * Does not print tokens, passwords, idempotency keys, request bodies, or
 * export JSON. Reveal checks never call scrollIntoView.
 *
 *   node scripts/s3-g2-live.mjs --stage1 URL --stage2 URL --destination URL --evidence DIR
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
const axePath = join(worktree, 'stage-3/web/node_modules/axe-core/axe.min.js');
const SECRET = 'correct horse';
const thursday = '2027-06-17';
const laterThursday = '2027-06-24';
const sunday = '2027-06-13';
const pastThursday = '2026-09-24';
const nearThursday = '2026-10-08';
const policyFrom = '2026-10-01';

const args = process.argv.slice(2);
function arg(name) {
  const index = args.indexOf(`--${name}`);
  return index >= 0 ? args[index + 1] : '';
}

const stage1 = arg('stage1');
const stage2 = arg('stage2');
const destination = arg('destination');
const evidence = arg('evidence');

if (!stage1 || !stage2 || !destination || !evidence) {
  console.error('usage: node scripts/s3-g2-live.mjs --stage1 URL --stage2 URL --destination URL --evidence DIR');
  process.exit(2);
}

const shotDir = join(evidence, 'shots');
const videoDir = join(evidence, 'video');
const privateDir = join(evidence, 'private');
const logPath = join(evidence, 'probe.log');
const report = {
  item: 'S3-G2A',
  stage1,
  stage2,
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
    bytes: Buffer.byteLength(text),
    sha256: sha256(text),
  };
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

function policyFixture() {
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

function policyBody(effective, cutoff, duration, capacities) {
  return {
    effective_from: effective,
    slot_minutes: 60,
    reservation_duration_minutes: duration,
    cancellation_cutoff_minutes: cutoff,
    opening_hours: hours('thu', '18:00', '23:00'),
    capacities,
  };
}

const baseCaps = { t_1: 4, t_2: 6, t_3: 4 };
const laterCaps = { t_1: 3, t_2: 7, t_3: 4 };

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
    restaurants: [
      anker({ combinable: [['t_1', 't_2']] }),
    ],
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
        const body = await response.json();
        if (body.status !== 'ok') fail(`health body status ${body.status}`);
        return;
      }
    } catch (error) {
      last = error instanceof Error ? error.name : 'error';
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  fail(`health not ready (${last})`);
}

function randomKey() {
  return [...crypto.getRandomValues(new Uint8Array(16))].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

async function api(base, path, { method = 'GET', token = '', key = '', body = undefined } = {}) {
  const headers = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json; charset=utf-8';
  if (token) headers.Authorization = `Bearer ${token}`;
  if (key) headers['Idempotency-Key'] = key;
  const response = await fetch(`${base}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
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

async function publish(token, body) {
  const result = await api(destination, '/restaurants/r_anker/policies', {
    method: 'POST',
    token,
    key: randomKey(),
    body,
  });
  if (result.status !== 201) fail(`policy ${result.status} ${errorCode(result.text)}`);
  const parsed = JSON.parse(result.text);
  return {
    version: parsed.policy_version,
    slot: parsed.slot_minutes,
    duration: parsed.reservation_duration_minutes,
    cutoff: parsed.cancellation_cutoff_minutes,
  };
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
  };
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
  report.axe.push({ label, violations, titles, dead });
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
    mark: window.__s3g2Mark ?? '',
    doc: document.documentElement.dataset.s3g2Doc ?? '',
    nav: performance.getEntriesByType('navigation').length,
    href: location.href,
  }));
}

async function lookupSummary(page, reference) {
  return page.evaluate(async (ref) => {
    const session = JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}');
    const response = await fetch(`/reservations/${encodeURIComponent(ref)}`, {
      headers: { Accept: 'application/json', Authorization: `Bearer ${session.token}` },
    });
    const payload = await response.json();
    const terms = payload.accepted_terms ?? null;
    return {
      status: response.status,
      code: payload.error?.code ?? '',
      reference: payload.reference ?? '',
      revision: payload.revision ?? null,
      hasTerms: Boolean(terms && typeof terms === 'object'),
      hasEffective: Boolean(terms && Object.prototype.hasOwnProperty.call(terms, 'effective_from')),
      policyVersion: terms?.policy_version ?? null,
      slot: terms?.slot_minutes ?? null,
      duration: terms?.reservation_duration_minutes ?? null,
      cutoff: terms?.cancellation_cutoff_minutes ?? null,
      capacities: terms?.capacities ?? null,
      tableIds: payload.table_ids ?? null,
      tableId: payload.table_id ?? null,
    };
  }, reference);
}

async function patchStart(page, reference, when) {
  return page.evaluate(async ({ ref, starts }) => {
    const session = JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}');
    const response = await fetch(`/reservations/${encodeURIComponent(ref)}`, {
      method: 'PATCH',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json; charset=utf-8',
        Authorization: `Bearer ${session.token}`,
      },
      body: JSON.stringify({ starts_at_local: starts }),
    });
    const payload = await response.json();
    const terms = payload.accepted_terms ?? null;
    return {
      status: response.status,
      code: payload.error?.code ?? '',
      reference: payload.reference ?? '',
      revision: payload.revision ?? null,
      hasEffective: Boolean(terms && Object.prototype.hasOwnProperty.call(terms, 'effective_from')),
      policyVersion: terms?.policy_version ?? null,
      slot: terms?.slot_minutes ?? null,
      duration: terms?.reservation_duration_minutes ?? null,
      cutoff: terms?.cancellation_cutoff_minutes ?? null,
      capacities: terms?.capacities ?? null,
    };
  }, { ref: reference, starts: when });
}

async function createOwned(page, body) {
  return page.evaluate(async (payload) => {
    const session = JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}');
    const response = await fetch('/reservations', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json; charset=utf-8',
        Authorization: `Bearer ${session.token}`,
        'Idempotency-Key': crypto.randomUUID(),
      },
      body: JSON.stringify(payload),
    });
    const parsed = await response.json();
    return { status: response.status, reference: parsed.reference ?? '', code: parsed.error?.code ?? '' };
  }, body);
}

async function runDestination(browser) {
  await reset(destination, policyFixture());
  const managerLogin = await api(destination, '/auth/login', {
    method: 'POST',
    body: { email: 'mira.manager@example.com', password: SECRET },
  });
  if (managerLogin.status !== 200) fail(`manager login ${managerLogin.status} ${errorCode(managerLogin.text)}`);
  const manager = JSON.parse(managerLogin.text);
  if (manager.user_id !== 'u_mgr') fail(`manager id ${manager.user_id}`);
  const first = await publish(manager.token, policyBody(policyFrom, 60, 60, baseCaps));
  if (first.version !== 1 || first.slot !== 60 || first.duration !== 60 || first.cutoff !== 60) {
    fail(`first policy ${first.version} ${first.slot}/${first.duration}/${first.cutoff}`);
  }
  const policies = await api(destination, '/restaurants/r_anker/policies');
  const policyList = JSON.parse(policies.text);
  if (policies.status !== 200 || policyList.policies?.length !== 1 || policyList.policies[0].policy_version !== 1) {
    fail(`public policies ${policies.status} count ${policyList.policies?.length ?? 'none'}`);
  }
  const detail = JSON.parse((await api(destination, '/restaurants/r_anker')).text);
  const caps = (detail.tables ?? []).map((table) => table.capacity).join(',');
  if (detail.slot_minutes !== 30 || detail.reservation_duration_minutes !== 90 || caps !== '2,4,4') {
    fail(`detail stayed ${detail.slot_minutes}/${detail.reservation_duration_minutes}/${caps}`);
  }
  await pass('R240', 'restaurant detail remains 30/90 and capacities 2,4,4 after publication');

  const live = JSON.parse((await api(destination, `/availability?restaurant_id=r_anker&date=${thursday}&party_size=8`)).text);
  const clocks = (live.slots ?? []).map((slot) => clockOf(slot.starts_at_local));
  if (clocks.join(',') !== '18:00,19:00,20:00,21:00,22:00') fail(`hourly clocks ${clocks.join(',')}`);
  for (const slot of live.slots) {
    if ((slot.available_table_ids ?? []).length !== 0) fail('party 8 listed a single table');
    const pair = (slot.available_options ?? []).find((option) => (option.table_ids ?? []).join('+') === 't_1+t_2');
    if (!pair || pair.capacity !== 10) fail('party 8 pair capacity was not 10');
  }
  await pass('R241', 'published date uses an hourly grid and pair capacity 10');

  await runReduced(browser);
  await runPhone(browser);

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

  await knownScroll(page);
  await search(page, 'r_anker', sunday, 8);
  await waitSearch(page);
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
  await search(page, 'r_anker', thursday, 8);
  await page.getByTestId('search-error').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'error', '[data-testid="search-error"]', 24);
  await captureQuartet(page, 'error');
  await runAxe(page, 'error');
  await pass('R115-error', 'search-error after a lost availability response');

  await knownScroll(page);
  await search(page, 'r_anker', thursday, 8);
  await waitSearch(page);
  if ((await page.getByTestId('slot-t_1-19:00').getAttribute('data-available')) !== 'false') fail('party 8 single looked available');
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
  await page.getByTestId('login-email').fill('ada.s3g2@example.com');
  await page.getByTestId('login-password').fill('not-the-password');
  await page.getByTestId('login-submit').click();
  await page.getByTestId('auth-error').waitFor();
  await captureQuartet(page, 'auth-bad-login');
  await runAxe(page, 'auth-bad-login');
  if ((await page.getByTestId('current-user').count()) !== 0) fail('bad login signed in');
  await pass('R138', 'auth-error is present only after a refused login');

  await signupUi(page, destination, 'ada.s3g2@example.com', 'Ada');
  for (const path of ['/', '/lookup', '/login', '/signup']) {
    await page.goto(`${destination}${path}`);
    await assertSessionChrome(page, { signedIn: true, display: 'Ada' });
  }
  await page.goto(`${destination}/`);
  await page.getByTestId('restaurant-select').waitFor();
  await pass('R137-R139', 'signup hooks and Ada on every signed-in screen');

  await knownScroll(page);
  const signedGets = state.publicGets.length;
  await search(page, 'r_anker', thursday, 8);
  await waitSearch(page);
  await settledScroll(page);
  const gridReveal = await assertRevealed(page, 'party-8-grid', '.grid-caption', 24);
  const floorReveal = await readBox(page, '[data-testid="floor-plan"]');
  if (floorReveal.bottom <= 0 || gridReveal.top < 48) fail('search reveal hid the floor or pinned the grid');
  const heads = await page.locator('.rowhead .seats').allInnerTexts();
  if (!heads.includes('2 seats') || !heads.includes('4 seats') || !heads.includes('10 seats together')) {
    fail(`row heads ${heads.join('|')}`);
  }
  const shownClocks = await page.evaluate(() => [...document.querySelectorAll('[data-testid^="slot-t_1-"]')].map((node) => (node.getAttribute('data-testid') || '').slice('slot-t_1-'.length)));
  if (shownClocks.join(',') !== '18:00,19:00,20:00,21:00,22:00') fail(`grid clocks ${shownClocks.join(',')}`);
  if ((await page.getByTestId('slot-t_2-19:00').getAttribute('data-available')) !== 'false') fail('single stayed available');
  if ((await page.getByTestId('slot-t_1+t_2-19:00').getAttribute('data-available')) !== 'true') fail('pair was not selectable');
  const signedPublic = state.publicGets.slice(signedGets);
  if (!signedPublic.some((row) => row.path.startsWith('/availability') && row.hasAuthorization === false)) {
    fail('signed-in availability request carried a bearer');
  }
  if (signedPublic.some((row) => row.hasAuthorization)) fail('a public GET carried a bearer');
  report.publicGets = signedPublic.map((row) => ({ path: row.path.split('?')[0], hasAuthorization: row.hasAuthorization }));
  await pass('R42-R146-R187', 'party 8 grid follows the returned hourly options and public GETs have no bearer');

  const selected = await clickAndReadSelected(page, 'slot-t_1+t_2-19:00');
  if (selected.selected !== 'true') fail(`pair selection ${selected.selected} running ${selected.running}`);
  report.selection = { selected: selected.selected, runningAnimations: selected.running };
  await page.getByTestId('booking-form').waitFor();
  const summary = await page.getByTestId('booking-summary').innerText();
  if (!summary.includes('Table 1') || !summary.includes('Table 2') || !summary.includes('19:00')) fail(`summary ${summary}`);
  const partyValue = await page.getByTestId('booking-party-size').inputValue();
  if (partyValue !== '8') fail(`prefilled party ${partyValue}`);
  await settledScroll(page);
  await assertRevealed(page, 'pair-form', '[data-testid="booking-form"]', 80);
  await measurePaint(page, 'party-8-pair');
  await measureRenderedContrast(page, [
    { role: 'selected-pair', testId: 'slot-t_1+t_2-19:00', word: 'Held', available: 'true', selected: 'true' },
    { role: 'unavailable-single', testId: 'slot-t_1-19:00', word: 'Taken', available: 'false', selected: 'false' },
  ], 'party-8');
  await measureSettled(page, 'pair-selected-1280');
  await page.setViewportSize({ width: 375, height: 812 });
  await page.waitForTimeout(200);
  await measureSettled(page, 'pair-selected-375');
  await page.setViewportSize({ width: 1280, height: 900 });
  await pass('R114-R188', 'pair selection is true immediately and the summary names both tables');

  const created = await bookFromForm(page, state);
  if (created.status !== 201) fail(`pair create ${created.status} ${errorCode(created.text)}`);
  const createdSummary = receiptSummary(created.text);
  const createdBody = JSON.parse(created.body);
  if (createdBody.table_id || createdBody.table_ids?.join('+') !== 't_1+t_2' || createdBody.party_size !== 8) {
    fail('pair request was not table_ids only');
  }
  if (createdSummary.revision !== 1 || createdSummary.policyVersion !== 1 || createdSummary.slot !== 60 || createdSummary.duration !== 60) {
    fail(`create terms revision ${createdSummary.revision} policy ${createdSummary.policyVersion} ${createdSummary.slot}/${createdSummary.duration}`);
  }
  if (createdSummary.termsHaveEffective || createdSummary.capacities?.t_1 !== 4 || createdSummary.capacities?.t_2 !== 6 || createdSummary.capacities?.t_3 !== 4) {
    fail('create terms were not the selected policy snapshot');
  }
  await page.getByTestId('confirmation-reference').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'confirmed', '[data-testid="confirmation"]', 60);
  const pairRef = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (pairRef !== createdSummary.reference) fail('confirmation was not the server reference');
  const tables = await page.getByTestId('confirmation-tables').innerText();
  const details = await page.getByTestId('confirmation-details').innerText();
  if (!tables.includes('Table 1') || !tables.includes('Table 2')) fail(`confirmation tables ${tables}`);
  if (!details.includes('Zum Anker') || !details.includes('19:00')) fail(`confirmation details ${details}`);
  if ((await page.getByTestId('booking-form').count()) !== 1) fail('confirmation removed the form');
  await captureQuartet(page, 'confirmed', 500);
  await runAxe(page, 'confirmed');
  await pass('R155-R189-R242', `live pair ${pairRef} revision 1 terms 60/60 capacities 4,6,4`);

  const replay = await bookFromForm(page, state);
  if (replay.status !== 200 || replay.key !== created.key || !sameJsonText(replay.body, created.body) || !sameJsonText(replay.text, created.text)) {
    fail(`unchanged replay ${replay.status}`);
  }
  if ((await page.getByTestId('confirmation-reference').innerText()).trim() !== pairRef) fail('replay changed the reference');
  await pass('R153-R245', 'unchanged resubmit reused the key and original receipt');

  await page.getByTestId('booking-party-size').fill('9');
  const changed = await bookFromForm(page, state);
  if (changed.key === created.key || JSON.parse(changed.body).party_size !== 9) fail('changed party reused the key or body');
  if (changed.status !== 409 || errorCode(changed.text) !== 'table_unavailable') fail(`changed party ${changed.status} ${errorCode(changed.text)}`);
  await page.getByTestId('booking-error').waitFor();
  if ((await page.getByTestId('confirmation').count()) !== 0) fail('occupied change showed a confirmation');
  if ((await page.getByTestId('booking-uncertain').count()) !== 0) fail('occupied change showed uncertainty');
  await pass('R154-R126', 'changed party used a new key and was refused 409');

  const rivalSelect = await clickAndReadSelected(page, 'slot-t_1+t_2-21:00');
  if (rivalSelect.selected !== 'true') fail('rival target was not selectable');
  await page.getByTestId('booking-party-size').fill('9');
  const rivalToken = await signup(destination, 'nia.s3g2@example.com', 'Nia');
  const rival = await api(destination, '/reservations', {
    method: 'POST',
    token: rivalToken,
    key: randomKey(),
    body: { restaurant_id: 'r_anker', table_ids: ['t_1', 't_2'], starts_at_local: `${thursday}T21:00`, party_size: 8 },
  });
  if (rival.status !== 201) fail(`rival create ${rival.status} ${errorCode(rival.text)}`);
  const partiesBefore = state.availabilityParties.length;
  const refused = await bookFromForm(page, state);
  if (refused.status !== 409 || errorCode(refused.text) !== 'table_unavailable') fail(`rival response ${refused.status} ${errorCode(refused.text)}`);
  await page.getByTestId('booking-error').waitFor();
  if ((await page.getByTestId('booking-party-size').inputValue()) !== '9') fail('conflict cleared the edited party');
  if ((await page.getByTestId('booking-form').count()) !== 1) fail('conflict closed the form');
  if ((await page.getByTestId('confirmation').count()) !== 0 || (await page.getByTestId('booking-uncertain').count()) !== 0) {
    fail('rival 409 showed confirmation or uncertainty');
  }
  await page.waitForFunction(
    () => document.querySelector('[data-testid="slot-t_1+t_2-21:00"]')?.getAttribute('data-available') === 'false',
    null,
    { timeout: 8000 },
  );
  const refreshParties = state.availabilityParties.slice(partiesBefore);
  if (!refreshParties.includes('8')) fail(`refresh parties ${refreshParties.join(',')}`);
  await captureQuartet(page, 'booking-error');
  await runAxe(page, 'refused');
  await pass('R127-R128-R129', 'rival 409 keeps the edited party and refreshes the searched party');

  if ((await page.getByTestId('slot-t_1+t_2-20:00').getAttribute('data-available')) !== 'true') fail('drop target was not free');
  await page.getByTestId('slot-t_1+t_2-20:00').click();
  await page.getByTestId('booking-summary').waitFor();
  state.dropNextReservation = true;
  const droppedClick = page.getByTestId('booking-submit').click();
  await page.getByTestId('booking-uncertain').waitFor();
  await droppedClick;
  if (!state.dropped || state.dropped.status !== 201 || state.dropped.servedToBrowser !== false) {
    fail(`drop commit ${state.dropped?.status ?? 'missing'} ${errorCode(state.dropped?.text ?? '')}`);
  }
  const uncertainText = (await page.getByTestId('booking-uncertain').innerText()).trim();
  if (!uncertainText || (await page.getByTestId('booking-error').count()) !== 0 || (await page.getByTestId('confirmation').count()) !== 0) {
    fail('lost response was not uncertain alone');
  }
  await captureQuartet(page, 'uncertain', 200);
  await runAxe(page, 'uncertain');
  const droppedSummary = receiptSummary(state.dropped.text);
  const recovered = await bookFromForm(page, state);
  if (recovered.status !== 200 || recovered.key !== state.dropped.key || recovered.cachedBodyReused !== false) {
    fail(`recovery ${recovered.status}`);
  }
  if (!sameJsonText(recovered.body, state.dropped.body) || recovered.text !== state.dropped.text) fail('recovery key, body, or receipt differed');
  await page.getByTestId('confirmation-reference').waitFor();
  if ((await page.getByTestId('confirmation-reference').innerText()).trim() !== droppedSummary.reference) fail('recovery showed another reference');
  if ((await page.getByTestId('booking-uncertain').count()) !== 0) fail('recovery left uncertainty');
  await pass('R130-R133-R135', `lost commit recovered ${droppedSummary.reference}`);

  const later = await publish(manager.token, policyBody(laterThursday, 60, 120, laterCaps));
  if (later.version !== 2 || later.duration !== 120) fail(`later policy ${later.version} ${later.duration}`);
  const amended = await patchStart(page, pairRef, `${laterThursday}T19:00`);
  if (amended.status !== 200 || amended.reference !== pairRef || amended.revision !== 2) {
    fail(`amendment ${amended.status} ${amended.code} revision ${amended.revision}`);
  }
  if (amended.policyVersion !== 2 || amended.duration !== 120 || amended.capacities?.t_1 !== 3 || amended.capacities?.t_2 !== 7 || amended.hasEffective) {
    fail(`amended terms policy ${amended.policyVersion} duration ${amended.duration}`);
  }
  state.captureLookup = true;
  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await page.getByTestId('lookup-reference-input').fill(pairRef);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-detail').waitFor();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed') fail('amended lookup status');
  const lookedTables = await page.getByTestId('reservation-tables').innerText();
  if (!lookedTables.includes('Table 1') || !lookedTables.includes('Table 2')) fail(`lookup tables ${lookedTables}`);
  const looked = state.lookups.at(-1);
  const lookedSummary = looked ? receiptSummary(looked.text) : null;
  if (!looked || looked.status !== 200 || lookedSummary.revision !== 2 || lookedSummary.duration !== 120 || lookedSummary.policyVersion !== 2) {
    fail(`lookup json ${looked?.status ?? 'missing'} revision ${lookedSummary?.revision ?? 'none'}`);
  }
  await captureQuartet(page, 'lookup-confirmed');
  await pass('R157-R190-R249', `lookup after amendment shows revision 2 and the new terms`);

  await page.getByTestId('reservation-cancel-button').click();
  await page.waitForFunction(() => document.querySelector('[data-testid="reservation-status"]')?.textContent?.trim() === 'cancelled');
  if ((await page.getByTestId('reservation-cancel-button').count()) !== 0) fail('cancel button remained');
  await captureQuartet(page, 'lookup-cancelled');
  await pass('R159-R252', 'owner cancel removes the button and shows cancelled');

  const past = await createOwned(page, {
    restaurant_id: 'r_anker',
    table_id: 't_1',
    starts_at_local: `${pastThursday}T18:00`,
    party_size: 2,
  });
  if (past.status !== 201) fail(`past booking ${past.status} ${past.code}`);
  await page.getByTestId('lookup-reference-input').fill(past.reference);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-status').waitFor();
  await page.getByTestId('reservation-cancel-button').click();
  await page.getByTestId('reservation-error').waitFor();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed') fail('cutoff changed the status');
  if ((await page.getByTestId('reservation-cancel-button').count()) !== 1) fail('cutoff removed the button');
  await captureQuartet(page, 'lookup-cutoff');
  await pass('R160-cutoff', 'past booking stays confirmed when the accepted cutoff has passed');

  const near = await createOwned(page, {
    restaurant_id: 'r_anker',
    table_id: 't_3',
    starts_at_local: `${nearThursday}T18:00`,
    party_size: 2,
  });
  if (near.status !== 201) fail(`near booking ${near.status} ${near.code}`);
  const superseded = await publish(manager.token, policyBody(policyFrom, 10080, 60, baseCaps));
  if (superseded.version !== 3 || superseded.cutoff !== 10080) fail(`superseding policy ${superseded.version}`);
  const nearBefore = await lookupSummary(page, near.reference);
  if (nearBefore.status !== 200 || nearBefore.cutoff !== 60 || nearBefore.revision !== 1) {
    fail(`accepted cutoff before cancel ${nearBefore.status} cutoff ${nearBefore.cutoff}`);
  }
  await page.getByTestId('lookup-reference-input').fill(near.reference);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-cancel-button').waitFor();
  await page.getByTestId('reservation-cancel-button').click();
  await page.waitForFunction(() => document.querySelector('[data-testid="reservation-status"]')?.textContent?.trim() === 'cancelled');
  if ((await page.getByTestId('reservation-error').count()) !== 0) fail('accepted cutoff was refused');
  await pass('R247', 'cancel uses the accepted 60-minute cutoff after a 10080-minute policy');

  const other = await browser.newContext({ viewport: { width: 1280, height: 900 }, colorScheme: 'dark' });
  await other.addInitScript(() => localStorage.setItem('chaaya-theme', 'light'));
  const otherPage = await other.newPage();
  otherPage.on('pageerror', (error) => report.pageErrors.push(String(error.message ?? error)));
  await loginUi(otherPage, destination, 'nia.s3g2@example.com');
  await otherPage.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await otherPage.getByTestId('lookup-reference-input').fill(past.reference);
  await otherPage.getByTestId('lookup-submit').click();
  await otherPage.getByTestId('reservation-error').waitFor();
  if ((await otherPage.getByTestId('reservation-detail').count()) !== 0) fail('non-owner saw the reservation');
  await other.close();
  await pass('R160-owner', 'another diner sees reservation-error for a private reference');

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

  await page.getByTestId('logout-button').click();
  await assertSessionChrome(page, { signedIn: false });
  await pass('R140', 'logout clears the signed-in name');
  await finishVideo(context, page, 'main-desktop.webm');

  state.dropped = null;
  state.reservations = [];
  state.lookups = [];
}

async function runBlankName(browser) {
  const context = await browser.newContext({ viewport: { width: 375, height: 812 }, colorScheme: 'dark' });
  await context.addInitScript(() => localStorage.setItem('chaaya-theme', 'dark'));
  const page = await context.newPage();
  page.on('pageerror', (error) => report.pageErrors.push(String(error.message ?? error)));
  await page.goto(`${destination}/signup`);
  await page.getByTestId('signup-email').fill('blank.s3g2@example.com');
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
  const state = freshState();
  state.captureReservation = true;
  const { context, page } = await launch(browser, { width: 375, height: 812, video: true });
  await installRouter(page, state);
  await signupUi(page, destination, 'phone.s3g2@example.com', 'Ada');
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 8);
  await waitSearch(page);
  await settledScroll(page);
  await page.getByTestId('slot-t_1+t_2-22:00').click();
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
  const state = freshState();
  state.captureReservation = true;
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true, reduced: true });
  await installRouter(page, state);
  await signupUi(page, destination, 'motion.s3g2@example.com', 'Ada');
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 8);
  await waitSearch(page);
  const cell = page.getByTestId('slot-t_1+t_2-18:00');
  await cell.focus();
  await page.keyboard.press('Enter');
  const selected = await cell.getAttribute('data-selected');
  const running = await cell.evaluate((element) => element.getAnimations({ subtree: true }).filter((item) => item.playState === 'running').length);
  report.reducedSelection = { selected, runningAnimations: running };
  if (selected !== 'true') fail(`reduced selection ${selected}`);
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
    window.__s3g2Mark = 'alive';
    document.documentElement.dataset.s3g2Doc = doc;
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
  const committed = receiptSummary(state.dropped.text);
  const sent = JSON.parse(state.dropped.body);
  if (kind === 'stage1') {
    if (!committed.hasTableId || committed.hasTableIds || committed.hasRevision || committed.hasAcceptedTerms || committed.tableId !== 't_2') {
      fail(`${name} receipt was not a legacy singleton`);
    }
    if (sent.table_id !== 't_2' || sent.table_ids || sent.party_size !== party) fail(`${name} pending body changed`);
  } else {
    if (committed.hasTableId || !committed.hasTableIds || committed.tableIds?.join('+') !== 't_1+t_2' || committed.hasRevision || committed.hasAcceptedTerms) {
      fail(`${name} receipt was not a legacy pair`);
    }
    if (sent.table_id || sent.table_ids?.join('+') !== 't_1+t_2' || sent.party_size !== party) fail(`${name} pending body changed`);
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
  if (!retry || retry.status !== 200 || retry.cachedBodyReused !== false) fail(`${name} retry ${retry?.status ?? 'missing'}`);
  if (!retry.fetchedUrl.startsWith(destination)) fail(`${name} retry did not reach the destination`);
  if (retry.key !== state.dropped.key || !sameJsonText(retry.body, state.dropped.body) || retry.text !== state.dropped.text) {
    fail(`${name} retry key, body, or bytes differed`);
  }
  const shown = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (shown !== committed.reference) fail(`${name} confirmation was not the original reference`);
  if ((await page.getByTestId('booking-uncertain').count()) !== 0 || (await page.getByTestId('booking-error').count()) !== 0) {
    fail(`${name} recovery left an error state`);
  }
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
  const token = await page.evaluate(() => JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}').token);
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
  if (still.mark !== 'alive' || still.doc !== name) fail(`${name} lookup reloaded the document`);
  await captureQuartet(page, `${name}-confirmed`, 300);
  report.upgrades[name] = {
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
    receipt: committed,
    lookupStatus: direct.status,
    lookupReference: directSummary.reference,
    oneRecord: listed.count === 1,
    displayNameRemained: true,
  };
  await pass(kind === 'stage1' ? 'R288-stage1' : 'R288-stage2', `${name} commit 201 lost, export 200, import 204, destination retry 200`);
  await finishVideo(context, page, `${name}-upgrade.webm`);
  state.dropped = null;
  state.reservations = [];
}

async function main() {
  await mkdir(shotDir, { recursive: true });
  await mkdir(videoDir, { recursive: true });
  await mkdir(privateDir, { recursive: true });
  await chmod(privateDir, 0o700);
  await writeFile(logPath, `COMMAND node scripts/s3-g2-live.mjs --stage1 ${stage1} --stage2 ${stage2} --destination ${destination} --evidence ${evidence}\nCWD ${worktree}/stage-3/web\nSTARTED ${new Date().toISOString()}\n`);
  const days = {
    thursday: weekday(thursday),
    laterThursday: weekday(laterThursday),
    sunday: weekday(sunday),
    pastThursday: weekday(pastThursday),
    nearThursday: weekday(nearThursday),
  };
  if (days.thursday !== 'thu' || days.laterThursday !== 'thu' || days.sunday !== 'sun' || days.pastThursday !== 'thu' || days.nearThursday !== 'thu') {
    fail(`weekdays ${JSON.stringify(days)}`);
  }
  const head = execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  const stageTrees = {
    stage1: execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD:stage-1'], { encoding: 'utf8' }).trim(),
    stage2: execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD:stage-2'], { encoding: 'utf8' }).trim(),
  };
  report.head = head;
  report.stageTrees = stageTrees;
  report.startedAt = new Date().toISOString();
  await log(`HEAD ${head}`);
  await log(`STAGE1 TREE ${stageTrees.stage1}`);
  await log(`STAGE2 TREE ${stageTrees.stage2}`);
  runContrastGate();
  await pass('R110', `${report.contrastGate.pairs} token pairs pass contrastGate`);

  await waitHealth(stage1);
  await waitHealth(stage2);
  await waitHealth(destination);
  const source1 = inspectContainer('tk-s3-g2-src1');
  const source2 = inspectContainer('tk-s3-g2-src2');
  const dest = inspectContainer('tk-s3-g2-dst');
  report.containers = { source1, source2, dest };
  await log(`SRC1 image=${source1.image} id=${source1.id} imageId=${source1.imageId} ${source1.portEnv} ${source1.status}`);
  await log(`SRC2 image=${source2.image} id=${source2.id} imageId=${source2.imageId} ${source2.portEnv} ${source2.status}`);
  await log(`DST image=${dest.image} id=${dest.id} imageId=${dest.imageId} ${dest.portEnv} ${dest.status}`);
  await expectCheck('containers', source1.portEnv === 'PORT=9149' && source2.portEnv === 'PORT=9150' && dest.portEnv === 'PORT=9151' && new Set([source1.imageId, source2.imageId, dest.imageId]).size === 3, 'three images on 9149/9150/9151');

  const browser = await chromium.launch({ executablePath: chrome, headless: true });
  try {
    await runDestination(browser);
    await runBlankName(browser);
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
    const pairRows = report.renderedContrast.filter((row) => row.role === 'selected-pair' && row.word === 'Held');
    for (const row of pairRows) {
      const expected = row.theme === 'light' ? 7.611 : row.theme === 'dark' ? 9.915 : null;
      if (expected == null) continue;
      if (Math.abs(row.ratio - expected) > 0.02) fail(`${row.role} ${row.width} ${row.theme} ${row.ratio} expected ${expected}`);
    }
    if (pairRows.filter((row) => row.theme === 'light' || row.theme === 'dark').length < 4) fail('selected pair contrast samples were incomplete');
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
  if (failures.length) {
    await log(`FAIL ${failures.join(' | ')}`);
    report.result = 'fail';
    report.failures = failures;
    report.endedAt = new Date().toISOString();
    await writeFile(join(evidence, 'report.json'), JSON.stringify(report, null, 2));
    process.exit(1);
  }
  report.result = 'pass';
  report.endedAt = new Date().toISOString();
  await writeFile(join(evidence, 'report.json'), JSON.stringify(report, null, 2));
  await log(`PASS probe ${report.endedAt}`);
}

const isMain = process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href;
if (isMain) {
  main().catch(async (error) => {
    const message = error instanceof Error ? error.message : String(error);
    await log(`FAIL ${message}`).catch(() => undefined);
    report.result = 'fail';
    report.failures = [...failures, message];
    report.endedAt = new Date().toISOString();
    await mkdir(evidence, { recursive: true }).catch(() => undefined);
    await writeFile(join(evidence, 'report.json'), JSON.stringify(report, null, 2)).catch(() => undefined);
    process.exit(1);
  });
}
