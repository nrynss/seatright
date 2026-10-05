/**
 * S4-G2B1 live recurring-clock probe.
 *
 * A real stage-4 process, a real browser booking, and the real series,
 * replan, policy and amend routes. This file does not import another stage,
 * does not call POST /_test/import, and does not build a manager or series UI.
 *
 * The repaired middle table is computed here from the fixture capacities and
 * the closure interval. It is not read from a solver or a replan helper.
 *
 *   node scripts/s4-g2-clock.mjs --destination URL --evidence DIR
 *
 * S4_G2_CLOCK_SABOTAGE=1 forces the expected repaired table to t_1 so the
 * shared assignment check fails. That guard is not a product result.
 *
 * Tokens, receipts, export bodies and idempotency keys stay in private/.
 */
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { chmod, mkdir, writeFile, appendFile, unlink } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { umask } from 'node:process';
import { chromium } from 'playwright';

umask(0o077);

const worktree = '/home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-grok';
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const axePath = fileURLToPath(new URL('../node_modules/axe-core/axe.min.js', import.meta.url));
const SECRET = 'correct horse';
const anchorDate = '2027-06-17';
const middleDate = '2027-06-24';
const laterDate = '2027-07-01';
const dates = [anchorDate, middleDate, laterDate];
const PAIR = ['t_1', 't_2'];
const REPAIRED = ['t_3'];
const closureFrom = `${middleDate}T19:30:00+02:00`;
const closureTo = `${middleDate}T20:00:00+02:00`;
const TERM_KEYS = ['cancellation_cutoff_minutes', 'capacities', 'opening_hours', 'policy_version', 'reservation_duration_minutes', 'slot_minutes'];
const PLAN_KEYS = ['assignments', 'closure', 'moved_count', 'plan_id', 'restaurant_revision', 'unused_seats'];
const hours = [{ weekday: 'thu', opens: '18:00', closes: '23:00' }];
const policy0 = {
  policy_version: 0,
  slot_minutes: 30,
  reservation_duration_minutes: 90,
  cancellation_cutoff_minutes: 120,
  opening_hours: hours,
  capacities: { t_1: 2, t_2: 4, t_3: 6 },
};
const policy1 = {
  policy_version: 1,
  slot_minutes: 30,
  reservation_duration_minutes: 60,
  cancellation_cutoff_minutes: 0,
  opening_hours: hours,
  capacities: { t_1: 4, t_2: 6, t_3: 8 },
};
const humanClock = { '18:30': '6:30 PM', '19:00': '7:00 PM', '20:00': '8:00 PM', '21:00': '9:00 PM' };
const humanDate = {
  '2027-06-17': 'Thursday 17 June 2027',
  '2027-06-24': 'Thursday 24 June 2027',
  '2027-07-01': 'Thursday 1 July 2027',
};
const tableName = { t_1: 'Table 1', t_2: 'Table 2', t_3: 'Table 3' };

const args = process.argv.slice(2);
function arg(name) {
  const index = args.indexOf(`--${name}`);
  return index >= 0 ? args[index + 1] : '';
}
const destination = arg('destination');
const evidence = arg('evidence');
const sabotage = process.env.S4_G2_CLOCK_SABOTAGE === '1';
if (!destination || !evidence) {
  console.error('usage: node scripts/s4-g2-clock.mjs --destination URL --evidence DIR');
  process.exit(2);
}

