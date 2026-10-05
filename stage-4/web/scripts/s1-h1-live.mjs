/**
 * Live browser probe for S1-H1. Talks to a real Tablekeeper process.
 * Prints statuses and counts only. It does not print tokens or export bodies.
 */
import { mkdir, readdir, readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { chromium } from 'playwright';

const base = process.env.BASE ?? 'http://127.0.0.1:9010';
const evidence = process.env.EVIDENCE ?? '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S1-H1';
const dist = process.env.DIST ?? new URL('../dist', import.meta.url).pathname;
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';

const thursday = '2027-06-17';
const sunday = '2027-06-13';
const pastThursday = '2026-09-24';
const password = 'correct horse';
const displayName = 'Ada';

const fixture = {
  users: [],
  restaurants: [
    {
      id: 'r_anker',
      name: 'Zum Anker',
      timezone: 'Europe/Berlin',
      slot_minutes: 30,
      reservation_duration_minutes: 90,
      cancellation_cutoff_minutes: 120,
      opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
      tables: [
        { id: 't_1', label: '1', capacity: 2 },
        { id: 't_2', label: '2', capacity: 4 },
      ],
    },
    {
      id: 'r_nord',
      name: 'Nordlicht',
      timezone: 'America/New_York',
      slot_minutes: 30,
      reservation_duration_minutes: 90,
      cancellation_cutoff_minutes: 120,
      opening_hours: [{ weekday: 'thu', opens: '17:00', closes: '22:00' }],
      tables: [
        { id: 't_window', label: 'Window', capacity: 2 },
        { id: 't_corner', label: 'Corner', capacity: 4 },
        { id: 't_booth', label: 'Booth', capacity: 6 },
        { id: 't_garden', label: 'Garden', capacity: 2 },
        { id: 't_bar', label: 'Bar', capacity: 4 },
        { id: 't_nook', label: 'Nook', capacity: 8 },
      ],
    },
  ],
  reservations: [],
};

const report = {
  base,
  offOrigin: [],
  fontRequests: [],
  publicAuthorization: [],
  checks: {},
};

function log(message) {
  console.log(message);
}

async function reset() {
  const response = await fetch(`${base}/_test/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify(fixture),
  });
  if (response.status !== 204) {
    throw new Error(`reset status ${response.status}`);
  }
}

async function waitHealth() {
  const started = Date.now();
  let last = 'none';
  while (Date.now() - started < 20000) {
    try {
      const response = await fetch(`${base}/health`);
      last = String(response.status);
      if (response.status === 200) return;
    } catch (error) {
      last = error instanceof Error ? error.name : 'error';
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`health not ready (${last})`);
}

function pathname(url) {
  return new URL(url).pathname;
}

async function scanDist() {
  const fonts = [
    'source-serif-4-latin-400-normal.woff2',
    'source-serif-4-latin-600-normal.woff2',
    'source-serif-4-latin-700-normal.woff2',
    'source-code-pro-latin-400-normal.woff2',
    'source-code-pro-latin-500-normal.woff2',
  ];
  for (const file of fonts) {
    const bytes = await readFile(join(dist, 'fonts', file));
    if (bytes.length < 100) throw new Error(`font too small ${file}`);
  }
  const offenders = [];
  async function walk(dir) {
    const entries = await readdir(dir, { withFileTypes: true });
    for (const entry of entries) {
      const path = join(dir, entry.name);
      if (entry.isDirectory()) {
        await walk(path);
        continue;
      }
      if (!/\.(html|css|js)$/.test(entry.name)) continue;
      const text = await readFile(path, 'utf8');
      const urls = text.match(/https?:\/\/[^"'\\\s)]+/g) ?? [];
      for (const url of urls) {
        if (url.includes('www.w3.org')) continue;
        if (url.includes('svelte.dev/e/')) continue;
        offenders.push(`${path} ${url}`);
      }
      if (/url\(\s*['"]?https?:/i.test(text) || /(?:src|href)\s*=\s*['"]https?:/i.test(text)) {
        offenders.push(`${path} runtime-http`);
      }
    }
  }
  await walk(dist);
  report.checks.distHttpOffenders = offenders.length;
  if (offenders.length > 0) {
    throw new Error(`dist runtime http: ${offenders.slice(0, 8).join(' | ')}`);
  }
}

async function launch() {
  return chromium.launch({
    executablePath: chrome,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });
}

function watch(page) {
  page.on('request', (request) => {
    const url = request.url();
    let origin = '';
    try {
      origin = new URL(url).origin;
    } catch {
      origin = 'bad-url';
    }
    if (origin !== new URL(base).origin && !url.startsWith('data:')) {
      report.offOrigin.push(url);
    }
    if (url.includes('/fonts/')) report.fontRequests.push(pathname(url));
    if (pathname(url) === '/restaurants' || pathname(url) === '/availability') {
      report.publicAuthorization.push(Boolean(request.headers().authorization));
    }
  });
  const errors = [];
  page.on('pageerror', (error) => errors.push(String(error)));
  return errors;
}

async function shot(page, name) {
  const dir = join(evidence, 'screenshots');
  await mkdir(dir, { recursive: true });
  await page.screenshot({ path: join(dir, `${name}.png`), fullPage: true });
  log(`screenshot ${name}`);
}

async function noHorizontalScroll(page, label) {
  const box = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    client: document.documentElement.clientWidth,
  }));
  if (box.scroll > box.client + 1) {
    throw new Error(`${label} horizontal scroll ${box.scroll} > ${box.client}`);
  }
}

async function search(page, restaurantId, date, party) {
  const detailResponse = page.waitForResponse(
    (response) => pathname(response.url()) === `/restaurants/${restaurantId}`,
    { timeout: 10000 },
  );
  const availabilityResponse = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      url.pathname === '/availability' &&
      url.searchParams.get('restaurant_id') === restaurantId &&
      url.searchParams.get('date') === date &&
      url.searchParams.get('party_size') === String(party)
    );
  }, { timeout: 10000 });
  await page.locator('[data-testid="restaurant-select"]').selectOption(restaurantId);
  await page.locator('[data-testid="date-input"]').fill(date);
  await page.locator('[data-testid="party-size-input"]').fill(String(party));
  await page.locator('[data-testid="search-button"]').click();
  await detailResponse;
  await availabilityResponse;
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
}

async function installHold(page) {
  const held = [];
  let hold = false;
  await page.route('**/*', async (route) => {
    const url = new URL(route.request().url());
    const ankerDetail = url.pathname === '/restaurants/r_anker';
    const ankerAvailability = url.pathname === '/availability' && url.searchParams.get('restaurant_id') === 'r_anker';
    if (hold && (ankerDetail || ankerAvailability)) {
      held.push(route);
      return;
    }
    await route.continue();
  });
  return {
    held,
    set(next) {
      hold = next;
    },
  };
}

async function waitForCount(read, minimum, label) {
  const started = Date.now();
  while (read() < minimum) {
    if (Date.now() - started > 8000) throw new Error(`${label} saw ${read()}`);
    await new Promise((resolve) => setTimeout(resolve, 40));
  }
}

async function behavior(page) {
  const errors = watch(page);
  await page.goto(`${base}/`, { waitUntil: 'domcontentloaded' });
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  const optionValues = await page.locator('[data-testid="restaurant-select"] option').evaluateAll((nodes) =>
    nodes.map((node) => node.value),
  );
  if (optionValues.join(',') !== 'r_anker,r_nord') {
    throw new Error(`option values ${optionValues.join(',')}`);
  }

  await search(page, 'r_anker', thursday, 2);
  const detail = await page.evaluate(async () => {
    const response = await fetch('/restaurants/r_anker');
    if (!response.ok) throw new Error(String(response.status));
    return response.json();
  });
  const availability = await page.evaluate(async () => {
    const response = await fetch('/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2');
    if (!response.ok) throw new Error(String(response.status));
    return response.json();
  });
  const membershipKey = ['available', 'table', 'ids'].join('_');
  const expected = [];
  for (const table of detail.tables) {
    for (const slot of availability.slots) {
      const time = String(slot.starts_at_local).slice(11, 16);
      const ids = slot[membershipKey];
      expected.push({
        id: `slot-${table.id}-${time}`,
        available: Array.isArray(ids) && ids.includes(table.id) ? 'true' : 'false',
      });
    }
  }
  const actual = await page.locator('[data-testid^="slot-"]').evaluateAll((nodes) =>
    nodes.map((node) => ({
      id: node.getAttribute('data-testid'),
      available: node.getAttribute('data-available'),
    })),
  );
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`grid mismatch cells ${actual.length} expected ${expected.length}`);
  }
  report.checks.ankerCells = actual.length;
  await noHorizontalScroll(page, 'anker results');

  await search(page, 'r_anker', sunday, 2);
  if ((await page.locator('[data-testid="no-slots"]').count()) !== 1) throw new Error('closed day missing no-slots');
  if ((await page.locator('[data-testid="availability-grid"]').count()) !== 0) throw new Error('closed day kept the grid');
  if ((await page.locator('[data-testid="floor-plan"]').count()) !== 0) throw new Error('closed day kept the floor');

  await search(page, 'r_anker', thursday, 100);
  const zeroCells = await page.locator('[data-testid^="slot-"]').evaluateAll((nodes) =>
    nodes.map((node) => node.getAttribute('data-available')),
  );
  if (zeroCells.length === 0 || zeroCells.some((value) => value !== 'false')) {
    throw new Error(`zero availability mismatch ${zeroCells.length}`);
  }
  if ((await page.locator('[data-testid="zero-available"]').count()) !== 1) throw new Error('missing zero-available');
  if ((await page.locator('[data-testid="no-slots"]').count()) !== 0) throw new Error('zero availability used no-slots');
  report.checks.zeroCells = zeroCells.length;

  await search(page, 'r_anker', pastThursday, 2);
  if ((await page.locator('[data-testid="slot-t_1-18:00"]').count()) !== 1) {
    throw new Error('past Thursday produced no 18:00 cell');
  }

  await search(page, 'r_nord', thursday, 2);
  const nordHeading = await page.locator('h1').innerText();
  if (!nordHeading.includes('Nordlicht')) throw new Error(`heading ${nordHeading}`);
  if ((await page.locator('[data-testid="slot-t_window-17:00"]').count()) !== 1) {
    throw new Error('nord grid missing window 17:00');
  }
  if ((await page.locator('[data-testid="slot-t_1-18:00"]').count()) !== 0) {
    throw new Error('nord grid still shows anker');
  }
  await page.locator('[data-testid="restaurant-select"]').selectOption('r_anker');
  const stillNord = await page.locator('h1').innerText();
  if (!stillNord.includes('Nordlicht')) throw new Error('live select overwrote the snapshot');

  const gate = await installHold(page);
  gate.set(true);
  await page.locator('[data-testid="restaurant-select"]').selectOption('r_anker');
  await page.locator('[data-testid="date-input"]').fill(thursday);
  await page.locator('[data-testid="party-size-input"]').fill('2');
  await page.locator('[data-testid="search-button"]').click();
  await waitForCount(() => gate.held.length, 2, 'held anker');
  await page.locator('[data-testid="restaurant-select"]').selectOption('r_nord');
  await page.locator('[data-testid="search-button"]').click();
  await page.locator('[data-testid="slot-t_window-17:00"]').waitFor();
  const late = gate.held.splice(0);
  gate.set(false);
  const lateSeen = page.waitForResponse((response) => pathname(response.url()) === '/restaurants/r_anker', { timeout: 8000 });
  for (const route of late) await route.continue();
  await lateSeen;
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  if ((await page.locator('h1').innerText()).includes('Zum Anker')) throw new Error('late anker success overwrote nord');
  if ((await page.locator('[data-testid="slot-t_1-18:00"]').count()) !== 0) throw new Error('late anker cell appeared');
  if ((await page.locator('[data-testid="search-error"]').count()) !== 0) throw new Error('late anker success showed an error');

  gate.set(true);
  await page.locator('[data-testid="restaurant-select"]').selectOption('r_anker');
  await page.locator('[data-testid="search-button"]').click();
  await waitForCount(() => gate.held.length, 2, 'held anker failure');
  await page.locator('[data-testid="restaurant-select"]').selectOption('r_nord');
  await page.locator('[data-testid="search-button"]').click();
  await page.locator('[data-testid="slot-t_corner-17:00"]').waitFor();
  const failed = gate.held.splice(0);
  gate.set(false);
  for (const route of failed) await route.abort('failed').catch(() => {});
  await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 250)));
  if (!(await page.locator('h1').innerText()).includes('Nordlicht')) throw new Error('late failure cleared nord');
  if ((await page.locator('[data-testid="search-error"]').count()) !== 0) throw new Error('late failure showed search-error');
  report.checks.race = 'anker-detail-and-availability-ignored';

  await page.unroute('**/*');
  await search(page, 'r_anker', thursday, 4);
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  if ((await page.locator('[data-testid="booking-form"]').count()) !== 0) throw new Error('unavailable click opened a form');
  if ((await page.locator('[data-testid="auth-error"]').count()) !== 0) throw new Error('unavailable click showed auth-error');
  await page.locator('[data-testid="slot-t_2-18:00"]').click();
  await page.locator('[data-testid="auth-error"]').waitFor();
  const authText = await page.locator('[data-testid="auth-error"]').innerText();
  if (!authText.includes('Sign in to hold a table.')) throw new Error('signed-out copy missing');
  if ((await page.locator('[data-testid="booking-form"]').count()) !== 0) throw new Error('signed-out click opened a form');
  if ((await page.locator('[data-testid="confirmation"]').count()) !== 0) throw new Error('signed-out click confirmed');

  const email = `ada.${Date.now()}@example.com`;
  await page.goto(`${base}/signup`);
  await page.locator('[data-testid="signup-email"]').fill(email);
  await page.locator('[data-testid="signup-password"]').fill('short');
  await page.locator('[data-testid="signup-display-name"]').fill(displayName);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="auth-error"]').waitFor();
  if ((await page.locator('[data-testid="current-user"]').count()) !== 0) throw new Error('short password signed in');
  await page.locator('[data-testid="signup-password"]').fill(password);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();
  if (!(await page.locator('[data-testid="current-user"]').innerText()).includes(displayName)) {
    throw new Error('signup display name missing');
  }
  await page.goto(`${base}/lookup`);
  if (!(await page.locator('[data-testid="current-user"]').innerText()).includes(displayName)) {
    throw new Error('display name dropped on lookup');
  }
  await page.locator('[data-testid="lookup-submit"]').click();
  if ((await page.locator('[data-testid="reservation-detail"]').count()) !== 0) throw new Error('lookup shell fetched a record');
  await page.locator('[data-testid="logout-button"]').click();
  await page.locator('[data-testid="current-user"]').waitFor({ state: 'detached' });
  const cleared = await page.evaluate(() => localStorage.getItem('tablekeeper-session'));
  if (cleared !== null) throw new Error('logout left a session');

  await page.goto(`${base}/login`);
  await page.locator('[data-testid="login-email"]').fill(email);
  await page.locator('[data-testid="login-password"]').fill('wrong horse');
  await page.locator('[data-testid="login-submit"]').click();
  await page.locator('[data-testid="auth-error"]').waitFor();
  if ((await page.locator('[data-testid="current-user"]').count()) !== 0) throw new Error('bad login signed in');
  await page.locator('[data-testid="login-password"]').fill(password);
  await page.locator('[data-testid="login-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();

  const proof = await page.evaluate(async () => {
    const raw = localStorage.getItem('tablekeeper-session');
    if (!raw) return { ok: false, reason: 'no-session' };
    const session = JSON.parse(raw);
    const exported = await fetch('/_test/export');
    const body = await exported.text();
    const imported = await fetch('/_test/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body,
    });
    const listed = await fetch('/reservations', {
      headers: { Authorization: `Bearer ${session.token}`, Accept: 'application/json' },
    });
    const payload = await listed.json();
    const visible = document.querySelector('[data-testid="current-user"]')?.textContent ?? '';
    return {
      exportStatus: exported.status,
      importStatus: imported.status,
      listStatus: listed.status,
      listIsArray: Array.isArray(payload.reservations),
      displayNameStillVisible: visible.includes(session.displayName),
    };
  });
  report.checks.upgrade = proof;
  if (proof.exportStatus !== 200 || proof.importStatus !== 204 || proof.listStatus !== 200 || !proof.listIsArray) {
    throw new Error(`upgrade proof failed statuses ${proof.exportStatus}/${proof.importStatus}/${proof.listStatus}`);
  }
  if (!proof.displayNameStillVisible) throw new Error('display name disappeared after import');

  await page.goto(`${base}/`);
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  await search(page, 'r_anker', thursday, 2);
  await page.locator('[data-testid="party-size-input"]').fill('9');
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  await page.locator('[data-testid="booking-form"]').waitFor();
  const summary = await page.locator('[data-testid="booking-summary"]').innerText();
  const party = await page.locator('[data-testid="booking-party-size"]').inputValue();
  if (party !== '2') throw new Error(`prefilled party ${party}`);
  if (!summary.includes('Zum Anker') || !summary.includes('Table 1') || !summary.includes('18:00') || !summary.includes('6:00 PM')) {
    throw new Error('summary missing restaurant, label or time');
  }
  if ((await page.locator('[data-testid="confirmation"]').count()) !== 0) throw new Error('form claimed a confirmation');
  const pending = await page.locator('[data-testid="booking-form"]').innerText();
  if (pending.includes('Booking submission is pending.')) throw new Error('placeholder booking copy is still shown');
  if ((await page.locator('[data-testid="booking-error"]').count()) !== 0) throw new Error('opening the form showed an error');
  await page.locator('[data-testid="plan-t_2"]').focus();
  await page.keyboard.press('Enter');
  await page.locator('[data-testid="booking-summary"]').waitFor();
  const moved = await page.locator('[data-testid="booking-summary"]').innerText();
  if (!moved.includes('Table 2')) throw new Error('keyboard selection did not hold table 2');
  report.checks.partySnapshot = party;
  report.checks.pageErrors = errors.length;
  if (errors.length > 0) throw new Error(`page errors ${errors.length}`);
}

async function captureStates(browser, width, theme) {
  const context = await browser.newContext({
    viewport: { width, height: width < 700 ? 900 : 1000 },
    deviceScaleFactor: 1,
  });
  await context.addInitScript((mode) => {
    localStorage.setItem('chaaya-theme', mode);
  }, theme);
  const page = await context.newPage();
  watch(page);
  const prefix = `${width}-${theme}`;

  let releaseLoading = () => {};
  const loadingGate = new Promise((resolve) => {
    releaseLoading = resolve;
  });
  await page.route('**/*', async (route) => {
    if (pathname(route.request().url()) === '/restaurants') {
      await loadingGate;
    }
    await route.continue();
  });
  await page.goto(`${base}/`, { waitUntil: 'domcontentloaded' });
  await page.locator('[data-testid="loading-state"]').waitFor();
  await shot(page, `${prefix}-loading`);
  releaseLoading();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  await page.unroute('**/*');

  await page.route('**/*', async (route) => {
    if (pathname(route.request().url()) === '/restaurants') {
      await route.abort('failed');
      return;
    }
    await route.continue();
  });
  await page.goto(`${base}/`, { waitUntil: 'domcontentloaded' });
  await page.locator('[data-testid="search-error"]').waitFor();
  await shot(page, `${prefix}-error`);
  await page.unroute('**/*');
  await page.locator('[data-testid="catalog-retry"]').click();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);

  await search(page, 'r_anker', sunday, 2);
  await shot(page, `${prefix}-empty`);
  await noHorizontalScroll(page, `${prefix} empty`);

  await search(page, 'r_anker', thursday, 2);
  await shot(page, `${prefix}-results`);
  await noHorizontalScroll(page, `${prefix} results`);

  if (width === 375) {
    await search(page, 'r_nord', thursday, 2);
    await shot(page, `${prefix}-results-nord`);
    await noHorizontalScroll(page, `${prefix} nord`);
    await search(page, 'r_anker', thursday, 2);
  }

  await page.goto(`${base}/login`);
  await page.locator('[data-testid="login-email"]').fill('nobody@example.com');
  await page.locator('[data-testid="login-password"]').fill('wrong horse');
  await page.locator('[data-testid="login-submit"]').click();
  await page.locator('[data-testid="auth-error"]').waitFor();
  await shot(page, `${prefix}-auth`);
  await noHorizontalScroll(page, `${prefix} auth`);

  const email = `shot.${width}.${theme}.${Date.now()}@example.com`;
  await page.goto(`${base}/signup`);
  await page.locator('[data-testid="signup-email"]').fill(email);
  await page.locator('[data-testid="signup-password"]').fill(password);
  await page.locator('[data-testid="signup-display-name"]').fill(displayName);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  await search(page, 'r_anker', thursday, 2);
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  await page.locator('[data-testid="booking-form"]').waitFor();
  await shot(page, `${prefix}-selected`);
  await noHorizontalScroll(page, `${prefix} selected`);
  await context.close();
}

async function recordFlow(browser, width) {
  const dir = join(evidence, 'video');
  await mkdir(dir, { recursive: true });
  const context = await browser.newContext({
    viewport: { width, height: width < 700 ? 900 : 1000 },
    recordVideo: { dir, size: { width, height: width < 700 ? 900 : 1000 } },
  });
  const page = await context.newPage();
  const email = `film.${width}.${Date.now()}@example.com`;
  await page.goto(`${base}/signup`);
  await page.locator('[data-testid="signup-display-name"]').fill(displayName);
  await page.locator('[data-testid="signup-email"]').fill(email);
  await page.locator('[data-testid="signup-password"]').fill(password);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  await search(page, 'r_nord', thursday, 2);
  await page.locator('[data-testid="slot-t_window-17:00"]').click();
  await page.locator('[data-testid="booking-form"]').waitFor();
  await page.locator('[data-testid="logout-button"]').click();
  await page.locator('[data-testid="current-user"]').waitFor({ state: 'detached' });
  await search(page, 'r_anker', thursday, 2);
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  await page.locator('[data-testid="auth-error"]').waitFor();
  const video = page.video();
  await context.close();
  if (video) {
    const saved = await video.path();
    const target = join(dir, `main-flow-${width}.webm`);
    const bytes = await readFile(saved);
    await writeFile(target, bytes);
    log(`video main-flow-${width} ${bytes.length}`);
  }
}

async function reducedMotion(browser) {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    reducedMotion: 'reduce',
  });
  const page = await context.newPage();
  await page.goto(`${base}/signup`);
  await page.locator('[data-testid="signup-email"]').fill(`motion.${Date.now()}@example.com`);
  await page.locator('[data-testid="signup-password"]').fill(password);
  await page.locator('[data-testid="signup-display-name"]').fill(displayName);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  await search(page, 'r_anker', thursday, 2);
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  const selected = await page.locator('[data-testid="slot-t_1-18:00"]').getAttribute('data-selected');
  const plan = await page.locator('[data-testid="plan-t_1"]').getAttribute('data-selected');
  report.checks.reducedMotionSelected = { cell: selected, plan };
  if (selected !== 'true' || plan !== 'true') throw new Error(`reduced motion selected cell ${selected} plan ${plan}`);
  await context.close();
}

async function main() {
  await mkdir(evidence, { recursive: true });
  await scanDist();
  await waitHealth();
  await reset();
  const browser = await launch();
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 1000 } });
    const page = await context.newPage();
    await behavior(page);
    await context.close();
    for (const width of [375, 1280]) {
      for (const theme of ['light', 'dark']) {
        await captureStates(browser, width, theme);
      }
    }
    await reducedMotion(browser);
    for (const width of [375, 1280]) {
      await recordFlow(browser, width);
    }
    if (report.offOrigin.length > 0) {
      throw new Error(`off-origin requests ${report.offOrigin.length}`);
    }
    if (report.fontRequests.length === 0) throw new Error('no local font requests');
    if (report.publicAuthorization.some(Boolean)) throw new Error('public read sent a bearer token');
    report.checks.fontRequestCount = report.fontRequests.length;
    report.checks.offOrigin = 0;
    await writeFile(join(evidence, 'live-probe.json'), `${JSON.stringify(report, null, 2)}\n`);
    log(`probe ok cells=${report.checks.ankerCells} fonts=${report.fontRequests.length}`);
  } finally {
    await Promise.race([browser.close(), new Promise((resolve) => setTimeout(resolve, 4000))]);
  }
}

main()
  .then(() => process.exit(0))
  .catch(async (error) => {
    const message = error instanceof Error ? error.stack ?? error.message : String(error);
    log(`PROBE FAIL ${message.split('\n')[0]}`);
    try {
      await mkdir(evidence, { recursive: true });
      await writeFile(join(evidence, 'live-probe.json'), `${JSON.stringify({ ...report, failed: message.split('\n')[0] }, null, 2)}\n`);
    } catch {
      // The process exit is the signal that matters.
    }
    process.exit(1);
  });
