/**
 * S2-F live probe.
 *
 * Talks to a real stage-1 source, the reviewed stage-2 image, and the fixed
 * stage-2 destination. Asserts the four presentation repairs plus paired
 * booking, refusal, lost-response recovery, lookup, density, and a
 * between-request upgrade. Exits nonzero on failure.
 *
 * Does not print tokens, passwords, idempotency keys, or export bodies.
 * Reveal checks never call scrollIntoView or scrollIntoViewIfNeeded.
 *
 *   node scripts/s2-f-live.mjs --source URL --destination URL --before URL --evidence DIR
 */
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { chmod, mkdir, writeFile, appendFile, stat } from 'node:fs/promises';
import { join } from 'node:path';
import { contrastGate } from '@nrynss/chaaya/testing';
import { chromium } from 'playwright';

const worktree = '/home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-grok';
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const axePath = join(worktree, 'stage-2/web/node_modules/axe-core/axe.min.js');
const SECRET = 'correct horse';
const thursday = '2027-06-17';
const sunday = '2027-06-13';
const pastThursday = '2026-09-24';

const args = process.argv.slice(2);
function arg(name) {
  const index = args.indexOf(`--${name}`);
  return index >= 0 ? args[index + 1] : '';
}

const source = arg('source');
const destination = arg('destination');
const before = arg('before');
const evidence = arg('evidence');
const sourceContainer = arg('source-container') || 'tk-s2-f-src';
const destContainer = arg('dest-container') || 'tk-s2-f-dst';
const beforeContainer = arg('before-container') || 'tk-s2-f-before';

if (!source || !destination || !before || !evidence) {
  console.error('usage: node scripts/s2-f-live.mjs --source URL --destination URL --before URL --evidence DIR');
  process.exit(2);
}

const shotDir = join(evidence, 'shots');
const videoDir = join(evidence, 'video');
const privateDir = join(evidence, 'private');
const logPath = join(evidence, 'probe.log');
const report = {
  item: 'S2-F',
  source,
  destination,
  before,
  sourceContainer,
  destContainer,
  beforeContainer,
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
  beforeMeasurements: null,
};
const failures = [];

function log(message) {
  const line = `${message}\n`;
  process.stdout.write(line);
  return appendFile(logPath, line);
}