const shotDir = join(evidence, 'screens');
const videoDir = join(evidence, 'videos');
const privateDir = join(evidence, 'private');
const logPath = join(evidence, 'probe.log');
const startedAt = new Date();
const report = {
  item: 'S4-G2B1',
  shared: 62,
  destination,
  sabotage,
  started: startedAt.toISOString(),
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
  forbiddenPageCalls: [],
  paint: {},
  repair: {},
  clocks: {},
  recovery: {},
  nonClaims: [
    'POST /_test/import was not called',
    'modern stage-4 to stage-4 portability was not claimed',
    'native stage-4 importer acceptance was not claimed',
    'no manager or series UI was added',
    'harness, Gorace and old-document transfer were not run',
    'ends_at is asserted on the parsed record, not as painted text',
    'axe disabled color-contrast and region; accessibility is not claimed exhaustive',
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

function same(left, right) {
  return JSON.stringify(canonical(left)) === JSON.stringify(canonical(right));
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

function randomKey() {
  return [...crypto.getRandomValues(new Uint8Array(16))].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

function clockFixture() {
  return {
    users: [
      { id: 'u_mgr', email: 'mira.manager@example.com', password: SECRET, display_name: 'Mira' },
      { id: 'u_diner', email: 'ada.clock@example.com', password: SECRET, display_name: 'Ada' },
    ],
    restaurants: [
      {
        id: 'r_anker',
        name: 'Zum Anker',
        timezone: 'Europe/Berlin',
        slot_minutes: 30,
        reservation_duration_minutes: 90,
        cancellation_cutoff_minutes: 120,
        opening_hours: hours,
        tables: [
          { id: 't_1', label: '1', capacity: 2 },
          { id: 't_2', label: '2', capacity: 4 },
          { id: 't_3', label: '3', capacity: 6 },
        ],
        combinable: [['t_1', 't_2'], ['t_2', 't_3']],
        manager_user_ids: ['u_mgr'],
      },
    ],
    reservations: [],
  };
}

function policyBody() {
  return {
    effective_from: middleDate,
    slot_minutes: 30,
    reservation_duration_minutes: 60,
    cancellation_cutoff_minutes: 0,
    opening_hours: hours,
    capacities: { t_1: 4, t_2: 6, t_3: 8 },
  };
}

function phrase(ids) {
  return ids.map((id) => tableName[id]).join(' and ');
}

function instant(date, hhmm) {
  return `${date}T${hhmm}:00+02:00`;
}

function plusMinutes(date, hhmm, minutes) {
  const [hour, minute] = hhmm.split(':').map(Number);
  const total = hour * 60 + minute + minutes;
  const nextHour = String(Math.floor(total / 60)).padStart(2, '0');
  const nextMinute = String(total % 60).padStart(2, '0');
  return instant(date, `${nextHour}:${nextMinute}`);
}

function elapsedMinutes(starts, ends) {
  const delta = Date.parse(ends) - Date.parse(starts);
  return Number.isFinite(delta) ? delta / 60000 : null;
}

function termsOk(actual, expected) {
  if (!actual || typeof actual !== 'object') return false;
  if (JSON.stringify(Object.keys(actual).sort()) !== JSON.stringify(TERM_KEYS)) return false;
  return same(actual, expected);
}

async function writePrivate(name, bytes) {
  const file = join(privateDir, name);
  await writeFile(file, bytes);
  await chmod(file, 0o600);
  return file;
}

async function api(path, { method = 'GET', token = '', key = '', body = undefined, rawBody = undefined } = {}) {
  const headers = { Accept: 'application/json' };
  if (body !== undefined || rawBody !== undefined) headers['Content-Type'] = 'application/json; charset=utf-8';
  if (token) headers.Authorization = `Bearer ${token}`;
  if (key) headers['Idempotency-Key'] = key;
  const response = await fetch(`${destination}${path}`, {
    method,
    headers,
    body: rawBody !== undefined ? rawBody : body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await response.text();
  return { status: response.status, text, contentType: response.headers.get('content-type') ?? '' };
}

async function reset() {
  const response = await fetch(`${destination}/_test/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify(clockFixture()),
  });
  if (response.status !== 204) fail(`reset status ${response.status}`);
}

async function exportRaw() {
  const result = await api('/_test/export');
  if (result.status !== 200) fail(`export ${result.status} ${errorCode(result.text)}`);
  return result.text;
}

function stateOf(text) {
  return JSON.parse(text).state ?? {};
}

async function login(email) {
  const result = await api('/auth/login', { method: 'POST', body: { email, password: SECRET } });
  if (result.status !== 200) fail(`login ${result.status} ${errorCode(result.text)}`);
  const value = JSON.parse(result.text);
  if (!value.token) fail('login token missing');
  return value;
}

async function ownerGet(token, reference) {
  const result = await api(`/reservations/${encodeURIComponent(reference)}`, { token });
  if (result.status !== 200) fail(`reservation ${result.status} ${errorCode(result.text)}`);
  return { result, value: JSON.parse(result.text) };
}

async function historyOf(token, reference) {
  const result = await api(`/reservations/${encodeURIComponent(reference)}/history`, { token });
  if (result.status !== 200) fail(`history ${result.status} ${errorCode(result.text)}`);
  return JSON.parse(result.text);
}

async function decisionOf(token, reference) {
  const result = await api(`/reservations/${encodeURIComponent(reference)}/decision`, { token });
  if (result.status !== 200) fail(`decision ${result.status} ${errorCode(result.text)}`);
  return JSON.parse(result.text);
}

async function seriesOf(token, seriesId) {
  const result = await api(`/series/${encodeURIComponent(seriesId)}`, { token });
  if (result.status !== 200) fail(`series ${result.status} ${errorCode(result.text)}`);
  return { result, value: JSON.parse(result.text) };
}

function occurrence(series, reference) {
  return (series.occurrences ?? []).find((item) => item.reference === reference) ?? null;
}

async function preview(token, key) {
  return api('/restaurants/r_anker/replans', {
    method: 'POST',
    token,
    key,
    body: { table_id: 't_2', from: closureFrom, to: closureTo },
  });
}

async function applyPlan(token, planId, key) {
  return api(`/restaurants/r_anker/replans/${encodeURIComponent(planId)}/apply`, {
    method: 'POST',
    token,
    key,
    rawBody: '{}',
  });
}

async function adopt(token, reference, count, key) {
  const rawBody = JSON.stringify({ anchor_reference: reference, count, interval_weeks: 1 });
  const result = await api('/series', { method: 'POST', token, key, rawBody });
  return { result, rawBody, key };
}

async function amend(token, seriesId, revision, fromIndex, localTime, key) {
  const rawBody = JSON.stringify({ expected_revision: revision, from_index: fromIndex, local_time: localTime });
  const result = await api(`/series/${encodeURIComponent(seriesId)}/amend`, { method: 'POST', token, key, rawBody });
  return { result, rawBody, key };
}

function digestEvents(history) {
  return (history.entries ?? []).map((entry) => entry.event).join(',');
}

function changeDigest(entry) {
  return (entry.changes ?? []).map((change) => `${change.field}:${JSON.stringify(change.from)}->${JSON.stringify(change.to)}`).join('|');
}

async function replay(token, path, key, rawBody, originalText, label) {
  const before = await exportRaw();
  const result = await api(path, { method: 'POST', token, key, rawBody });
  await expectCheck(`${label}-replay-status`, result.status === 200, `status ${result.status} ${errorCode(result.text)}`);
  await expectCheck(`${label}-replay-bytes`, result.text === originalText, `sha ${sha256(result.text)} original ${sha256(originalText)} bytes ${result.text.length}/${originalText.length}`);
  const after = await exportRaw();
  await expectCheck(`${label}-replay-export`, after === before, `export sha ${sha256(before)} vs ${sha256(after)}`);
}

function noteUrl(url) {
  if (url.hostname !== '127.0.0.1' && url.hostname !== 'localhost') report.offOrigin.push(`${url.origin}${url.pathname}`);
}

function isApi(pathname) {
  return pathname === '/health' || pathname.startsWith('/restaurants') || pathname.startsWith('/availability') || pathname.startsWith('/reservations') || pathname.startsWith('/auth/') || pathname.startsWith('/reservation-moves') || pathname.startsWith('/series') || pathname.startsWith('/_test/');
}

async function installRouter(page, state) {
  await page.route('**/*', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    noteUrl(url);
    if (isApi(url.pathname)) state.apiPaths.push(`${request.method()} ${url.pathname}${url.search}`);
    if (state.dropNextReservation && request.method() === 'POST' && url.pathname === '/reservations') {
      state.dropNextReservation = false;
      const fetched = await route.fetch();
      const fresh = await fetched.text();
      state.dropped = {
        status: fetched.status(),
        text: fresh,
        key: request.headers()['idempotency-key'] ?? '',
        body: request.postData() ?? '',
        method: request.method(),
        path: url.pathname,
      };
      await route.abort('failed');
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
        method: request.method(),
        path: url.pathname,
      });
      await route.fulfill({ status: fetched.status(), headers, body: fresh });
      return;
    }
    await route.continue();
  });
}

function freshState() {
  return { apiPaths: [], reservations: [], dropped: null, dropNextReservation: false, captureReservation: false };
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
  page.setDefaultTimeout(20000);
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

async function loginUi(page) {
  await page.goto(`${destination}/login`);
  await page.getByTestId('login-email').fill('ada.clock@example.com');
  await page.getByTestId('login-password').fill(SECRET);
  await page.getByTestId('login-submit').click();
  await page.getByTestId('current-user').waitFor();
  const name = (await page.getByTestId('current-user').innerText()).trim();
  if (name !== 'Ada') fail(`current-user ${name}`);
}

async function search(page, date, party) {
  const select = page.getByTestId('restaurant-select');
  await select.waitFor();
  await select.locator('option[value="r_anker"]').waitFor({ state: 'attached' });
  await select.selectOption('r_anker');
  await page.getByTestId('date-input').fill(date);
  await page.getByTestId('party-size-input').fill(String(party));
  await page.getByTestId('search-button').click();
  await page.waitForFunction(() => document.querySelector('[data-testid="availability-grid"], [data-testid="no-slots"], [data-testid="search-error"]'));
}

async function sessionToken(page) {
  const token = await page.evaluate(() => JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}').token ?? '');
  if (!token) fail('session token missing');
  return token;
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
  if (overflow) fail(`${label} horizontal overflow ${box.scrollWidth}>${box.clientWidth}`);
}

async function readBox(page, selector) {
  return page.evaluate((sel) => {
    const node = document.querySelector(sel);
    const viewH = window.innerHeight;
    if (!node) return { present: false, scrollY: window.scrollY, viewH, visible: 0, top: null };
    const rect = node.getBoundingClientRect();
    const visible = Math.max(0, Math.min(rect.bottom, viewH) - Math.max(rect.top, 0));
    return { present: true, scrollY: window.scrollY, viewH, top: Math.round(rect.top * 10) / 10, visible: Math.round(visible * 10) / 10 };
  }, selector);
}

async function waitRevealed(page, label, selector) {
  let box = await readBox(page, selector);
  const started = Date.now();
  while (Date.now() - started < 1600 && (!box.present || box.visible < 24 || box.top >= box.viewH - 8)) {
    await page.waitForTimeout(50);
    box = await readBox(page, selector);
  }
  report.reveal.push({ label, ...box });
  await log(`REVEAL ${label} present=${box.present} scrollY=${box.scrollY} top=${box.top} visible=${box.visible}`);
  if (!box.present || box.visible < 24 || box.top >= box.viewH - 8) fail(`${label} not revealed visible=${box.visible} top=${box.top}`);
}

async function capturePair(page, name) {
  for (const width of [1280, 375]) {
    await page.setViewportSize({ width, height: width === 375 ? 812 : 900 });
    for (const theme of ['light', 'dark']) {
      await page.getByTestId(`theme-${theme}`).click();
      await page.waitForTimeout(150);
      const file = join(shotDir, `${name}-${width}-${theme}.png`);
      await page.screenshot({ path: file, fullPage: true });
      report.shots.push(file);
      await assertNoPageOverflow(page, `${name}-${width}-${theme}`);
    }
  }
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.getByTestId('theme-light').click();
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
      available: cell.getAttribute('data-available'),
      selected: cell.getAttribute('data-selected'),
    };
  }, testId);
}

async function measureSelectedContrast(page) {
  const saved = page.viewportSize();
  const samples = [];
  for (const width of [1280, 375]) {
    await page.setViewportSize({ width, height: width === 375 ? 812 : 900 });
    for (const theme of ['light', 'dark']) {
      await page.getByTestId(`theme-${theme}`).click();
      await page.waitForTimeout(120);
      const paint = await readPaint(page, 'slot-t_1+t_2-19:00');
      if (!paint) fail('selected pair cell missing');
      const ratio = contrastRatio(paint.color, paint.background);
      const row = {
        width,
        theme,
        word: paint.word,
        available: paint.available,
        selected: paint.selected,
        ratio: ratio === null ? null : Math.round(ratio * 1000) / 1000,
      };
      samples.push(row);
      report.renderedContrast.push(row);
      await log(`CONTRAST selected-pair ${width} ${theme} ${row.word} ${row.ratio}:1 available=${row.available} selected=${row.selected}`);
    }
  }
  await page.setViewportSize(saved ?? { width: 1280, height: 900 });
  await page.getByTestId('theme-light').click();
  const bad = samples.filter((row) => row.word !== 'Held' || row.available !== 'true' || row.selected !== 'true' || row.ratio === null || row.ratio < 4.5);
  await expectCheck('R118-selected-contrast', bad.length === 0, samples.map((row) => `${row.width}/${row.theme} ${row.ratio}`).join(' '));
}

async function runAxe(page, label) {
  await page.addScriptTag({ path: axePath });
  const violations = await page.evaluate(async () => {
    const results = await window.axe.run(document, {
      resultTypes: ['violations'],
      rules: { 'color-contrast': { enabled: false }, region: { enabled: false } },
    });
    return results.violations
      .filter((item) => item.impact === 'serious' || item.impact === 'critical')
      .map((item) => ({ id: item.id, impact: item.impact, nodes: item.nodes.length }));
  });
  report.axe.push({ label, violations, disabledRules: ['color-contrast', 'region'], exhaustive: false });
  if (violations.length) fail(`${label} axe ${violations.map((item) => item.id).join(',')}`);
}

async function assertLabels(page, ids, checkId) {
  const rows = await page.evaluate((names) => names.map((id) => {
    const field = document.getElementById(id);
    const label = field ? document.querySelector(`label[for="${id}"]`) : null;
    return { id, labelled: Boolean(label && (label.textContent || '').trim()), text: (label?.textContent || '').trim() };
  }), ids);
  const missing = rows.filter((row) => !row.labelled).map((row) => row.id);
  await expectCheck(checkId, missing.length === 0, rows.map((row) => `${row.id}:${row.text}`).join(' '));
}

async function assertKeyboardFocus(page, testId) {
  await page.evaluate(() => document.querySelector('.skip')?.focus());
  for (let step = 0; step < 24; step += 1) {
    const current = await page.evaluate(() => document.activeElement?.getAttribute('data-testid') ?? '');
    if (current === testId) break;
    await page.keyboard.press('Tab');
  }
  const focus = await page.evaluate(() => {
    const el = document.activeElement;
    const style = getComputedStyle(el);
    return {
      testid: el?.getAttribute('data-testid') ?? '',
      outlineStyle: style.outlineStyle,
      outlineWidth: style.outlineWidth,
    };
  });
  const visible = focus.testid === testId && focus.outlineStyle !== 'none' && focus.outlineWidth !== '0px';
  await expectCheck('R118-focus', visible, `${focus.testid} outline ${focus.outlineStyle} ${focus.outlineWidth}`);
}

async function lookup(page, reference) {
  await page.goto(`${destination}/lookup`);
  await page.getByTestId('lookup-reference-input').fill(reference);
  await page.getByTestId('lookup-submit').click();
  await page.getByTestId('reservation-detail').waitFor();
}

async function readLookup(page) {
  const status = (await page.getByTestId('reservation-status').innerText()).trim();
  const tables = (await page.getByTestId('reservation-tables').innerText()).trim();
  const detail = (await page.getByTestId('reservation-detail').innerText()).trim();
  const cancel = await page.getByTestId('reservation-cancel-button').count();
  return { status, tables, detail, cancel };
}

function paintsClock(detail, date, clock) {
  return detail.includes(humanDate[date]) && detail.includes(humanClock[clock]) && detail.includes(`(${clock})`);
}

async function assertLookup(page, token, reference, { status, ids, date, clock, label }) {
  const record = await ownerGet(token, reference);
  const view = await readLookup(page);
  const ends = record.value.ends_at ?? '';
  const paintedEnd = ends && view.detail.includes(ends);
  const ok = view.status === status
    && view.status === record.value.status
    && view.tables === phrase(ids)
    && JSON.stringify(record.value.table_ids) === JSON.stringify(ids)
    && record.value.starts_at_local === `${date}T${clock}`
    && paintsClock(view.detail, date, clock)
    && !paintedEnd
    && view.cancel === (status === 'confirmed' ? 1 : 0);
  await expectCheck(label, ok, `status ${view.status} tables ${view.tables} cancel ${view.cancel} clock ${record.value.starts_at_local} paintedEnd ${paintedEnd}`);
  return { view, record: record.value };
}

async function mirrorGrid(page) {
  const result = await api(`/availability?restaurant_id=r_anker&date=${anchorDate}&party_size=6`);
  if (result.status !== 200) fail(`availability ${result.status} ${errorCode(result.text)}`);
  const body = JSON.parse(result.text);
  const mismatches = [];
  for (const slot of body.slots ?? []) {
    const time = slot.starts_at_local.slice(11, 16);
    const open = new Set(slot.available_table_ids ?? []);
    for (const id of ['t_1', 't_2', 't_3']) {
      const got = await page.getByTestId(`slot-${id}-${time}`).getAttribute('data-available');
      const want = open.has(id) ? 'true' : 'false';
      if (got !== want) mismatches.push(`slot-${id}-${time}`);
    }
    for (const key of ['t_1+t_2', 't_2+t_3']) {
      const eligible = (slot.available_options ?? []).some((option) => (option.table_ids ?? []).join('+') === key);
      const got = await page.getByTestId(`slot-${key}-${time}`).getAttribute('data-available');
      if (got !== (eligible ? 'true' : 'false')) mismatches.push(`slot-${key}-${time}`);
    }
  }
  await expectCheck('R146-R187-grid', mismatches.length === 0, mismatches.slice(0, 8).join(',') || `${(body.slots ?? []).length} slots`);
}

function forbid(state, label) {
  const hits = state.apiPaths.filter((line) => line.includes('/series') || line.includes('/replans') || line.includes('explain='));
  report.forbiddenPageCalls.push({ label, hits });
  if (hits.length) fail(`${label} diner called ${hits.join(' ')}`);
}

async function assertRecordShape(value, { ids, date, clock, terms, party = 6, status = 'confirmed' }) {
  const singleton = ids.length === 1;
  const elapsed = elapsedMinutes(value.starts_at, value.ends_at);
  const problems = [];
  if (value.starts_at_local !== `${date}T${clock}`) problems.push('clock');
  if (value.starts_at !== instant(date, clock)) problems.push('starts');
  if (JSON.stringify(value.table_ids) !== JSON.stringify(ids)) problems.push('tables');
  if (singleton !== Object.prototype.hasOwnProperty.call(value, 'table_id')) problems.push('scalar');
  if (singleton && value.table_id !== ids[0]) problems.push('table_id');
  if (value.party_size !== party || value.status !== status) problems.push('party-or-status');
  if (!termsOk(value.accepted_terms, terms)) problems.push('terms');
  if (elapsed !== terms.reservation_duration_minutes) problems.push(`elapsed ${elapsed}`);
  if (value.ends_at !== plusMinutes(date, clock, terms.reservation_duration_minutes)) problems.push('ends');
  return problems;
}

async function assertPreviewAssignment(plan, middleRef) {
  const keys = Object.keys(plan).sort();
  await expectCheck('preview-six-keys', JSON.stringify(keys) === JSON.stringify(PLAN_KEYS), keys.join(','));
  await expectCheck('preview-counts', plan.moved_count === 1 && plan.unused_seats === 0, `moved ${plan.moved_count} unused ${plan.unused_seats}`);
  await expectCheck('preview-closure', plan.closure?.table_id === 't_2' && plan.closure?.from === closureFrom && plan.closure?.to === closureTo, 'closure window');
  const rows = Array.isArray(plan.assignments) ? plan.assignments : [];
  await expectCheck('preview-one-middle', rows.length === 1 && rows[0].reference === middleRef && rows[0].changed === true, `rows ${rows.length}`);
  // Hand count: party 6, accepted caps 2/4/6, t_2 closed on the middle evening only.
  // t_1 is too small, t_2 is closed and too small, both declared pairs include t_2.
  // t_3 capacity 6 is the only feasible table. Unused seats are 6 - 6.
  const expected = sabotage ? ['t_1'] : REPAIRED;
  const got = rows[0]?.table_ids ?? [];
  report.repair = { expected, got, moved: plan.moved_count, unused: plan.unused_seats, sabotage };
  await expectCheck('R377-preview-t3', JSON.stringify(got) === JSON.stringify(expected), `got ${got.join('+')} expected ${expected.join('+')}`);
}

async function runReduced(browser) {
  const state = freshState();
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true, reduced: true });
  await installRouter(page, state);
  await loginUi(page);
  await page.goto(`${destination}/`);
  await search(page, anchorDate, 6);
  const before = await page.getByTestId('booking-form').count();
  await page.getByTestId('slot-t_1-19:00').click();
  await page.waitForTimeout(80);
  if ((await page.getByTestId('booking-form').count()) !== before) fail('unavailable cell opened a form');
  const cell = page.getByTestId('slot-t_1+t_2-19:00');
  await cell.click();
  const selected = await cell.getAttribute('data-selected');
  const running = await cell.evaluate((element) => element.getAnimations({ subtree: true }).filter((item) => item.playState === 'running').length);
  const documentRunning = await page.evaluate(() => document.getAnimations().filter((item) => item.playState === 'running').length);
  report.reduced = { selected, running, documentRunning };
  await expectCheck('R114-reduced-selected', selected === 'true' && running === 0 && documentRunning === 0, `selected ${selected} cell ${running} document ${documentRunning}`);
  await page.getByTestId('booking-form').waitFor();
  await assertNoPageOverflow(page, 'reduced-1280');
  await page.setViewportSize({ width: 375, height: 812 });
  await assertNoPageOverflow(page, 'reduced-375');
  await page.setViewportSize({ width: 1280, height: 900 });
  await cell.evaluate((element) => element.scrollIntoView({ block: 'center', behavior: 'instant' }));
  await page.waitForTimeout(1600);
  forbid(state, 'reduced');
  await finishVideo(context, page, 'reduced-select.webm');
}

async function bookPair(page, state) {
  state.captureReservation = true;
  const before = state.reservations.length;
  await page.getByTestId('booking-submit').click();
  const started = Date.now();
  while (state.reservations.length === before && Date.now() - started < 12000) await page.waitForTimeout(40);
  if (state.reservations.length === before) fail('booking request was not observed');
  return state.reservations[state.reservations.length - 1];
}

async function runPass(browser) {
  await reset();
  const manager = await login('mira.manager@example.com');
  await writePrivate('manager.token', manager.token);
  if (manager.user_id !== 'u_mgr' || manager.display_name !== 'Mira') fail('manager identity');

  await runReduced(browser);

  const state = freshState();
  const main = await launch(browser, { width: 1280, height: 900, video: true });
  await installRouter(main.page, state);
  await loginUi(main.page);
  const diner = await sessionToken(main.page);
  await writePrivate('diner.token', diner);
  await main.page.goto(`${destination}/`);
  await main.page.getByTestId('restaurant-select').waitFor();
  const nav = await main.page.getByRole('navigation', { name: 'Primary' }).innerText();
  await expectCheck('R124-R139-nav', nav.includes('Search') && nav.includes('Look up') && !nav.includes('Sign in') && (await main.page.getByTestId('current-user').innerText()).trim() === 'Ada', 'signed-in nav');
  await assertLabels(main.page, ['restaurant-select', 'date-input', 'party-size-input'], 'R118-search-labels');
  await search(main.page, anchorDate, 6);
  if ((await main.page.getByTestId('party-size-input').inputValue()) !== '6') fail('party input');
  if ((await main.page.getByTestId('floor-plan').count()) !== 1) fail('floor missing');
  await mirrorGrid(main.page);
  const formBefore = await main.page.getByTestId('booking-form').count();
  await main.page.getByTestId('slot-t_1-19:00').click();
  await main.page.waitForTimeout(80);
  await expectCheck('R148-unavailable', (await main.page.getByTestId('booking-form').count()) === formBefore, 'unavailable click');
  const selected = main.page.getByTestId('slot-t_1+t_2-19:00');
  await selected.click();
  await main.page.getByTestId('booking-form').waitFor();
  if ((await selected.getAttribute('data-selected')) !== 'true') fail('pair was not selected');
  if ((await main.page.getByTestId('booking-party-size').inputValue()) !== '6') fail('party was not prefilled');
  const summary = (await main.page.getByTestId('booking-summary').innerText()).trim();
  const summaryOk = summary.includes('Zum Anker') && summary.includes('Table 1') && summary.includes('Table 2') && summary.includes('7:00 PM') && summary.includes('(19:00)') && summary.includes(humanDate[anchorDate]);
  await expectCheck('R151-R188-summary', summaryOk, 'pair summary');
  await measureSelectedContrast(main.page);
  const captured = await bookPair(main.page, state);
  await expectCheck('R61-create-201', captured.status === 201, `status ${captured.status} ${errorCode(captured.text)}`);
  const created = JSON.parse(captured.text);
  const fetchedCreate = await api(`/reservations/${encodeURIComponent(created.reference)}`, { token: diner });
  await expectCheck('R18-reservation-charset', fetchedCreate.status === 200 && fetchedCreate.contentType === 'application/json; charset=utf-8', fetchedCreate.contentType || 'missing');
  const createProblems = await assertRecordShape(created, { ids: PAIR, date: anchorDate, clock: '19:00', terms: policy0 });
  await expectCheck('R180-R242-R243-create', createProblems.length === 0, createProblems.join(',') || 'pair policy 0 90m');
  await main.page.getByTestId('confirmation-reference').waitFor();
  const paintedRef = (await main.page.getByTestId('confirmation-reference').innerText()).trim();
  const paintedTables = (await main.page.getByTestId('confirmation-tables').innerText()).trim();
  const paintedDetails = (await main.page.getByTestId('confirmation-details').innerText()).trim();
  report.paint.confirmation = { reference: paintedRef, tables: paintedTables, details: paintedDetails };
  await expectCheck('R155-reference', paintedRef === created.reference && paintedRef === paintedRef.trim(), 'confirmation reference');
  await expectCheck('R156-R189-confirmation', paintedTables === phrase(PAIR) && paintsClock(paintedDetails, anchorDate, '19:00') && paintedDetails.includes('Zum Anker') && !paintedDetails.includes(created.ends_at) && !paintedDetails.includes('8:30 PM'), 'original 19:00 pair');
  await waitRevealed(main.page, 'confirmation', '[data-testid="confirmation"]');
  await capturePair(main.page, 'confirmation-original');
  await runAxe(main.page, 'confirmation');
  await writePrivate('create-201.json', captured.text);
  await writePrivate('create-request.json', captured.body);
  await writePrivate('create.key', captured.key);
  const anchorRef = created.reference;
  report.paint.anchorReference = anchorRef;

  const adopted = await adopt(diner, anchorRef, 3, randomKey());
  await expectCheck('R272-adopt', adopted.result.status === 201, `status ${adopted.result.status} ${errorCode(adopted.result.text)}`);
  const series = JSON.parse(adopted.result.text);
  const seriesId = series.series_id;
  const refs = (series.occurrences ?? []).map((item) => item.reference);
  await expectCheck('adopt-shape', refs.length === 3 && refs[0] === anchorRef && series.revision === 1 && series.interval_weeks === 1, `revision ${series.revision} count ${refs.length}`);
  const anchorAfterAdopt = await ownerGet(diner, anchorRef);
  await expectCheck('R264-anchor-identity', anchorAfterAdopt.result.text.includes(created.reservation_id) && anchorAfterAdopt.value.revision === 1 && anchorAfterAdopt.value.starts_at_local === `${anchorDate}T19:00` && anchorAfterAdopt.value.created_at === created.created_at, 'anchor identity held');
  const beforeRepairExport = await exportRaw();
  await writePrivate('export-before-repair.json', beforeRepairExport);
  const beforeState = stateOf(beforeRepairExport);
  const members = beforeState.series?.[seriesId]?.members ?? [];
  const memberDates = members.map((item) => item.scheduled_date);
  const memberFlags = members.map((item) => item.exception);
  await expectCheck('R265-scheduled-dates', same(memberDates, dates) && memberFlags.every((flag) => flag === false) && members.map((item) => item.reference).join(',') === refs.join(','), memberDates.join(','));
  await writePrivate('adopt-201.json', adopted.result.text);
  await writePrivate('adopt-request.json', adopted.rawBody);
  await writePrivate('adopt.key', adopted.key);
  const middleRef = refs[1];
  const laterRef = refs[2];
  report.paint.references = { anchor: anchorRef, middle: middleRef, later: laterRef };
  for (const ref of refs) {
    const history = await historyOf(diner, ref);
    await expectCheck(`history-created-${refs.indexOf(ref)}`, digestEvents(history) === 'created' && history.entries[0].changes.map((change) => change.field).join(',') === 'table_ids,starts_at_local,party_size', digestEvents(history));
  }
  const currentSeries = await seriesOf(diner, seriesId);
  await writePrivate('series-before-repair.json', currentSeries.result.text);

  const dinerPreview = await preview(diner, randomKey());
  await expectCheck('diner-cannot-replan', dinerPreview.status === 403 && errorCode(dinerPreview.text) === 'forbidden', `status ${dinerPreview.status} ${errorCode(dinerPreview.text)}`);
  const managerSeries = await api(`/series/${encodeURIComponent(seriesId)}`, { token: manager.token });
  await expectCheck('manager-cannot-read-series', managerSeries.status === 404 && errorCode(managerSeries.text) === 'not_found', `status ${managerSeries.status} ${errorCode(managerSeries.text)}`);

  const prePreview = await exportRaw();
  const previewKey = randomKey();
  const previewResult = await preview(manager.token, previewKey);
  await expectCheck('preview-201', previewResult.status === 201, `status ${previewResult.status} ${errorCode(previewResult.text)}`);
  const plan = JSON.parse(previewResult.text);
  const postPreview = await exportRaw();
  const preView = stateOf(prePreview);
  const postView = stateOf(postPreview);
  const previewStable = same(preView.reservations, postView.reservations) && same(preView.histories, postView.histories) && same(preView.series, postView.series) && same(preView.restaurant_revisions, postView.restaurant_revisions) && same(preView.closures, postView.closures);
  await expectCheck('preview-stores-plan-only', previewStable, `export reservations stable ${previewStable}`);
  await assertPreviewAssignment(plan, middleRef);
  await writePrivate('preview-201.json', previewResult.text);
  const planId = plan.plan_id;

  const applyKey = randomKey();
  const applied = await applyPlan(manager.token, planId, applyKey);
  await expectCheck('apply-201', applied.status === 201, `status ${applied.status} ${errorCode(applied.text)}`);
  const applyBody = JSON.parse(applied.text);
  const moved = (applyBody.reservations ?? [])[0] ?? {};
  const movedProblems = await assertRecordShape(moved, { ids: REPAIRED, date: middleDate, clock: '19:00', terms: policy0 });
  await expectCheck('R377-apply-frozen-clock', (applyBody.reservations ?? []).length === 1 && moved.reference === middleRef && moved.revision === 2 && moved.reservation_id && movedProblems.length === 0, movedProblems.join(',') || 't3 19:00 policy 0');
  const middleHistory = await historyOf(diner, middleRef);
  const reassigned = middleHistory.entries?.[1] ?? {};
  const reassignedOk = digestEvents(middleHistory) === 'created,reassigned'
    && reassigned.plan_id === planId
    && reassigned.revision === 2
    && changeDigest(reassigned) === `table_ids:${JSON.stringify(PAIR)}->${JSON.stringify(REPAIRED)}`
    && termsOk(middleHistory.entries[0].accepted_terms, policy0)
    && termsOk(reassigned.accepted_terms, policy0);
  await expectCheck('R377-reassigned-history', reassignedOk, digestEvents(middleHistory));
  const afterApply = stateOf(await exportRaw());
  await expectCheck('R378-series-once', afterApply.series?.[seriesId]?.revision === 2 && afterApply.restaurant_revisions?.r_anker === 3, `series ${afterApply.series?.[seriesId]?.revision} restaurant ${afterApply.restaurant_revisions?.r_anker}`);
  const flagsAfter = (afterApply.series?.[seriesId]?.members ?? []).map((item) => item.exception);
  const datesAfter = (afterApply.series?.[seriesId]?.members ?? []).map((item) => item.scheduled_date);
  await expectCheck('R377-flags-dates', flagsAfter.every((flag) => flag === false) && same(datesAfter, dates), datesAfter.join(','));
  await writePrivate('apply-201.json', applied.text);
  await writePrivate('apply.key', applyKey);

  await lookup(main.page, middleRef);
  await assertLookup(main.page, diner, middleRef, { status: 'confirmed', ids: REPAIRED, date: middleDate, clock: '19:00', label: 'lookup-middle-before' });
  await waitRevealed(main.page, 'lookup-before', '[data-testid="reservation-detail"]');
  await capturePair(main.page, 'lookup-middle-before');
  await assertKeyboardFocus(main.page, 'lookup-reference-input');
  await assertLabels(main.page, ['lookup-reference-input'], 'R118-lookup-labels');

  const policyKey = randomKey();
  const published = await api('/restaurants/r_anker/policies', { method: 'POST', token: manager.token, key: policyKey, body: policyBody() });
  await expectCheck('policy-201', published.status === 201, `status ${published.status} ${errorCode(published.text)}`);
  const publishedBody = JSON.parse(published.text);
  await expectCheck('policy-version-1', publishedBody.policy_version === 1 && publishedBody.reservation_duration_minutes === 60 && publishedBody.effective_from === middleDate, `version ${publishedBody.policy_version}`);
  const anchorStill = await ownerGet(diner, anchorRef);
  const middleStill = await ownerGet(diner, middleRef);
  await expectCheck('R246-publication-not-retroactive', termsOk(anchorStill.value.accepted_terms, policy0) && termsOk(middleStill.value.accepted_terms, policy0) && anchorStill.value.ends_at === created.ends_at && middleStill.value.starts_at_local === `${middleDate}T19:00`, 'existing bookings held policy 0');

  const beforeStale = await exportRaw();
  const stale = await amend(diner, seriesId, 1, 0, '20:00', randomKey());
  const afterStale = await exportRaw();
  await expectCheck('R356-stale-revision', stale.result.status === 409 && errorCode(stale.result.text) === 'stale_revision', `status ${stale.result.status} ${errorCode(stale.result.text)}`);
  await expectCheck('R356-stale-no-mutation', afterStale === beforeStale, `export sha ${sha256(beforeStale)} ${sha256(afterStale)}`);

  const decisions = [];
  for (const ref of refs) decisions.push(await decisionOf(diner, ref));
  const cutoffs = decisions.map((item) => item.accepted_terms?.cancellation_cutoff_minutes);
  await expectCheck('R362-open-cutoff', cutoffs.every((value) => value === 120), `cutoffs ${cutoffs.join(',')}`);
  const amendKey = randomKey();
  const amended = await amend(diner, seriesId, 2, 0, '20:00', amendKey);
  await expectCheck('R370-amend-201', amended.result.status === 201 && errorCode(amended.result.text) !== 'cutoff_passed', `status ${amended.result.status} ${errorCode(amended.result.text)}`);
  const amendedSeries = JSON.parse(amended.result.text);
  await writePrivate('amend-201.json', amended.result.text);
  await writePrivate('amend-request.json', amended.rawBody);
  await writePrivate('amend.key', amendKey);
  await expectCheck('amend-series-revision', amendedSeries.revision === 3 && amendedSeries.series_id === seriesId, `revision ${amendedSeries.revision}`);

  const wantTerms = [policy0, policy1, policy1];
  const wantElapsed = [90, 60, 60];
  for (let index = 0; index < 3; index += 1) {
    const occ = occurrence(amendedSeries, refs[index]);
    const record = occ?.reservation ?? {};
    const ids = index === 1 ? REPAIRED : PAIR;
    const problems = await assertRecordShape(record, { ids, date: dates[index], clock: '20:00', terms: wantTerms[index] });
    if (occ?.exception !== false) problems.push('exception');
    if (record.reference !== refs[index]) problems.push('reference');
    if (record.party_size !== 6) problems.push('party');
    await expectCheck(`R359-R360-R363-R374-occ-${index}`, problems.length === 0, problems.join(',') || `${dates[index]} 20:00 ${wantElapsed[index]}m`);
  }
  const afterAmend = stateOf(await exportRaw());
  await expectCheck('R371-R372-R373-once', afterAmend.series?.[seriesId]?.revision === 3 && afterAmend.restaurant_revisions?.r_anker === 5 && afterAmend.reservations?.[anchorRef]?.revision === 2 && afterAmend.reservations?.[middleRef]?.revision === 3 && afterAmend.reservations?.[laterRef]?.revision === 2, `series ${afterAmend.series?.[seriesId]?.revision} restaurant ${afterAmend.restaurant_revisions?.r_anker}`);
  const scheduled = (afterAmend.series?.[seriesId]?.members ?? []).map((item) => `${item.scheduled_date}:${item.exception}`);
  await expectCheck('scheduled-dates-held', scheduled.join(',') === dates.map((date) => `${date}:false`).join(','), scheduled.join(','));
  const ownerIds = [anchorRef, middleRef, laterRef].map((ref) => afterAmend.reservations?.[ref]?.user_id);
  await expectCheck('R360-owner', ownerIds.every((id) => id === 'u_diner'), ownerIds.join(','));

  const middleAfter = await historyOf(diner, middleRef);
  const anchorHist = await historyOf(diner, anchorRef);
  const laterHist = await historyOf(diner, laterRef);
  const middleChanged = middleAfter.entries?.[2] ?? {};
  const middleOrder = digestEvents(middleAfter) === 'created,reassigned,changed'
    && changeDigest(middleChanged) === `starts_at_local:"${middleDate}T19:00"->"${middleDate}T20:00"`
    && termsOk(middleAfter.entries[0].accepted_terms, policy0)
    && termsOk(middleAfter.entries[1].accepted_terms, policy0)
    && termsOk(middleChanged.accepted_terms, policy1)
    && middleAfter.entries[1].plan_id === planId;
  await expectCheck('R371-middle-history', middleOrder, digestEvents(middleAfter));
  const anchorChanged = anchorHist.entries?.[1] ?? {};
  const laterChanged = laterHist.entries?.[1] ?? {};
  await expectCheck('anchor-later-history', digestEvents(anchorHist) === 'created,changed' && digestEvents(laterHist) === 'created,changed' && termsOk(anchorChanged.accepted_terms, policy0) && termsOk(laterChanged.accepted_terms, policy1) && changeDigest(anchorChanged).startsWith('starts_at_local:') && changeDigest(laterChanged).startsWith('starts_at_local:'), `${digestEvents(anchorHist)} ${digestEvents(laterHist)}`);
  const decisionMiddle = await decisionOf(diner, middleRef);
  await expectCheck('decision-middle-policy1', decisionMiddle.revision === 3 && termsOk(decisionMiddle.accepted_terms, policy1), `revision ${decisionMiddle.revision}`);

  const beforeNoop = await exportRaw();
  const noop = await amend(diner, seriesId, 3, 0, '20:00', randomKey());
  await expectCheck('R361-noop-201', noop.result.status === 201, `status ${noop.result.status} ${errorCode(noop.result.text)}`);
  const afterNoopState = stateOf(await exportRaw());
  const beforeNoopState = stateOf(beforeNoop);
  const noopStable = afterNoopState.series?.[seriesId]?.revision === 3
    && afterNoopState.restaurant_revisions?.r_anker === beforeNoopState.restaurant_revisions?.r_anker
    && same(afterNoopState.reservations?.[middleRef]?.accepted_terms, beforeNoopState.reservations?.[middleRef]?.accepted_terms)
    && afterNoopState.reservations?.[middleRef]?.revision === beforeNoopState.reservations?.[middleRef]?.revision
    && same(afterNoopState.histories?.[middleRef], beforeNoopState.histories?.[middleRef]);
  await expectCheck('R361-noop-retains', noopStable, `series ${afterNoopState.series?.[seriesId]?.revision}`);

  await lookup(main.page, middleRef);
  await assertLookup(main.page, diner, middleRef, { status: 'confirmed', ids: REPAIRED, date: middleDate, clock: '20:00', label: 'lookup-middle-after' });
  await capturePair(main.page, 'lookup-middle-after');
  await runAxe(main.page, 'lookup-middle-after');
  await lookup(main.page, anchorRef);
  await assertLookup(main.page, diner, anchorRef, { status: 'confirmed', ids: PAIR, date: anchorDate, clock: '20:00', label: 'lookup-anchor-current' });
  await capturePair(main.page, 'lookup-anchor-current');
  await expectCheck('legacy-confirmation-unchanged', report.paint.confirmation.details.includes('(19:00)') && report.paint.confirmation.tables === phrase(PAIR), 'stored confirmation paint');

  const phoneState = freshState();
  const phone = await launch(browser, { width: 375, height: 812, video: true });
  await installRouter(phone.page, phoneState);
  await loginUi(phone.page);
  await lookup(phone.page, middleRef);
  await assertLookup(phone.page, diner, middleRef, { status: 'confirmed', ids: REPAIRED, date: middleDate, clock: '20:00', label: 'phone-lookup-middle' });
  await assertNoPageOverflow(phone.page, 'phone-lookup');
  await phone.page.waitForTimeout(700);
  forbid(phoneState, 'phone');
  await finishVideo(phone.context, phone.page, 'phone-lookup.webm');

  const patched = await api(`/reservations/${encodeURIComponent(laterRef)}`, {
    method: 'PATCH',
    token: diner,
    body: { starts_at_local: `${laterDate}T18:30` },
  });
  await expectCheck('patch-exception-200', patched.status === 200, `status ${patched.status} ${errorCode(patched.text)}`);
  const patchedBody = JSON.parse(patched.text);
  const patchProblems = await assertRecordShape(patchedBody, { ids: PAIR, date: laterDate, clock: '18:30', terms: policy1 });
  await expectCheck('patch-18:30', patchProblems.length === 0 && patchedBody.revision === 3, patchProblems.join(',') || 'later 18:30');
  const seriesAfterPatch = await seriesOf(diner, seriesId);
  const laterOcc = occurrence(seriesAfterPatch.value, laterRef);
  const middleOcc = occurrence(seriesAfterPatch.value, middleRef);
  await expectCheck('exception-flag', laterOcc?.exception === true && middleOcc?.exception === false && occurrence(seriesAfterPatch.value, anchorRef)?.exception === false && seriesAfterPatch.value.revision === 4, `series ${seriesAfterPatch.value.revision}`);
  await lookup(main.page, laterRef);
  await assertLookup(main.page, diner, laterRef, { status: 'confirmed', ids: PAIR, date: laterDate, clock: '18:30', label: 'lookup-exception' });
  await capturePair(main.page, 'lookup-exception');

  const cancelledLater = await api(`/reservations/${encodeURIComponent(laterRef)}/cancel`, { method: 'POST', token: diner });
  await expectCheck('cancel-exception-member', cancelledLater.status === 200 && JSON.parse(cancelledLater.text).status === 'cancelled', `status ${cancelledLater.status} ${errorCode(cancelledLater.text)}`);
  const seriesAfterCancelLater = await seriesOf(diner, seriesId);
  await expectCheck('cancel-keeps-exception', occurrence(seriesAfterCancelLater.value, laterRef)?.exception === true && seriesAfterCancelLater.value.revision === 5, `series ${seriesAfterCancelLater.value.revision}`);
  await lookup(main.page, laterRef);
  await assertLookup(main.page, diner, laterRef, { status: 'cancelled', ids: PAIR, date: laterDate, clock: '18:30', label: 'R159-lookup-cancelled-exception' });
  await capturePair(main.page, 'lookup-cancelled-exception');

  const cancelledAnchor = await api(`/reservations/${encodeURIComponent(anchorRef)}/cancel`, { method: 'POST', token: diner });
  await expectCheck('cancel-anchor', cancelledAnchor.status === 200 && JSON.parse(cancelledAnchor.text).status === 'cancelled', `status ${cancelledAnchor.status}`);
  const seriesAfterCancelAnchor = await seriesOf(diner, seriesId);
  await expectCheck('anchor-cancel-not-exception', occurrence(seriesAfterCancelAnchor.value, anchorRef)?.exception === false && occurrence(seriesAfterCancelAnchor.value, middleRef)?.exception === false && seriesAfterCancelAnchor.value.revision === 6, `series ${seriesAfterCancelAnchor.value.revision}`);
  await lookup(main.page, anchorRef);
  await assertLookup(main.page, diner, anchorRef, { status: 'cancelled', ids: PAIR, date: anchorDate, clock: '20:00', label: 'lookup-cancelled-anchor' });
  await capturePair(main.page, 'lookup-cancelled-anchor');

  const beforeSuffix = stateOf(await exportRaw());
  const suffix = await amend(diner, seriesId, 6, 0, '21:00', randomKey());
  await expectCheck('suffix-201', suffix.result.status === 201, `status ${suffix.result.status} ${errorCode(suffix.result.text)}`);
  const suffixSeries = JSON.parse(suffix.result.text);
  const suffixMiddle = occurrence(suffixSeries, middleRef)?.reservation ?? {};
  const suffixLater = occurrence(suffixSeries, laterRef);
  const suffixAnchor = occurrence(suffixSeries, anchorRef);
  const suffixProblems = await assertRecordShape(suffixMiddle, { ids: REPAIRED, date: middleDate, clock: '21:00', terms: policy1 });
  const suffixOk = suffixProblems.length === 0
    && occurrence(suffixSeries, middleRef)?.exception === false
    && suffixLater?.exception === true
    && suffixLater?.reservation?.status === 'cancelled'
    && suffixLater?.reservation?.starts_at_local === `${laterDate}T18:30`
    && suffixAnchor?.exception === false
    && suffixAnchor?.reservation?.status === 'cancelled'
    && suffixAnchor?.reservation?.starts_at_local === `${anchorDate}T20:00`
    && suffixSeries.revision === 7;
  await expectCheck('R358-suffix-survivor-only', suffixOk, suffixProblems.join(',') || `series ${suffixSeries.revision}`);
  const afterSuffix = stateOf(await exportRaw());
  await expectCheck('suffix-revisions', afterSuffix.restaurant_revisions?.r_anker === beforeSuffix.restaurant_revisions.r_anker + 1 && afterSuffix.reservations?.[middleRef]?.revision === beforeSuffix.reservations[middleRef].revision + 1 && afterSuffix.reservations?.[anchorRef]?.revision === beforeSuffix.reservations[anchorRef].revision && afterSuffix.reservations?.[laterRef]?.starts_at_local === `${laterDate}T18:30`, `restaurant ${afterSuffix.restaurant_revisions?.r_anker}`);
  const suffixDates = (afterSuffix.series?.[seriesId]?.members ?? []).map((item) => item.scheduled_date);
  await expectCheck('suffix-scheduled-dates', same(suffixDates, dates), suffixDates.join(','));
  await lookup(main.page, middleRef);
  await assertLookup(main.page, diner, middleRef, { status: 'confirmed', ids: REPAIRED, date: middleDate, clock: '21:00', label: 'lookup-survivor' });
  await capturePair(main.page, 'lookup-survivor');
  await lookup(main.page, laterRef);
  const laterView = await readLookup(main.page);
  await expectCheck('lookup-exception-still-cancelled', laterView.status === 'cancelled' && laterView.cancel === 0 && paintsClock(laterView.detail, laterDate, '18:30') && laterView.tables === phrase(PAIR), laterView.status);

  const elapsedChecked = [suffixMiddle, suffixLater.reservation, suffixAnchor.reservation].map((record) => elapsedMinutes(record.starts_at, record.ends_at));
  await expectCheck('elapsed-on-record', same(elapsedChecked, [60, 60, 90]), elapsedChecked.join(','));

  await replay(diner, '/reservations', captured.key, captured.body, captured.text, 'create');

  forbid(state, 'main');
  await expectCheck('diner-page-no-admin', report.forbiddenPageCalls.every((row) => row.hits.length === 0), report.forbiddenPageCalls.map((row) => `${row.label}:${row.hits.length}`).join(' '));
  await finishVideo(main.context, main.page, 'main-recurring.webm');
  return { diner, manager: manager.token, captured, adopted, applied, applyKey, planId, amended, amendKey, seriesId, refs };
}

async function runRecovery(browser, dinerFromWorld) {
  void dinerFromWorld;
  await reset();
  const manager = await login('mira.manager@example.com');
  void manager;
  const state = freshState();
  state.dropNextReservation = true;
  const { context, page } = await launch(browser, { width: 1280, height: 900, video: true });
  await installRouter(page, state);
  await loginUi(page);
  const diner = await sessionToken(page);
  await writePrivate('recovery-diner.token', diner);
  await page.goto(`${destination}/`);
  await search(page, anchorDate, 6);
  await page.getByTestId('slot-t_1+t_2-19:00').click();
  await page.getByTestId('booking-form').waitFor();
  await page.getByTestId('booking-submit').click();
  await page.getByTestId('booking-uncertain').waitFor();
  const uncertain = (await page.getByTestId('booking-uncertain').innerText()).trim();
  const errorCount = await page.getByTestId('booking-error').count();
  const confirmationCount = await page.getByTestId('confirmation').count();
  await expectCheck('R130-R131-uncertain', uncertain.length > 0 && errorCount === 0 && confirmationCount === 0, `uncertain ${uncertain.length} error ${errorCount} confirmation ${confirmationCount}`);
  if (!state.dropped || state.dropped.status !== 201) fail(`withheld status ${state.dropped?.status ?? 0} ${errorCode(state.dropped?.text ?? '')}`);
  const original = state.dropped.text;
  const originalValue = JSON.parse(original);
  await writePrivate('world2-create-201.json', original);
  await writePrivate('world2-create-request.json', state.dropped.body);
  await writePrivate('world2-create.key', state.dropped.key);
  await capturePair(page, 'uncertain');
  await runAxe(page, 'uncertain');

  const adopted = await adopt(diner, originalValue.reference, 2, randomKey());
  await expectCheck('recovery-adopt', adopted.result.status === 201, `status ${adopted.result.status} ${errorCode(adopted.result.text)}`);
  const series = JSON.parse(adopted.result.text);
  const amended = await amend(diner, series.series_id, series.revision, 0, '20:00', randomKey());
  await expectCheck('recovery-amend-before-retry', amended.result.status === 201, `status ${amended.result.status} ${errorCode(amended.result.text)}`);
  const current = await ownerGet(diner, originalValue.reference);
  await expectCheck('recovery-current-clock', current.value.starts_at_local === `${anchorDate}T20:00` && originalValue.starts_at_local === `${anchorDate}T19:00` && JSON.stringify(current.value.table_ids) === JSON.stringify(PAIR), `${originalValue.starts_at_local} -> ${current.value.starts_at_local}`);
  report.recovery = {
    originalStarts: originalValue.starts_at_local,
    currentStarts: current.value.starts_at_local,
    originalRevision: originalValue.revision,
    currentRevision: current.value.revision,
    originalEnds: originalValue.ends_at,
    currentEnds: current.value.ends_at,
    tables: current.value.table_ids,
  };

  const preRetry = await exportRaw();
  await writePrivate('world2-export-pre-retry.json', preRetry);
  state.captureReservation = true;
  await page.getByTestId('booking-submit').click();
  await page.getByTestId('confirmation-reference').waitFor();
  const retry = state.reservations[0];
  if (!retry) fail('retry was not observed');
  const sameRequest = retry.method === 'POST' && retry.path === '/reservations' && retry.key === state.dropped.key && JSON.stringify(canonical(JSON.parse(retry.body))) === JSON.stringify(canonical(JSON.parse(state.dropped.body)));
  await expectCheck('R132-retry-same-request', sameRequest, `status ${retry.status} path ${retry.path} rawBody ${retry.body === state.dropped.body}`);
  await expectCheck('R48-R53-retry-bytes', retry.status === 200 && retry.text === original, `sha ${sha256(retry.text)} original ${sha256(original)}`);
  await writePrivate('world2-retry-200.json', retry.text);
  const postRetry = await exportRaw();
  await writePrivate('world2-export-post-retry.json', postRetry);
  await expectCheck('recovery-export-equal', preRetry === postRetry, `sha ${sha256(preRetry)} ${sha256(postRetry)}`);
  const rows = Object.values(stateOf(postRetry).reservations ?? {});
  const sameRef = rows.filter((item) => item.reference === originalValue.reference);
  await expectCheck('recovery-one-record', sameRef.length === 1 && rows.length === 2, `matching ${sameRef.length} total ${rows.length}`);
  const recoveredRef = (await page.getByTestId('confirmation-reference').innerText()).trim();
  const recoveredDetails = (await page.getByTestId('confirmation-details').innerText()).trim();
  const recoveredTables = (await page.getByTestId('confirmation-tables').innerText()).trim();
  await expectCheck('recovered-confirmation-original', recoveredRef === originalValue.reference && recoveredTables === phrase(PAIR) && paintsClock(recoveredDetails, anchorDate, '19:00') && !paintsClock(recoveredDetails, anchorDate, '20:00'), 'retry painted the original receipt');
  await capturePair(page, 'recovered-confirmation');
  await lookup(page, originalValue.reference);
  await assertLookup(page, diner, originalValue.reference, { status: 'confirmed', ids: PAIR, date: anchorDate, clock: '20:00', label: 'lookup-recovered' });
  await capturePair(page, 'lookup-recovered');
  const live = await ownerGet(diner, originalValue.reference);
  await expectCheck('recovery-lookup-not-receipt', live.value.starts_at_local === `${anchorDate}T20:00` && elapsedMinutes(live.value.starts_at, live.value.ends_at) === 90, live.value.starts_at_local);
  forbid(state, 'recovery');
  await finishVideo(context, page, 'recovery.webm');
}

async function runSabotage() {
  await reset();
  const manager = await login('mira.manager@example.com');
  const diner = await login('ada.clock@example.com');
  await writePrivate('manager.token', manager.token);
  await writePrivate('diner.token', diner.token);
  const created = await api('/reservations', {
    method: 'POST',
    token: diner.token,
    key: randomKey(),
    body: { restaurant_id: 'r_anker', table_ids: PAIR, starts_at_local: `${anchorDate}T19:00`, party_size: 6 },
  });
  await expectCheck('sabotage-create', created.status === 201, `status ${created.status} ${errorCode(created.text)}`);
  const reference = JSON.parse(created.text).reference;
  const adopted = await adopt(diner.token, reference, 3, randomKey());
  await expectCheck('sabotage-adopt', adopted.result.status === 201, `status ${adopted.result.status} ${errorCode(adopted.result.text)}`);
  const middleRef = JSON.parse(adopted.result.text).occurrences[1].reference;
  const previewResult = await preview(manager.token, randomKey());
  await expectCheck('sabotage-preview', previewResult.status === 201, `status ${previewResult.status} ${errorCode(previewResult.text)}`);
  await assertPreviewAssignment(JSON.parse(previewResult.text), middleRef);
}

async function writeReport() {
  report.ended = new Date().toISOString();
  report.elapsedMs = Date.now() - startedAt.getTime();
  report.passCount = Object.keys(report.checks).length;
  report.failCount = failures.length;
  report.failures = failures;
  report.head = execFileSync('git', ['-C', worktree, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  const file = join(evidence, 'report.json');
  await writeFile(file, `${JSON.stringify(report, null, 2)}\n`);
  await chmod(file, 0o600);
}

async function main() {
  await mkdir(shotDir, { recursive: true });
  await mkdir(videoDir, { recursive: true });
  await mkdir(privateDir, { recursive: true });
  await chmod(evidence, 0o700);
  await chmod(shotDir, 0o700);
  await chmod(videoDir, 0o700);
  await chmod(privateDir, 0o700);
  await log(`START ${startedAt.toISOString()} sabotage=${sabotage}`);
  for (const date of dates) {
    if (weekday(date) !== 'thu') fail(`${date} is ${weekday(date)}`);
  }
  await pass('dates-thursday', dates.join(','));
  const health = await fetch(`${destination}/health`);
  const healthText = await health.text();
  const healthType = health.headers.get('content-type') ?? '';
  await expectCheck('R15-health', health.status === 200 && healthText === '{"status":"ok"}' && Buffer.byteLength(healthText) === 15, `status ${health.status} bytes ${Buffer.byteLength(healthText)}`);
  await expectCheck('R18-health-charset', healthType === 'application/json; charset=utf-8', healthType);
  if (sabotage) {
    await runSabotage();
    await writeReport();
    await log(`CHECKS pass=${report.passCount} fail=${report.failCount}`);
    return;
  }
  const browser = await chromium.launch({ executablePath: chrome, headless: true });
  try {
    const bundle = await runPass(browser);
    await replay(bundle.diner, '/series', bundle.adopted.key, bundle.adopted.rawBody, bundle.adopted.result.text, 'adopt');
    await replay(bundle.manager, `/restaurants/r_anker/replans/${encodeURIComponent(bundle.planId)}/apply`, bundle.applyKey, '{}', bundle.applied.text, 'apply');
    await replay(bundle.diner, `/series/${encodeURIComponent(bundle.seriesId)}/amend`, bundle.amendKey, bundle.amended.rawBody, bundle.amended.result.text, 'amend');
    await runRecovery(browser);
    await expectCheck('page-errors', report.pageErrors.length === 0, String(report.pageErrors.length));
    await expectCheck('off-origin', report.offOrigin.length === 0, String(report.offOrigin.length));
  } finally {
    await browser.close();
  }
  await writeReport();
  await log(`CHECKS pass=${report.passCount} fail=${report.failCount}`);
  if (failures.length) process.exit(1);
}

main().catch(async (error) => {
  const message = error instanceof Error ? error.message : 'probe failed';
  await log(`FAIL ${message}`).catch(() => undefined);
  await writeReport().catch(() => undefined);
  process.exit(1);
});