function sha256(text) {
  return createHash('sha256').update(text).digest('hex');
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

const hours = (opens, closes) => [{ weekday: 'thu', opens, closes }];

function anker(extra = {}) {
  return {
    id: 'r_anker',
    name: 'Zum Anker',
    timezone: 'Europe/Berlin',
    slot_minutes: 30,
    reservation_duration_minutes: 90,
    cancellation_cutoff_minutes: 120,
    opening_hours: hours('18:00', '23:00'),
    tables: [
      { id: 't_1', label: '1', capacity: 2 },
      { id: 't_2', label: '2', capacity: 4 },
      { id: 't_3', label: '3', capacity: 4 },
    ],
    ...extra,
  };
}

const pairFixture = {
  users: [],
  restaurants: [
    anker({ combinable: [['t_1', 't_2'], ['t_2', 't_3']] }),
    {
      id: 'r_nord',
      name: 'Nordlicht',
      timezone: 'America/New_York',
      slot_minutes: 30,
      reservation_duration_minutes: 90,
      cancellation_cutoff_minutes: 120,
      opening_hours: hours('17:00', '22:00'),
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
      opening_hours: hours('18:00', '22:00'),
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

const upgradeFixture = {
  users: [],
  restaurants: [
    {
      id: 'r_anker',
      name: 'Zum Anker',
      timezone: 'Europe/Berlin',
      slot_minutes: 30,
      reservation_duration_minutes: 90,
      cancellation_cutoff_minutes: 120,
      opening_hours: hours('18:00', '23:00'),
      tables: [
        { id: 't_1', label: '1', capacity: 2 },
        { id: 't_2', label: '2', capacity: 4 },
      ],
    },
  ],
  reservations: [],
};

async function reset(base, fixture) {
  const response = await fetch(`${base}/_test/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify(fixture),
  });
  if (response.status !== 204) fail(`reset ${base} status ${response.status}`);
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
        if (body.status !== 'ok') fail(`health ${base} body status ${body.status}`);
        return;
      }
    } catch (error) {
      last = error instanceof Error ? error.name : 'error';
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  fail(`health ${base} not ready (${last})`);
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
    pathname.startsWith('/_test/')
  );
}

function noteUrl(url) {
  if (url.hostname !== '127.0.0.1' && url.hostname !== 'localhost') {
    report.offOrigin.push(`${url.origin}${url.pathname}`);
  }
}

async function installRouter(page, state) {
  await page.route('**/*', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    noteUrl(url);
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
      };
      await route.abort('failed');
      return;
    }
    if (state.forward && isApi(url.pathname)) {
      const target = new URL(`${url.pathname}${url.search}`, state.forward).toString();
      const fetched = await route.fetch({ url: target });
      const headers = { ...fetched.headers() };
      delete headers['content-encoding'];
      delete headers['content-length'];
      const fresh = await fetched.text();
      if (state.captureReservation && request.method() === 'POST' && url.pathname === '/reservations') {
        state.reservations.push({
          status: fetched.status(),
          text: fresh,
          key: request.headers()['idempotency-key'] ?? '',
          body: request.postData() ?? '',
          fetchedUrl: target,
          cachedBodyReused: false,
        });
      }
      await route.fulfill({ status: fetched.status(), headers, body: fresh });
      return;
    }
    if (state.captureReservation && request.method() === 'POST' && url.pathname === '/reservations') {
      const fetched = await route.fetch();
      const headers = { ...fetched.headers() };
      delete headers['content-encoding'];
      delete headers['content-length'];
      const fresh = await fetched.text();
      state.reservations.push({
        status: fetched.status(),
        text: fresh,
        key: request.headers()['idempotency-key'] ?? '',
        body: request.postData() ?? '',
        fetchedUrl: request.url(),
        cachedBodyReused: false,
      });
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

async function pageBox(page) {
  return page.evaluate(() => {
    const root = document.documentElement;
    return { scrollWidth: root.scrollWidth, clientWidth: root.clientWidth };
  });
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
    failures.push(
      `${label} plate ${measured.plateOutside.length} word ${measured.wordOutside.length}`,
    );
  }
}

async function runAxe(page, label) {
  await page.addScriptTag({ path: axePath });
  const violations = await page.evaluate(async () => {
    const results = await window.axe.run(document, {
      resultTypes: ['violations'],
      rules: {
        // Same rule set as @nrynss/chaaya/testing a11yGate. axe cannot see SVG
        // fill. Token pairs stay on contrastGate. Painted grid words are
        // measured with getComputedStyle in measureRenderedContrast.
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
  const dead = await page.evaluate(() => {
    return [...document.querySelectorAll('button, a[href], input, select, textarea')].filter((element) => {
      if (element.hasAttribute('disabled')) return false;
      return element.getAttribute('tabindex') === '-1';
    }).length;
  });
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
          foregroundComputed: paint.color,
          backgroundComputed: paint.background,
          cellBackground: hexOf(paint.cellBackground),
          ratio: ratio === null ? null : Math.round(ratio * 1000) / 1000,
          available: paint.available,
          selected: paint.selected,
          fontSize: paint.fontSize,
          fontWeight: paint.fontWeight,
        };
        row.ratioText = row.ratio === null ? 'none' : row.ratio.toFixed(3);
        report.renderedContrast.push(row);
        await log(`CONTRAST ${row.role} ${width} ${theme} ${row.word} ${row.foreground} on ${row.background} ${row.ratioText}:1`);
        if (row.ratio === null || row.ratio < 4.5) {
          fail(`${row.role} ${width} ${theme} contrast ${row.ratio} ${row.foreground} on ${row.background}`);
        }
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
      await page.waitForTimeout(200);
      const file = join(shotDir, `${name}-${width}-${theme}.png`);
      await page.screenshot({ path: file, fullPage: true });
      report.shots.push(file);
      await assertNoPageOverflow(page, `${name}-${width}-${theme}`);
    }
  }
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.getByTestId('theme-light').click();
}

async function signupUi(page, email, displayName) {
  await page.goto(`${destination}/signup`);
  await page.getByTestId('signup-email').fill(email);
  await page.getByTestId('signup-password').fill(SECRET);
  await page.getByTestId('signup-display-name').fill(displayName);
  await page.getByTestId('signup-submit').click();
  await page.getByTestId('current-user').waitFor();
  const name = (await page.getByTestId('current-user').innerText()).trim();
  if (!name.includes(displayName)) fail(`signed-in name missing ${displayName}`);
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

async function assertAuthoritative(page, restaurant, date, party) {
  const live = await api(
    destination,
    `/availability?restaurant_id=${restaurant}&date=${date}&party_size=${party}`,
  );
  if (live.status !== 200) fail(`availability ${live.status} ${errorCode(live.text)}`);
  const body = JSON.parse(live.text);
  const detail = JSON.parse((await api(destination, `/restaurants/${restaurant}`)).text);
  const combinable = detail.combinable ?? [];
  let cells = 0;
  for (const slot of body.slots) {
    const clock = clockOf(slot.starts_at_local);
    const singles = new Set(slot.available_table_ids ?? []);
    for (const table of detail.tables) {
      const testId = `slot-${table.id}-${clock}`;
      const available = await page.getByTestId(testId).getAttribute('data-available');
      const expect = singles.has(table.id) ? 'true' : 'false';
      if (available !== expect) fail(`${testId} data-available ${available} expected ${expect}`);
      cells += 1;
    }
    for (const pair of combinable) {
      const testId = `slot-${pair[0]}+${pair[1]}-${clock}`;
      const listed = (slot.available_options ?? []).some(
        (item) =>
          Array.isArray(item.table_ids) &&
          item.table_ids.length === 2 &&
          item.table_ids[0] === pair[0] &&
          item.table_ids[1] === pair[1],
      );
      const available = await page.getByTestId(testId).getAttribute('data-available');
      if (available !== (listed ? 'true' : 'false')) fail(`${testId} data-available ${available}`);
      cells += 1;
    }
    const optionIds = (slot.available_options ?? []).map((item) => (item.table_ids ?? []).join('+'));
    const firstPair = optionIds.findIndex((id) => id.includes('+'));
    const singlesFirst = firstPair === -1 ? optionIds : optionIds.slice(0, firstPair);
    if (singlesFirst.some((id) => id.includes('+'))) fail(`options order at ${clock}`);
    const pairOptions = optionIds.filter((id) => id.includes('+'));
    const eligiblePairs = combinable.map((pair) => pair.join('+')).filter((id) => pairOptions.includes(id));
    if (eligiblePairs.join('|') !== pairOptions.join('|')) fail(`pair option order at ${clock}`);
    if ([...singles].some((id) => String(id).includes('+'))) fail(`available_table_ids pair at ${clock}`);
  }
  await pass('R146-R175-R187', `${cells} cells match ${body.slots.length} slots for party ${party}`);
  return body;
}

async function clickAndReadSelected(page, testId) {
  const locator = page.getByTestId(testId);
  await locator.click();
  return locator.getAttribute('data-selected');
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
  await video.saveAs(target);
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
  if ((await page.getByTestId('theme-light').count()) !== 1) fail('theme control missing');
}

async function knownScroll(page) {
  await page.evaluate(() => window.scrollTo(0, 0));
}

async function settledScroll(page) {
  // Smooth reveal keeps moving after the outcome is published. Two close
  // samples are not enough: a 40ms stall, or the pause before scrollBy
  // starts, used to be recorded as the final position.
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
    const visibleTop = Math.max(rect.top, 0);
    const visibleBottom = Math.min(rect.bottom, viewH);
    const visible = Math.max(0, visibleBottom - visibleTop);
    return {
      present: true,
      scrollY: window.scrollY,
      viewH,
      viewW,
      top: Math.round(rect.top * 10) / 10,
      bottom: Math.round(rect.bottom * 10) / 10,
      height: Math.round(rect.height * 10) / 10,
      visible: Math.round(visible * 10) / 10,
    };
  }, selector);
}

async function assertRevealed(page, label, selector, minVisible) {
  const box = await readBox(page, selector);
  report.reveal.push({ label, ...box });
  await log(`REVEAL ${label} present=${box.present} scrollY=${box.scrollY} top=${box.top} bottom=${box.bottom} visible=${box.visible} view=${box.viewW}x${box.viewH}`);
  if (!box.present) fail(`${label} missing`);
  if (box.visible < minVisible || box.top >= box.viewH - 8) {
    const trail = [];
    for (let sample = 0; sample < 8; sample += 1) {
      await page.waitForTimeout(100);
      trail.push(await page.evaluate(() => Math.round(window.scrollY)));
    }
    fail(`${label} not revealed visible=${box.visible} top=${box.top} trail=${trail.join(',')}`);
  }
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
    const seats = indexOf('.plan-seats');
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
    const hits = [];
    for (const node of svg.querySelectorAll('.plate-label')) {
      const rect = node.getBoundingClientRect();
      if (rect.width < 2 || rect.bottom < 0 || rect.top > window.innerHeight) continue;
      const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
      hits.push({
        text: (node.textContent || '').trim(),
        hit: hit ? hit.getAttribute('class') || hit.tagName : '',
      });
    }
    let gapHit = '';
    const path = svg.querySelector('.pair-link');
    if (path && typeof path.getPointAtLength === 'function') {
      const mid = path.getPointAtLength(path.getTotalLength() / 2);
      const matrix = path.getScreenCTM();
      if (matrix) {
        const x = matrix.a * mid.x + matrix.c * mid.y + matrix.e;
        const y = matrix.b * mid.x + matrix.d * mid.y + matrix.f;
        const hit = document.elementFromPoint(x, y);
        gapHit = hit ? hit.getAttribute('class') || hit.tagName : '';
      }
    }
    return { link, top, plate, seats, badge, planNames, plates, seatLines, hits, gapHit };
  });
  report.paint.push({ label, ...paint });
  await log(`PAINT ${label} order link=${paint.link} top=${paint.top} plate=${paint.plate} seats=${paint.seats} badge=${paint.badge} names=${(paint.planNames || []).length} hits=${(paint.hits || []).length}`);
  if (paint.missing) fail(`${label} floor missing`);
  if (!(paint.link >= 0 && paint.top > paint.link && paint.plate > paint.top && paint.badge > paint.plate)) {
    fail(`${label} paint order ${paint.link}/${paint.top}/${paint.plate}/${paint.badge}`);
  }
  if ((paint.planNames || []).length !== 0) fail(`${label} still has plan-name captions`);
  for (const plate of paint.plates || []) {
    if (plate.font < 13) fail(`${label} plate font ${plate.font} ${plate.text}`);
  }
  for (const line of paint.seatLines || []) {
    if (line.font < 13 || !/seat/.test(line.text)) fail(`${label} seat metadata ${line.text} ${line.font}`);
  }
  for (const hit of paint.hits || []) {
    if (/pair-link/.test(hit.hit)) fail(`${label} line cuts ${hit.text} via ${hit.hit}`);
  }
  return paint;
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
        opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
        tables: [
          { id: 't_1', label: 'Window alcove', capacity: 2 },
          { id: 't_2', label: 'Garden corner', capacity: 4 },
          { id: 't_3', label: 'Hearth booth', capacity: 4 },
        ],
        combinable: [
          ['t_1', 't_2'],
          ['t_2', 't_3'],
        ],
      },
    ],
    reservations: [],
  };
}

async function runDensity(browser) {
  for (const step of [15, 5]) {
    const expectCells = step === 15 ? 75 : 215;
    const expectSlots = step === 15 ? 15 : 43;
    await reset(destination, denseRestaurant(step));
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    await context.addInitScript(() => localStorage.setItem('chaaya-theme', 'light'));
    const page = await context.newPage();
    page.on('pageerror', (error) => report.pageErrors.push(String(error.message ?? error)));
    await page.goto(`${destination}/`);
    await page.getByTestId('restaurant-select').waitFor();
    await knownScroll(page);
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
          scrollWidth: root.scrollWidth,
          clientWidth: root.clientWidth,
          wrapScroll: wrap ? wrap.scrollWidth : 0,
          wrapClient: wrap ? wrap.clientWidth : 0,
          matrixDisplay: matrixStyle ? matrixStyle.display : '',
          matrixDirection: matrixStyle ? matrixStyle.flexDirection : '',
        };
      });
      report.layout.push({ label: `dense-${step}-${width}`, ...rail, cells, slots });
      await log(`DENSE ${step} ${width} cells=${cells} slots=${slots} page=${rail.pageOverflow} rail=${rail.railOverflow} wrap=${rail.wrapScroll}/${rail.wrapClient} matrix=${rail.matrixDisplay}/${rail.matrixDirection}`);
      if (rail.pageOverflow) fail(`dense ${step} ${width} page overflow`);
      // Desktop keeps the time tracks on an internal rail. Phone stacks each
      // slot as a full-width row, so the page stays still and there is no
      // horizontal rail to measure.
      if (width === 1280 && !rail.railOverflow) {
        fail(`dense ${step} desktop grid has no internal rail wrap=${rail.wrapScroll}/${rail.wrapClient}`);
      }
      if (width === 375 && (rail.matrixDisplay !== 'flex' || rail.matrixDirection !== 'column')) {
        fail(`dense ${step} phone matrix ${rail.matrixDisplay}/${rail.matrixDirection}`);
      }
    }
    await context.close();
  }
  await pass('R117-dense', '15 slots/75 cells and 43 slots/215 cells stay inside the page');
}

async function runBefore(browser) {
  await waitHealth(before);
  const info = inspectContainer(beforeContainer);
  report.beforeInfo = info;
  await log(`BEFORE image=${info.image} id=${info.id.slice(0, 12)} ${info.portEnv} ${info.status}`);
  await reset(before, pairFixture);
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  await context.addInitScript(() => localStorage.setItem('chaaya-theme', 'light'));
  const page = await context.newPage();
  await page.goto(`${before}/signup`);
  await page.getByTestId('signup-email').fill('before.s2f@example.com');
  await page.getByTestId('signup-password').fill(SECRET);
  await page.getByTestId('signup-display-name').fill('Dana');
  await page.getByTestId('signup-submit').click();
  await page.getByTestId('current-user').waitFor();
  const labels = await navLabels(page);
  await knownScroll(page);
  await page.goto(`${before}/`);
  await page.getByTestId('restaurant-select').waitFor();
  await knownScroll(page);
  await search(page, 'r_hall', thursday, 2);
  await waitSearch(page);
  await page.waitForTimeout(500);
  const grid = await readBox(page, '.grid-caption');
  const floor = await readBox(page, '[data-testid="floor-plan"]');
  const paint = await page.evaluate(() => {
    const svg = document.querySelector('svg.room-scene');
    const nodes = svg ? [...svg.querySelectorAll('*')] : [];
    const indexOf = (selector) => nodes.findIndex((node) => node.matches(selector));
    return {
      link: indexOf('.pair-link'),
      plate: indexOf('.plate-label'),
      planNames: [...document.querySelectorAll('.plan-name')].map((node) => (node.textContent || '').trim()),
      nav: [...document.querySelectorAll('nav[aria-label="Primary"] a')].map((node) => (node.textContent || '').trim()),
    };
  });
  const shot = join(evidence, 'before', 'results-d-light.png');
  await mkdir(join(evidence, 'before'), { recursive: true });
  await page.screenshot({ path: shot, fullPage: false });
  report.beforeMeasurements = { labels, grid, floor, paint, shot, image: info.image, id: info.id };
  await writeFile(join(evidence, 'before', 'measurements.json'), JSON.stringify(report.beforeMeasurements, null, 2));
  await log(`BEFORE nav=${labels.join('|')} gridTop=${grid.top} scrollY=${grid.scrollY} planNames=${paint.planNames.length} link=${paint.link} plate=${paint.plate}`);
  if (!labels.includes('Sign in') || !labels.includes('Create account')) fail('before image did not reproduce signed-in auth links');
  if (paint.planNames.length === 0) fail('before image did not reproduce table captions');
  if (!(paint.link > paint.plate)) fail(`before image did not paint the connector after the plate (${paint.link} ${paint.plate})`);
  await pass('F-before', 'reviewed image still shows auth links, duplicate captions, and a connector above the plate');
  await context.close();
}

async function runBlankName(browser) {
  const context = await browser.newContext({ viewport: { width: 375, height: 812 } });
  await context.addInitScript(() => localStorage.setItem('chaaya-theme', 'dark'));
  const page = await context.newPage();
  page.on('pageerror', (error) => report.pageErrors.push(String(error.message ?? error)));
  await page.goto(`${destination}/signup`);
  await page.getByTestId('signup-email').fill('blank.s2f@example.com');
  await page.getByTestId('signup-password').fill(SECRET);
  await page.getByTestId('signup-display-name').fill('');
  await page.getByTestId('signup-submit').click();
  await page.getByTestId('logout-button').waitFor();
  for (const path of ['/', '/lookup', '/login', '/signup']) {
    await page.goto(`${destination}${path}`);
    await page.getByTestId('logout-button').waitFor();
    await assertSessionChrome(page, { signedIn: true, display: '' });
  }
  await page.getByTestId('logout-button').click();
  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Sign in' }).waitFor();
  await assertSessionChrome(page, { signedIn: false });
  await context.close();
  await pass('F1-blank', 'blank display name stays signed in and logout restores the auth links');
}

async function runRevealVideo(browser) {
  await reset(destination, pairFixture);
  const state = { reservations: [], dropNextReservation: false, captureReservation: false, forward: '' };
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true, reduced: true });
  await installRouter(page, state);
  await signupUi(page, 'reveal.s2f@example.com', 'Ada');
  await knownScroll(page);
  await search(page, 'r_hall', thursday, 2);
  await waitSearch(page);
  const grid = await assertRevealed(page, 'reveal-video-grid', '.grid-caption', 24);
  const floor = await readBox(page, '[data-testid="floor-plan"]');
  if (floor.bottom <= 0) fail(`reveal video scrolled past the floor bottom=${floor.bottom}`);
  if (grid.top < 80) fail(`reveal video pinned the grid to the top ${grid.top}`);
  await page.getByTestId('plan-t_alcove').focus();
  await page.keyboard.press('Enter');
  await page.getByTestId('booking-form').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'reveal-video-form', '[data-testid="booking-form"]', 120);
  await page.getByTestId('booking-submit').click();
  await page.getByTestId('confirmation-reference').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'reveal-video-confirmation', '[data-testid="confirmation"]', 80);
  if ((await page.getByTestId('booking-form').count()) !== 1) fail('reveal video removed the form');
  await finishVideo(context, page, 'reveal-desktop.webm');
  await pass('F2-reveal-video', 'reduced-motion reveal keeps the floor in reach and shows the grid, form, and confirmation');
}

async function main() {
  await mkdir(shotDir, { recursive: true });
  await mkdir(videoDir, { recursive: true });
  await mkdir(privateDir, { recursive: true });
  await chmod(privateDir, 0o700);
  await writeFile(logPath, `COMMAND node scripts/s2-f-live.mjs --source ${source} --destination ${destination} --before ${before} --evidence ${evidence}\nCWD ${worktree}/stage-2/web\n`);
  if (weekday(thursday) !== 'thu' || weekday(sunday) !== 'sun' || weekday(pastThursday) !== 'thu') {
    fail(`fixture weekdays ${weekday(thursday)} ${weekday(sunday)} ${weekday(pastThursday)}`);
  }
  const head = execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  const gitStatus = execFileSync('git', ['-C', worktree, 'status', '--short'], { encoding: 'utf8' });
  const appDiff = execFileSync('git', ['-C', worktree, 'diff', '--', 'stage-2/web/src/app.css'], { encoding: 'utf8' });
  report.head = head;
  report.gitStatus = gitStatus.trim().split('\n').filter(Boolean);
  report.productFixCommitted = false;
  report.appCssDiff = appDiff;
  report.startedAt = new Date().toISOString();
  await log(`HEAD ${head}`);
  await log(`GIT STATUS uncommitted\n${gitStatus}`);
  await log(`APP.CSS DIFF\n${appDiff}`);
  runContrastGate();
  await pass('R110', `${report.contrastGate.pairs} token pairs pass contrastGate`);

  await waitHealth(source);
  await waitHealth(destination);
  const sourceInfo = inspectContainer(sourceContainer);
  const destInfo = inspectContainer(destContainer);
  report.sourceInfo = sourceInfo;
  report.destInfo = destInfo;
  await log(`SOURCE image=${sourceInfo.image} id=${sourceInfo.id.slice(0, 12)} ${sourceInfo.portEnv} ${sourceInfo.status}`);
  await log(`DEST image=${destInfo.image} id=${destInfo.id.slice(0, 12)} ${destInfo.portEnv} ${destInfo.status}`);
  await expectCheck('images-distinct', sourceInfo.imageId !== destInfo.imageId && sourceInfo.id !== destInfo.id, 'source and destination containers differ');
  await expectCheck('ports', sourceInfo.portEnv === 'PORT=9028' && destInfo.portEnv === 'PORT=9029', 'PORT 9028 source and 9029 destination');

  await reset(destination, pairFixture);
  const browser = await chromium.launch({ executablePath: chrome, headless: true });
  try {
    await runBefore(browser);
    await runDestination(browser);
    await runBlankName(browser);
    await runDensity(browser);
    await runPhoneVideo(browser);
    await runReducedVideo(browser);
    await runRevealVideo(browser);
    await runUpgrade(browser);
    const pairRows = report.renderedContrast.filter((row) => row.role.includes('selected-pair') && row.word === 'Held');
    if (pairRows.length < 2) fail(`selected pair contrast samples ${pairRows.length}`);
    for (const row of pairRows) {
      const expected = row.theme === 'light' ? 7.611 : row.theme === 'dark' ? 9.915 : null;
      if (expected == null) continue;
      if (Math.abs(row.ratio - expected) > 0.02) fail(`${row.role} ${row.width} ${row.theme} ${row.ratio} expected ${expected}`);
    }
    await pass('F-contrast', `${pairRows.length} selected-pair samples include 7.611 light and 9.915 dark`);
  } finally {
    await browser.close();
  }

  const noisy = report.pageErrors.filter((item) => !/Failed to fetch|NetworkError|aborted|favicon/i.test(item));
  const noisyConsole = report.consoleErrors.filter((item) => !/ERR_FAILED|Failed to fetch|net::|favicon|status of 409/i.test(item));
  report.unexpectedPageErrors = noisy;
  report.unexpectedConsole = noisyConsole;
  if (report.offOrigin.length) failures.push(`off-origin ${report.offOrigin.length}`);
  if (noisy.length) failures.push(`pageerror ${noisy.length}`);
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

async function runDestination(browser) {
  const state = { reservations: [], dropNextReservation: false, failAvailability: false, forward: '', captureReservation: true, hold: null, holdGate: null };
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
  await runAxe(page, 'loading-1280-light');
  state.hold = null;
  releaseCatalog();
  await page.getByTestId('restaurant-select').waitFor();
  await pass('R115-loading', 'loading-state captured before catalogue resolved');

  await signupUi(page, 'ada.s2f@example.com', 'Ada');
  await pass('R137-R139', 'signup hooks and current-user Ada');
  const user = await page.getByTestId('current-user').innerText();
  if (!user.includes('Ada')) fail('current-user');
  for (const path of ['/', '/lookup', '/login', '/signup']) {
    await page.goto(`${destination}${path}`);
    await assertSessionChrome(page, { signedIn: true, display: 'Ada' });
  }
  await page.goto(`${destination}/`);
  await page.getByTestId('restaurant-select').waitFor();
  await pass('F1', 'signed-in auth links are absent on every route');

  await knownScroll(page);
  await search(page, 'r_anker', sunday, 2);
  await waitSearch(page);
  if ((await page.getByTestId('no-slots').count()) !== 1) fail('closed day missing no-slots');
  if ((await page.getByTestId('availability-grid').count()) !== 0) fail('closed day still shows the grid');
  await settledScroll(page);
  await assertRevealed(page, 'closed-day', '[data-testid="no-slots"]', 80);
  const emptyText = await page.getByTestId('no-slots').innerText();
  if (!/Sunday 13 June 2027/.test(emptyText)) fail(`empty date text ${emptyText}`);
  await captureQuartet(page, 'empty');
  await runAxe(page, 'empty');
  await pass('R144-empty', 'no-slots replaces the grid on a closed day');

  state.failAvailability = true;
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 2);
  await page.getByTestId('search-error').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'search-error', '[data-testid="search-error"]', 40);
  if ((await page.getByTestId('availability-grid').count()) !== 0) fail('error still shows the grid');
  await captureQuartet(page, 'error');
  await runAxe(page, 'error');
  await pass('R115-error', 'search-error after a lost availability response');

  await knownScroll(page);
  await search(page, 'r_anker', thursday, 2);
  await waitSearch(page);
  await settledScroll(page);
  const gridReveal = await assertRevealed(page, 'party-2-grid', '.grid-caption', 24);
  const floorReveal = await readBox(page, '[data-testid="floor-plan"]');
  report.reveal.push({ label: 'party-2-floor', ...floorReveal });
  if (floorReveal.bottom <= 0) fail(`search reveal bypassed the floor bottom=${floorReveal.bottom}`);
  if (gridReveal.top < 48) fail(`search reveal pinned the grid heading to ${gridReveal.top}`);
  const lede = await page.locator('h1.display').locator('xpath=following-sibling::p[1]').innerText();
  if (!lede.includes('Thursday 17 June 2027')) fail(`lede ${lede}`);
  await pass('R116', lede);
  const partyTwo = await assertAuthoritative(page, 'r_anker', thursday, 2);
  const slot1800 = partyTwo.slots.find((slot) => slot.starts_at_local.endsWith('T18:00'));
  if (!slot1800?.available_table_ids?.includes('t_1') || !slot1800.available_options?.some((item) => item.table_ids?.join('+') === 't_1+t_2')) {
    fail('party 2 missing single and declared pair');
  }
  await pass('R174-R176-R177', 'party 2 options include the single and the declared pair');

  await page.getByTestId('slot-t_1-18:00').click();
  await page.getByTestId('booking-form').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'single-form', '[data-testid="booking-form"]', 120);
  await measureRenderedContrast(page, [
    { role: 'selected-single', testId: 'slot-t_1-18:00', word: 'Held', available: 'true', selected: 'true' },
    { role: 'available-pair', testId: 'slot-t_1+t_2-18:00', word: 'Free', available: 'true', selected: 'false' },
  ], 'party-2');
  await page.getByTestId('slot-t_1+t_2-19:30').click();
  await measureRenderedContrast(page, [
    { role: 'selected-pair', testId: 'slot-t_1+t_2-19:30', word: 'Held', available: 'true', selected: 'true' },
    { role: 'available-single', testId: 'slot-t_2-18:00', word: 'Free', available: 'true', selected: 'false' },
  ], 'party-2');

  const guest = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  await guest.addInitScript(() => localStorage.setItem('chaaya-theme', 'light'));
  const guestPage = await guest.newPage();
  await guestPage.goto(`${destination}/`);
  await guestPage.getByTestId('restaurant-select').waitFor();
  await search(guestPage, 'r_anker', thursday, 9);
  await waitSearch(guestPage);
  if ((await guestPage.getByTestId('no-slots').count()) !== 0) fail('party 9 hid the grid');
  if ((await guestPage.getByTestId('availability-grid').count()) !== 1) fail('party 9 removed the grid');
  const inert = await guestPage.getByTestId('slot-t_1-18:00').getAttribute('data-available');
  if (inert !== 'false') fail(`party 9 cell ${inert}`);
  await measureRenderedContrast(guestPage, [
    { role: 'unavailable-single', testId: 'slot-t_1-18:00', word: 'Taken', available: 'false', selected: 'false' },
    { role: 'unavailable-pair', testId: 'slot-t_1+t_2-18:00', word: 'Taken', available: 'false', selected: 'false' },
  ], 'party-9');
  await guestPage.getByTestId('slot-t_1-18:00').click();
  await guestPage.waitForTimeout(150);
  if ((await guestPage.getByTestId('auth-error').count()) !== 0) fail('unavailable cell raised auth-error');
  if ((await guestPage.getByTestId('booking-form').count()) !== 0) fail('unavailable cell opened a form');
  await pass('R144-R148', 'a day with slots keeps the grid, and an unavailable cell is inert');
  await guestPage.getByTestId('party-size-input').fill('2');
  await guestPage.getByTestId('search-button').click();
  await waitSearch(guestPage);
  await guestPage.getByTestId('slot-t_1-18:00').click();
  await guestPage.getByTestId('auth-error').waitFor();
  const authText = await guestPage.getByTestId('auth-error').innerText();
  if (!/Sign in/i.test(authText)) fail('signed-out available cell did not ask for sign-in');
  if ((await guestPage.getByTestId('booking-form').count()) !== 0) fail('signed-out click opened a form');
  await captureQuartet(guestPage, 'auth-error');
  await pass('R149', 'signed-out available cell shows auth-error');
  await guestPage.goto(`${destination}/login`);
  await guestPage.getByTestId('login-email').fill('ada.s2f@example.com');
  await guestPage.getByTestId('login-password').fill('not-the-password');
  await guestPage.getByTestId('login-submit').click();
  await guestPage.getByTestId('auth-error').waitFor();
  await captureQuartet(guestPage, 'auth-bad-login');
  await runAxe(guestPage, 'auth-bad-login');
  if ((await guestPage.getByTestId('current-user').count()) !== 0) fail('bad login signed in');
  await pass('R138', 'auth-error only after a refused login');
  await guest.close();

  const selectedNow = await clickAndReadSelected(page, 'slot-t_3-18:00');
  if (selectedNow !== 'true') fail(`selection attr ${selectedNow} was not immediate`);
  await pass('R114-R147', 'single cell selected immediately');
  const singleSummary = await page.getByTestId('booking-summary').innerText();
  if (!singleSummary.includes('Table 1') && !singleSummary.includes('Table 3')) fail(singleSummary);
  if (!singleSummary.includes('Table 3') || !singleSummary.includes('6:00 PM') || !singleSummary.includes('18:00')) {
    fail(`singleton summary ${singleSummary}`);
  }
  const single = await bookFromForm(page, state);
  const singleBody = JSON.parse(single.body);
  if (single.status !== 201 || singleBody.table_id !== 't_3' || 'table_ids' in singleBody) {
    fail(`singleton create ${single.status} fields ${Object.keys(singleBody).join(',')}`);
  }
  const singleReceipt = JSON.parse(single.text);
  if (!singleReceipt.table_id || !Array.isArray(singleReceipt.table_ids)) fail('stage-2 singleton response shape');
  await pass('R167-R179-R180', 'singleton request keeps table_id and the response has both shapes');
  const replay = await bookFromForm(page, state);
  if (replay.status !== 200 || replay.key !== single.key || !sameJsonText(replay.body, single.body) || !sameJsonText(replay.text, single.text)) {
    fail(`singleton replay ${replay.status}`);
  }
  const singleRef = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (singleRef !== singleReceipt.reference) fail('confirmation before or other than the server reference');
  await pass('R153-R155', `replay 200 reference ${singleRef}`);

  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  await assertAuthoritative(page, 'r_anker', thursday, 6);
  if ((await page.getByTestId('slot-t_1-18:00').getAttribute('data-available')) !== 'false') fail('party 6 single looked available');
  const formCount = await page.getByTestId('booking-form').count();
  await page.getByTestId('slot-t_1-18:00').click();
  await page.waitForTimeout(100);
  if ((await page.getByTestId('booking-form').count()) !== formCount) fail('unavailable signed-in cell opened a form');
  await pass('R148-signed-in', 'unavailable cell does nothing for a signed-in diner');

  const pairSelected = await clickAndReadSelected(page, 'slot-t_1+t_2-19:30');
  if (pairSelected !== 'true') fail(`pair selection ${pairSelected}`);
  const pairSummary = await page.getByTestId('booking-summary').innerText();
  if (!pairSummary.includes('Table 1') || !pairSummary.includes('Table 2') || !pairSummary.includes('7:30 PM')) {
    fail(`pair summary ${pairSummary}`);
  }
  const badge = await page.getByTestId('plan-t_1+t_2').evaluate((element) => element.textContent ?? '');
  if (badge.includes('t_1') || badge.includes('t_2') || !badge.includes('1') || !badge.includes('2')) {
    fail(`pair chip ${badge}`);
  }
  await captureQuartet(page, 'pair-selected', 700);
  await measureSettled(page, 'pair-selected-1280');
  await page.setViewportSize({ width: 375, height: 812 });
  await page.waitForTimeout(300);
  await measureSettled(page, 'pair-selected-375');
  await page.setViewportSize({ width: 1280, height: 900 });
  await pass('R188-R196', `summary and chip use labels (${badge})`);

  const rivalToken = await signup(destination, 'nia.s2f@example.com', 'Nia');
  const rival = await api(destination, '/reservations', {
    method: 'POST',
    token: rivalToken,
    key: randomKey(),
    body: { restaurant_id: 'r_anker', table_ids: ['t_1', 't_2'], starts_at_local: `${thursday}T19:30`, party_size: 6 },
  });
  if (rival.status !== 201) fail(`rival create ${rival.status} ${errorCode(rival.text)}`);
  const partySize = await page.getByTestId('booking-party-size').inputValue();
  const refused = await bookFromForm(page, state);
  if (refused.status !== 409 || errorCode(refused.text) !== 'table_unavailable') fail(`expected 409 table_unavailable got ${refused.status}`);
  await page.getByTestId('booking-error').waitFor();
  if ((await page.getByTestId('confirmation').count()) !== 0) fail('rival 409 showed a confirmation');
  if ((await page.getByTestId('booking-uncertain').count()) !== 0) fail('rival 409 showed uncertainty');
  if ((await page.getByTestId('booking-party-size').inputValue()) !== partySize) fail('conflict cleared the party size');
  if ((await page.getByTestId('booking-form').count()) !== 1) fail('conflict closed the form');
  await page.waitForFunction(
    () => document.querySelector('[data-testid="slot-t_1+t_2-19:30"]')?.getAttribute('data-available') === 'false',
    null,
    { timeout: 8000 },
  );
  await captureQuartet(page, 'booking-error');
  await pass('R126-R129-R134', 'rival 409 keeps the form and refreshes availability');

  const recoveredSelect = await clickAndReadSelected(page, 'slot-t_1+t_2-21:00');
  if (recoveredSelect !== 'true') fail('later pair was not selectable');
  const created = await bookFromForm(page, state);
  if (created.status !== 201) fail(`pair create ${created.status} ${errorCode(created.text)}`);
  const createdBody = JSON.parse(created.body);
  const createdReceipt = JSON.parse(created.text);
  if (createdBody.table_id || createdBody.table_ids?.join('+') !== 't_1+t_2') fail('pair body was not table_ids only');
  if (createdReceipt.table_id || createdReceipt.table_ids?.join('+') !== 't_1+t_2') fail('pair receipt shape');
  await page.getByTestId('confirmation-reference').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'pair-confirmation', '[data-testid="confirmation"]', 80);
  if ((await page.getByTestId('booking-form').count()) !== 1) fail('confirmation removed the form');
  const pairRef = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (pairRef !== createdReceipt.reference) fail('pair confirmation did not use the server reference');
  const tables = await page.getByTestId('confirmation-tables').innerText();
  const details = await page.getByTestId('confirmation-details').innerText();
  if (!tables.includes('Table 1') || !tables.includes('Table 2')) fail(`confirmation tables ${tables}`);
  if (!details.includes('Zum Anker') || !details.includes('Table 1') || !details.includes('Table 2') || !details.includes('21:00')) {
    fail(`confirmation details ${details}`);
  }
  await captureQuartet(page, 'confirmed', 700);
  await runAxe(page, 'confirmed');
  await measureRenderedContrast(page, [
    { role: 'confirmed-selected-pair', testId: 'slot-t_1+t_2-21:00', word: 'Held', available: 'true', selected: 'true' },
  ], 'confirmed');
  const painted = report.renderedContrast.length;
  await pass('R118-R198', `${painted} rendered state words are at least 4.5:1`);
  await pass('R155-R189', `pair confirmation ${pairRef}`);

  const replayed = await bookFromForm(page, state);
  if (replayed.status !== 200 || replayed.key !== created.key || !sameJsonText(replayed.body, created.body) || replayed.text !== created.text && !sameJsonText(replayed.text, created.text)) {
    fail(`pair replay ${replayed.status}`);
  }
  if ((await page.getByTestId('confirmation-reference').innerText()).trim() !== pairRef) fail('replay changed the reference');
  await pass('R153-pair', 'unchanged pair resubmit returned the original reference');

  state.dropNextReservation = true;
  if ((await page.getByTestId('slot-t_1+t_2-18:00').getAttribute('data-available')) !== 'true') {
    fail('drop target pair was not available');
  }
  await page.getByTestId('slot-t_1+t_2-18:00').click();
  await page.getByTestId('booking-summary').waitFor();
  const dropClick = page.getByTestId('booking-submit').click();
  await page.getByTestId('booking-uncertain').waitFor();
  await dropClick;
  if (!state.dropped || state.dropped.servedToBrowser !== false) fail('drop did not keep the response from the browser');
  if (state.dropped.status !== 201) fail(`pair drop commit ${state.dropped.status} ${errorCode(state.dropped.text)}`);
  if ((await page.getByTestId('booking-error').count()) !== 0) fail('lost pair response showed booking-error');
  if ((await page.getByTestId('confirmation').count()) !== 0) fail('lost pair response showed a confirmation');
  const uncertainText = (await page.getByTestId('booking-uncertain').innerText()).trim();
  if (!uncertainText) fail('booking-uncertain was empty');
  await captureQuartet(page, 'uncertain', 200);
  await runAxe(page, 'uncertain');
  const droppedReceipt = JSON.parse(state.dropped.text);
  const retry = await bookFromForm(page, state);
  if (retry.status !== 200 || retry.key !== state.dropped.key || !sameJsonText(retry.body, state.dropped.body) || !sameJsonText(retry.text, state.dropped.text)) {
    fail(`pair uncertain retry ${retry.status}`);
  }
  if (retry.cachedBodyReused !== false) fail('retry reused a cached body');
  await page.getByTestId('confirmation-reference').waitFor();
  if ((await page.getByTestId('booking-uncertain').count()) !== 0) fail('retry left uncertainty on screen');
  if ((await page.getByTestId('confirmation-reference').innerText()).trim() !== droppedReceipt.reference) fail('retry showed a different reference');
  await pass('R130-R133-R135', `lost pair response recovered ${droppedReceipt.reference}`);

  const oldKey = retry.key;
  await page.getByTestId('booking-party-size').fill('5');
  const changed = await bookFromForm(page, state);
  const changedBody = JSON.parse(changed.body);
  if (changed.key === oldKey || changedBody.party_size !== 5) fail('changed party reused the key or body');
  if ((await page.getByTestId('confirmation').count()) !== 0) fail('changed request kept the previous confirmation');
  await pass('R154', `changed party used a new key and status ${changed.status}`);

  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await page.getByTestId('lookup-reference-input').fill(pairRef);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-detail').waitFor();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed') fail('lookup status');
  const looked = await page.getByTestId('reservation-tables').innerText();
  if (!looked.includes('Table 1') || !looked.includes('Table 2')) fail(`lookup tables ${looked}`);
  await captureQuartet(page, 'lookup-confirmed');
  await pass('R157-R190', 'lookup names both table labels');
  await page.getByTestId('reservation-cancel-button').click();
  await page.waitForFunction(
    () => document.querySelector('[data-testid="reservation-status"]')?.textContent?.trim() === 'cancelled',
  );
  if ((await page.getByTestId('reservation-cancel-button').count()) !== 0) fail('cancel button remained');
  await captureQuartet(page, 'lookup-cancelled');
  await pass('R159-R186', 'pair cancel removes the button');

  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Search' }).click();
  await search(page, 'r_anker', thursday, 2);
  await waitSearch(page);
  const freedA = await page.getByTestId('slot-t_1-21:00').getAttribute('data-available');
  const freedB = await page.getByTestId('slot-t_2-21:00').getAttribute('data-available');
  const freedPair = await page.getByTestId('slot-t_1+t_2-21:00').getAttribute('data-available');
  if (freedA !== 'true' || freedB !== 'true' || freedPair !== 'true') fail(`cancel freed ${freedA} ${freedB} ${freedPair}`);
  await pass('R186-grid', 'cancelling the pair freed both members');

  const past = await page.evaluate(async (when) => {
    const session = JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}');
    const response = await fetch('/reservations', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json; charset=utf-8',
        Authorization: `Bearer ${session.token}`,
        'Idempotency-Key': crypto.randomUUID(),
      },
      body: JSON.stringify({ restaurant_id: 'r_anker', table_id: 't_1', starts_at_local: when, party_size: 2 }),
    });
    const payload = await response.json();
    return { status: response.status, reference: payload.reference ?? '', code: payload.error?.code ?? '' };
  }, `${pastThursday}T18:00`);
  if (past.status !== 201) fail(`past booking ${past.status} ${past.code}`);
  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await page.getByTestId('lookup-reference-input').fill(past.reference);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-status').waitFor();
  await page.getByTestId('reservation-cancel-button').click();
  await page.getByTestId('reservation-error').waitFor();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed') fail('cutoff changed status');
  if ((await page.getByTestId('reservation-cancel-button').count()) !== 1) fail('cutoff removed the button');
  await captureQuartet(page, 'lookup-cutoff');
  await pass('R160-cutoff', 'cutoff stays confirmed and keeps the button');

  const other = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  await other.addInitScript(() => localStorage.setItem('chaaya-theme', 'light'));
  const otherPage = await other.newPage();
  await loginUi(otherPage, destination, 'nia.s2f@example.com');
  await otherPage.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await otherPage.getByTestId('lookup-reference-input').fill(past.reference);
  await otherPage.getByTestId('lookup-submit').click();
  await otherPage.getByTestId('reservation-error').waitFor();
  if ((await otherPage.getByTestId('reservation-detail').count()) !== 0) fail('non-owner saw the reservation');
  await pass('R160-owner', 'non-owner lookup shows reservation-error');
  await other.close();

  await page.getByTestId('logout-button').click();
  if ((await page.getByTestId('current-user').count()) !== 0) fail('logout left current-user');
  await assertSessionChrome(page, { signedIn: false });
  await pass('R140', 'logout clears the signed-in name and restores auth links');
  await loginUi(page, destination, 'ada.s2f@example.com');

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
  if ((await page.getByTestId('slot-t_1-18:00').count()) !== 0) fail('search B showed search A before A returned');
  const scrollBeforeLate = await settledScroll(page);
  state.hold = null;
  releaseAnker();
  await page.waitForTimeout(700);
  const scrollAfterLate = await settledScroll(page);
  if (Math.abs(scrollAfterLate - scrollBeforeLate) > 40) fail(`late search moved scroll ${scrollBeforeLate} -> ${scrollAfterLate}`);
  if ((await page.locator('h1.display').innerText()) !== 'Nordlicht') fail('late search A replaced the heading');
  if ((await page.getByTestId('slot-t_window-17:00').count()) !== 1) fail('late search A removed B');
  if ((await page.getByTestId('slot-t_1-18:00').count()) !== 0) fail('late search A restored its grid');
  await pass('R125', 'late restaurant detail and availability did not restore search A');

  await knownScroll(page);
  await search(page, 'r_hall', thursday, 2);
  await waitSearch(page);
  await settledScroll(page);
  await assertRevealed(page, 'long-grid', '.grid-caption', 24);
  const longFloor = await readBox(page, '[data-testid="floor-plan"]');
  if (longFloor.bottom <= 0) fail(`long search bypassed the floor bottom=${longFloor.bottom}`);
  await measurePaint(page, 'long-labels');
  const cards = await page.locator('.place-cards').innerText();
  if (!cards.includes('Table Window alcove') || !cards.includes('Table Garden corner') || !cards.includes('Table Hearth booth')) {
    fail(`place cards ${cards}`);
  }
  await page.getByTestId('plan-t_garden').focus();
  await page.keyboard.press('Enter');
  await page.getByTestId('booking-form').waitFor();
  const gardenSummary = await page.getByTestId('booking-summary').innerText();
  if (!gardenSummary.includes('Garden corner')) fail(`keyboard plan summary ${gardenSummary}`);
  await page.getByTestId('plan-t_alcove+t_garden').focus();
  await page.keyboard.press('Enter');
  await page.getByTestId('booking-summary').waitFor();
  const pairPlan = await page.getByTestId('booking-summary').innerText();
  if (!pairPlan.includes('Window alcove') || !pairPlan.includes('Garden corner')) fail(`keyboard pair summary ${pairPlan}`);
  await settledScroll(page);
  await assertRevealed(page, 'long-form', '[data-testid="booking-form"]', 80);
  await page.getByTestId('slot-t_alcove-18:00').click();
  await page.waitForTimeout(800);
  await captureQuartet(page, 'long-labels', 200);
  await measureSettled(page, 'long-labels-1280');
  await page.setViewportSize({ width: 375, height: 812 });
  await page.waitForTimeout(400);
  await measureSettled(page, 'long-labels-375');
  await page.setViewportSize({ width: 1280, height: 900 });
  const longSummary = await page.getByTestId('booking-summary').innerText();
  if (!longSummary.includes('Window alcove')) fail(longSummary);
  await pass('R111-R198', 'long labels captured at both widths');

  const missingLabels = await page.evaluate(() => {
    return [...document.querySelectorAll('input, select, textarea')].filter((element) => {
      if (element.getAttribute('aria-label') || element.getAttribute('aria-labelledby')) return false;
      return !(element.id && document.querySelector(`label[for="${CSS.escape(element.id)}"]`));
    }).length;
  });
  if (missingLabels !== 0) fail(`${missingLabels} controls have no label`);
  const focusStyle = await page.evaluate(() => {
    const button = document.querySelector('[data-testid="search-button"]');
    button.focus({ focusVisible: true });
    const style = getComputedStyle(button);
    return { outline: style.outlineStyle, width: style.outlineWidth, color: style.outlineColor };
  });
  if (focusStyle.outline === 'none' || focusStyle.width === '0px') {
    fail(`focused search control has no visible focus (${focusStyle.outline} ${focusStyle.width})`);
  }
  await pass('R118', 'labels and a visible focus ring are present');

  await finishVideo(context, page, 'main-desktop.webm');
}

async function runPhoneVideo(browser) {
  await reset(destination, pairFixture);
  const state = { reservations: [], dropNextReservation: false, captureReservation: false, forward: '' };
  const { context, page } = await launch(browser, { width: 375, height: 812, video: true });
  await installRouter(page, state);
  await signupUi(page, 'phone.s2f@example.com', 'Ada');
  await assertSessionChrome(page, { signedIn: true, display: 'Ada' });
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  await settledScroll(page);
  const phoneGrid = await assertRevealed(page, 'phone-grid', '.grid-caption', 24);
  const phoneFloor = await readBox(page, '[data-testid="floor-plan"]');
  if (phoneFloor.bottom <= 0) fail(`phone reveal bypassed the floor bottom=${phoneFloor.bottom}`);
  if (phoneGrid.top < 24) fail(`phone reveal pinned the grid heading to ${phoneGrid.top}`);
  await page.getByTestId('slot-t_1+t_2-18:00').click();
  await page.getByTestId('booking-form').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'phone-form', '[data-testid="booking-form"]', 80);
  await page.getByTestId('booking-submit').click();
  await page.getByTestId('confirmation-reference').waitFor();
  await settledScroll(page);
  await assertRevealed(page, 'phone-confirmation', '[data-testid="confirmation"]', 60);
  await page.waitForTimeout(700);
  await assertNoPageOverflow(page, 'phone-confirmed');
  await finishVideo(context, page, 'main-phone.webm');
  await pass('R117-phone', 'phone main flow confirmed without page overflow');
}

async function runReducedVideo(browser) {
  await reset(destination, pairFixture);
  const state = { reservations: [], dropNextReservation: false, captureReservation: false, forward: '' };
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true, reduced: true });
  await installRouter(page, state);
  await signupUi(page, 'motion.s2f@example.com', 'Ada');
  await knownScroll(page);
  await search(page, 'r_anker', thursday, 6);
  await waitSearch(page);
  const reducedGrid = await assertRevealed(page, 'reduced-grid', '.grid-caption', 24);
  if (reducedGrid.scrollY < 0) fail('reduced grid scroll');
  const selected = await clickAndReadSelected(page, 'slot-t_1+t_2-18:00');
  const scale = await page.getByTestId('plan-t_1+t_2').getAttribute('transform');
  if (selected !== 'true') fail(`reduced-motion selection ${selected}`);
  if (!scale?.includes('scale(1.06)')) fail(`reduced-motion badge scale ${scale}`);
  await page.getByTestId('booking-submit').click();
  await page.getByTestId('confirmation-reference').waitFor();
  await finishVideo(context, page, 'main-reduced-motion.webm');
  await pass('R113-R114', 'reduced motion selects at once and still confirms after the response');
}

async function runUpgrade(browser) {
  await reset(source, upgradeFixture);
  const state = {
    reservations: [],
    dropNextReservation: false,
    captureReservation: false,
    forward: '',
    dropped: null,
  };
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true });
  await installRouter(page, state);
  await page.goto(`${source}/signup`);
  await page.getByTestId('signup-email').fill('ada.upgrade@example.com');
  await page.getByTestId('signup-password').fill(SECRET);
  await page.getByTestId('signup-display-name').fill('Ada');
  await page.getByTestId('signup-submit').click();
  await page.getByTestId('current-user').waitFor();
  await page.evaluate(() => {
    window.__s2hMark = 'alive';
  });
  const navBefore = await page.evaluate(() => performance.getEntriesByType('navigation').length);
  await search(page, 'r_anker', thursday, 2);
  await waitSearch(page);
  await page.getByTestId('slot-t_2-19:00').click();
  await page.getByTestId('booking-summary').waitFor();
  const summary = await page.getByTestId('booking-summary').innerText();
  if (!summary.includes('Table 2') || !summary.includes('19:00')) fail(`upgrade summary ${summary}`);
  state.dropNextReservation = true;
  const submitted = page.getByTestId('booking-submit').click();
  await page.getByTestId('booking-uncertain').waitFor();
  await submitted;
  if (!state.dropped || state.dropped.status !== 201 || state.dropped.servedToBrowser !== false) {
    fail(`upgrade source commit ${state.dropped?.status ?? 'missing'} ${errorCode(state.dropped?.text ?? '')}`);
  }
  if ((await page.getByTestId('booking-error').count()) !== 0 || (await page.getByTestId('confirmation').count()) !== 0) {
    fail('upgrade loss showed an error or a confirmation');
  }
  const committed = JSON.parse(state.dropped.text);
  if (!committed.table_id || 'table_ids' in committed) fail('stage-1 receipt was not a singleton without table_ids');
  const sent = JSON.parse(state.dropped.body);
  if (sent.table_id !== 't_2' || 'table_ids' in sent || sent.party_size !== 2 || sent.starts_at_local !== `${thursday}T19:00`) {
    fail('stage-1 pending body was not the original singleton');
  }
  await captureQuartet(page, 'upgrade-uncertain', 200);
  const exported = await fetch(`${source}/_test/export`);
  const exportBytes = Buffer.from(await exported.arrayBuffer());
  if (exported.status !== 200) fail(`export ${exported.status}`);
  const exportJson = JSON.parse(exportBytes.toString('utf8'));
  if (exportJson.track !== 'tablekeeper' || exportJson.format_version !== 1 || !exportJson.state || typeof exportJson.state !== 'object') {
    fail('export envelope was not track/version/state');
  }
  const exportPath = join(privateDir, 'source-export.json');
  await writeFile(exportPath, exportBytes, { mode: 0o600 });
  const imported = await fetch(`${destination}/_test/import`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: exportBytes,
  });
  if (imported.status !== 204) fail(`import ${imported.status}`);
  const mark = await page.evaluate(() => window.__s2hMark);
  const navAfter = await page.evaluate(() => performance.getEntriesByType('navigation').length);
  if (mark !== 'alive' || navAfter !== navBefore || new URL(page.url()).pathname !== '/') {
    fail('upgrade reloaded or left the original page');
  }
  state.forward = destination;
  state.captureReservation = true;
  await page.getByTestId('booking-submit').click();
  await page.getByTestId('confirmation-reference').waitFor();
  const retry = state.reservations.at(-1);
  if (!retry || retry.status !== 200 || retry.cachedBodyReused !== false) fail(`upgrade retry ${retry?.status ?? 'missing'}`);
  if (!retry.fetchedUrl.startsWith(destination)) fail('upgrade retry was not fetched from the destination');
  if (retry.key !== state.dropped.key || !sameJsonText(retry.body, state.dropped.body) || !sameJsonText(retry.text, state.dropped.text)) {
    fail('upgrade retry key, body, or receipt differed');
  }
  const shown = (await page.getByTestId('confirmation-reference').innerText()).trim();
  if (shown !== committed.reference) fail('upgrade confirmation was not the original reference');
  if ((await page.getByTestId('booking-uncertain').count()) !== 0 || (await page.getByTestId('booking-error').count()) !== 0) {
    fail('upgrade recovery left an error state');
  }
  const name = (await page.getByTestId('current-user').innerText()).trim();
  if (!name.includes('Ada')) fail('display name did not survive the upgrade');
  await captureQuartet(page, 'upgrade-confirmed', 500);

  const listed = await page.evaluate(async () => {
    const session = JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}');
    const response = await fetch('/reservations', {
      headers: { Accept: 'application/json', Authorization: `Bearer ${session.token}` },
    });
    const payload = await response.json();
    return {
      status: response.status,
      count: Array.isArray(payload.reservations) ? payload.reservations.length : -1,
      references: (payload.reservations ?? []).map((item) => item.reference),
    };
  });
  if (listed.status !== 200 || listed.count !== 1 || listed.references[0] !== committed.reference) {
    fail(`upgrade list ${listed.status} count ${listed.count}`);
  }
  const token = await page.evaluate(() => JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}').token);
  const direct = await api(destination, `/reservations/${encodeURIComponent(committed.reference)}`, { token });
  const directBody = JSON.parse(direct.text);
  if (direct.status !== 200 || directBody.reference !== committed.reference || directBody.table_id !== 't_2') {
    fail(`destination lookup ${direct.status} ${errorCode(direct.text)}`);
  }
  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Look up' }).click();
  await page.getByTestId('lookup-reference-input').fill(committed.reference);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-detail').waitFor();
  if ((await page.getByTestId('reservation-status').innerText()).trim() !== 'confirmed') fail('upgraded lookup status');
  const still = await page.evaluate(() => window.__s2hMark);
  if (still !== 'alive') fail('lookup navigation reloaded the stage-1 document');
  await captureQuartet(page, 'upgrade-lookup');

  report.upgrade = {
    sourceCommit: state.dropped.status,
    lostResponse: true,
    browserServedCommit: state.dropped.servedToBrowser,
    exportStatus: exported.status,
    exportSha256: sha256(exportBytes),
    exportBytes: exportBytes.length,
    track: exportJson.track,
    formatVersion: exportJson.format_version,
    importStatus: imported.status,
    destinationRetry: retry.status,
    destinationPort: new URL(retry.fetchedUrl).port,
    sameKey: true,
    sameBody: true,
    sameReceipt: true,
    reference: committed.reference,
    receiptHasTableId: true,
    receiptHasTableIds: false,
    oneBooking: listed.count === 1,
    lookupStatus: direct.status,
    displayNameRemained: true,
    reloaded: false,
  };
  await pass('R161-R165', `upgrade ${committed.reference} commit 201 lost export 200 import 204 retry 200`);
  await finishVideo(context, page, 'upgrade-desktop.webm');
  const donor = '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S2-D/donor-final/export.json';
  const donorStat = await stat(donor);
  report.donorExportBytes = donorStat.size;
  await log(`DONOR present bytes=${donorStat.size} not used as the upgrade proof`);
}

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
