/**
 * Live browser probe for S1-H2. Talks only to the built image.
 * Prints statuses, references and sameness flags. It does not print tokens or export bodies.
 */
import { mkdir, readdir, readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { chromium } from 'playwright';

const base = process.env.BASE ?? 'http://127.0.0.1:9010';
const evidence = process.env.EVIDENCE ?? '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S1-H2';
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
  fontStatuses: [],
  publicAuthorization: [],
  checks: {},
};

function log(message) {
  console.log(message);
}

function fail(message) {
  throw new Error(message);
}

async function reset() {
  const response = await fetch(`${base}/_test/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify(fixture),
  });
  if (response.status !== 204) fail(`reset status ${response.status}`);
}

async function waitHealth() {
  const started = Date.now();
  let last = 'none';
  while (Date.now() - started < 20000) {
    try {
      const response = await fetch(`${base}/health`);
      last = String(response.status);
      if (response.status === 200) {
        const body = await response.json();
        if (body.status !== 'ok') fail(`health body ${JSON.stringify(body)}`);
        return;
      }
    } catch (error) {
      last = error instanceof Error ? error.name : 'error';
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  fail(`health not ready (${last})`);
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
    if (bytes.length < 100) fail(`font too small ${file}`);
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
    }
  }
  await walk(dist);
  report.checks.distHttpOffenders = offenders.length;
  if (offenders.length > 0) fail(`dist runtime http: ${offenders.slice(0, 8).join(' | ')}`);
}

async function launch() {
  return chromium.launch({
    executablePath: chrome,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });
}

function watch(page) {
  const errors = [];
  page.on('request', (request) => {
    const url = request.url();
    let origin = '';
    try {
      origin = new URL(url).origin;
    } catch {
      origin = 'bad-url';
    }
    if (origin !== new URL(base).origin && !url.startsWith('data:')) report.offOrigin.push(url);
    if (url.includes('/fonts/')) report.fontRequests.push(pathname(url));
    const path = pathname(url);
    if (path === '/restaurants' || path === '/availability') {
      report.publicAuthorization.push(Boolean(request.headers().authorization));
    }
  });
  page.on('response', (response) => {
    if (response.url().includes('/fonts/')) report.fontStatuses.push(response.status());
  });
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
  if (box.scroll > box.client + 1) fail(`${label} horizontal scroll ${box.scroll} > ${box.client}`);
}

async function frames(page) {
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
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
  await frames(page);
}

function watchPosts(page, posts) {
  page.on('request', (request) => {
    if (request.method() !== 'POST' || pathname(request.url()) !== '/reservations') return;
    posts.push({
      key: request.headers()['idempotency-key'] ?? '',
      body: request.postDataJSON(),
    });
  });
}

async function ownedReferences(page) {
  return page.evaluate(async () => {
    const raw = localStorage.getItem('tablekeeper-session');
    const session = raw ? JSON.parse(raw) : null;
    if (!session || !session.token) return { status: 0, references: [] };
    const response = await fetch('/reservations', {
      headers: { Authorization: `Bearer ${session.token}`, Accept: 'application/json' },
    });
    const body = await response.json();
    const references = Array.isArray(body.reservations) ? body.reservations.map((item) => item.reference) : [];
    return { status: response.status, references };
  });
}

async function signupApi(email, name) {
  const response = await fetch(`${base}/auth/signup`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify({ email, password, display_name: name }),
  });
  const body = await response.json();
  if (response.status !== 201 || typeof body.token !== 'string') {
    fail(`competitor signup ${response.status} ${body.error?.code ?? ''}`);
  }
  return body.token;
}

async function bookApi(token, tableId, starts, party) {
  const response = await fetch(`${base}/reservations`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json; charset=utf-8',
      Authorization: `Bearer ${token}`,
      'Idempotency-Key': `other-${tableId}-${starts.replace(/\W/g, '')}-${Date.now()}`,
    },
    body: JSON.stringify({
      restaurant_id: 'r_anker',
      table_id: tableId,
      starts_at_local: starts,
      party_size: party,
    }),
  });
  const body = await response.json();
  return { status: response.status, code: body.error?.code ?? '', reference: body.reference ?? '' };
}

async function loseNextCreate(page) {
  const info = { key: '', body: null, status: 0, reference: '', code: '' };
  const handler = async (route) => {
    const request = route.request();
    if (info.status !== 0 || request.method() !== 'POST') {
      await route.continue();
      return;
    }
    info.key = request.headers()['idempotency-key'] ?? '';
    info.body = request.postDataJSON();
    const response = await route.fetch();
    info.status = response.status();
    try {
      const payload = await response.json();
      info.reference = typeof payload.reference === 'string' ? payload.reference : '';
      info.code = payload.error?.code ?? '';
    } catch {
      info.reference = '';
    }
    await route.abort('failed');
  };
  await page.route(/\/reservations$/, handler);
  return {
    info,
    async stop() {
      await page.unroute(/\/reservations$/, handler);
    },
  };
}

async function shotThemes(page, name) {
  const widths = [1280, 375];
  const themes = ['light', 'dark'];
  for (const width of widths) {
    await page.setViewportSize({ width, height: width < 700 ? 900 : 1000 });
    for (const theme of themes) {
      await page.locator(`[data-testid="theme-${theme}"]`).click();
      await frames(page);
      await shot(page, `${width}-${theme}-${name}`);
      await noHorizontalScroll(page, `${width}-${theme}-${name}`);
    }
  }
  await page.setViewportSize({ width: 1280, height: 1000 });
  await page.locator('[data-testid="theme-light"]').click();
}

async function behavior(page, posts) {
  const errors = watch(page);
  await page.goto(`${base}/`, { waitUntil: 'domcontentloaded' });
  await page.locator('[data-testid="theme-light"]').click();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  const optionValues = await page.locator('[data-testid="restaurant-select"] option').evaluateAll((nodes) =>
    nodes.map((node) => node.value),
  );
  if (optionValues.join(',') !== 'r_anker,r_nord') fail(`option values ${optionValues.join(',')}`);

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
    fail(`grid mismatch cells ${actual.length} expected ${expected.length}`);
  }
  report.checks.ankerCells = actual.length;
  await noHorizontalScroll(page, 'anker results');

  await search(page, 'r_anker', sunday, 2);
  if ((await page.locator('[data-testid="no-slots"]').count()) !== 1) fail('closed day missing no-slots');
  if ((await page.locator('[data-testid="availability-grid"]').count()) !== 0) fail('closed day kept the grid');

  await search(page, 'r_anker', thursday, 100);
  const zeroCells = await page.locator('[data-testid^="slot-"]').evaluateAll((nodes) =>
    nodes.map((node) => node.getAttribute('data-available')),
  );
  if (zeroCells.length === 0 || zeroCells.some((value) => value !== 'false')) {
    fail(`zero availability mismatch ${zeroCells.length}`);
  }
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  if ((await page.locator('[data-testid="booking-form"]').count()) !== 0) fail('unavailable click opened a form');
  report.checks.zeroCells = zeroCells.length;

  await search(page, 'r_anker', thursday, 4);
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  if ((await page.locator('[data-testid="booking-form"]').count()) !== 0) fail('small table opened for a party of 4');
  await page.locator('[data-testid="slot-t_2-18:00"]').click();
  await page.locator('[data-testid="auth-error"]').waitFor();
  if (!(await page.locator('[data-testid="auth-error"]').innerText()).includes('Sign in to hold a table.')) {
    fail('signed-out copy missing');
  }
  if ((await page.locator('[data-testid="booking-form"]').count()) !== 0) fail('signed-out click opened a form');

  await search(page, 'r_nord', thursday, 2);
  if (!(await page.locator('h1').innerText()).includes('Nordlicht')) fail('nord heading missing');
  await page.locator('[data-testid="restaurant-select"]').selectOption('r_anker');
  if (!(await page.locator('h1').innerText()).includes('Nordlicht')) fail('live select overwrote the snapshot');

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
  hold = true;
  await page.locator('[data-testid="restaurant-select"]').selectOption('r_anker');
  await page.locator('[data-testid="date-input"]').fill(thursday);
  await page.locator('[data-testid="party-size-input"]').fill('2');
  await page.locator('[data-testid="search-button"]').click();
  const started = Date.now();
  while (held.length < 2) {
    if (Date.now() - started > 8000) fail(`held anker saw ${held.length}`);
    await new Promise((resolve) => setTimeout(resolve, 40));
  }
  await page.locator('[data-testid="restaurant-select"]').selectOption('r_nord');
  await page.locator('[data-testid="search-button"]').click();
  await page.locator('[data-testid="slot-t_window-17:00"]').waitFor();
  const late = held.splice(0);
  hold = false;
  for (const route of late) await route.continue();
  await frames(page);
  await page.waitForTimeout(250);
  if ((await page.locator('h1').innerText()).includes('Zum Anker')) fail('late anker success overwrote nord');
  if ((await page.locator('[data-testid="slot-t_1-18:00"]').count()) !== 0) fail('late anker cell appeared');
  await page.unroute('**/*');
  report.checks.race = 'anker-detail-and-availability-ignored';

  const email = `ada.${Date.now()}@example.com`;
  await page.locator('a[href="/signup"]').click();
  await page.locator('[data-testid="signup-email"]').fill(email);
  await page.locator('[data-testid="signup-password"]').fill('short');
  await page.locator('[data-testid="signup-display-name"]').fill(displayName);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="auth-error"]').waitFor();
  await page.locator('[data-testid="signup-password"]').fill(password);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();
  if (!(await page.locator('[data-testid="current-user"]').innerText()).includes(displayName)) fail('signup name missing');

  await page.locator('a[href="/lookup"]').click();
  await page.locator('[data-testid="lookup-reference-input"]').waitFor();
  if (!(await page.locator('[data-testid="current-user"]').innerText()).includes(displayName)) fail('name missing on lookup');
  const lookupsBefore = posts.length;
  await page.locator('[data-testid="lookup-submit"]').click();
  await frames(page);
  if ((await page.locator('[data-testid="reservation-detail"]').count()) !== 0) fail('empty lookup showed a record');
  if (!(await page.locator('[data-testid="reservation-error"]').innerText()).includes('Enter a reservation reference.')) {
    fail('empty lookup missed its error');
  }
  if (posts.length !== lookupsBefore) fail('empty lookup submitted a booking');
  await page.locator('nav a[href="/"]').click();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);

  await search(page, 'r_anker', thursday, 2);
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  await page.locator('[data-testid="booking-form"]').waitFor();
  const summary = await page.locator('[data-testid="booking-summary"]').innerText();
  if (!summary.includes('Zum Anker') || !summary.includes('Table 1') || !summary.includes('18:00') || !summary.includes('6:00 PM')) {
    fail(`summary ${summary}`);
  }
  if ((await page.locator('[data-testid="booking-party-size"]').inputValue()) !== '2') fail('party was not the searched size');
  if ((await page.locator('[data-testid="booking-form"]').innerText()).includes('Booking submission is pending.')) {
    fail('placeholder booking copy is still shown');
  }

  const beforeFirst = posts.length;
  const firstResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && pathname(response.url()) === '/reservations',
    { timeout: 10000 },
  );
  await page.locator('[data-testid="booking-submit"]').click();
  const created = await firstResponse;
  const createdBody = await created.json();
  const first = posts[beforeFirst];
  if (!first || created.status() !== 201) fail(`create status ${created.status()}`);
  if (!/^[A-Z0-9]{6,12}$/.test(createdBody.reference ?? '')) fail('reference shape');
  const referenceA = createdBody.reference;
  await page.locator('[data-testid="confirmation-reference"]').waitFor();
  if ((await page.locator('[data-testid="confirmation-reference"]').innerText()) !== referenceA) fail('reference text');
  const details = await page.locator('[data-testid="confirmation-details"]').innerText();
  if (!details.includes('Zum Anker') || !details.includes('Table 1') || !details.includes('Thursday 17 June 2027') || !details.includes('6:00 PM')) {
    fail(`confirmation details ${details}`);
  }
  if (!(await page.locator('[data-testid="confirmation-tables"]').innerText()).includes('1')) fail('confirmation tables');
  if ((await page.locator('[data-testid="booking-form"]').count()) !== 1) fail('form left after success');
  await shotThemes(page, 'confirmed');

  const beforeReplay = posts.length;
  const replayResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && pathname(response.url()) === '/reservations',
    { timeout: 10000 },
  );
  await page.locator('[data-testid="booking-submit"]').click();
  const replayed = await replayResponse;
  const replayBody = await replayed.json();
  const replay = posts[beforeReplay];
  const listedOnce = await ownedReferences(page);
  const countA = listedOnce.references.filter((item) => item === referenceA).length;
  report.checks.replay = {
    status: replayed.status(),
    sameKey: replay.key === first.key && replay.key.length > 0,
    sameBody: JSON.stringify(replay.body) === JSON.stringify(first.body),
    sameReference: replayBody.reference === referenceA,
    recordCount: countA,
  };
  if (replayed.status() !== 200 || !report.checks.replay.sameKey || !report.checks.replay.sameBody || countA !== 1) {
    fail(`replay ${JSON.stringify(report.checks.replay)}`);
  }
  if ((await page.locator('[data-testid="confirmation-reference"]').innerText()) !== referenceA) fail('replay changed the reference');

  await page.locator('[data-testid="booking-party-size"]').fill('1');
  const beforeChanged = posts.length;
  const changedResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && pathname(response.url()) === '/reservations',
    { timeout: 10000 },
  );
  const changedRefresh = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === '/availability' && url.searchParams.get('party_size') === '2' && url.searchParams.get('date') === thursday;
  }, { timeout: 10000 });
  await page.locator('[data-testid="booking-submit"]').click();
  const changed = await changedResponse;
  const changedPayload = await changed.json();
  await changedRefresh;
  const changedPost = posts[beforeChanged];
  const afterChanged = await ownedReferences(page);
  report.checks.changedField = {
    status: changed.status(),
    code: changedPayload.error?.code ?? '',
    sameKey: changedPost.key === first.key,
    party: changedPost.body?.party_size ?? null,
    recordCount: afterChanged.references.filter((item) => item === referenceA).length,
    total: afterChanged.references.length,
  };
  if (changedPost.key === first.key || changedPost.body?.party_size !== 1) fail(`changed attempt ${JSON.stringify(report.checks.changedField)}`);
  if (changed.status() !== 409 || changedPayload.error?.code !== 'table_unavailable') {
    fail(`changed field response ${changed.status()} ${changedPayload.error?.code ?? ''}`);
  }
  if ((await page.locator('[data-testid="confirmation"]').count()) !== 0) fail('rejected attempt showed a confirmation');
  if ((await page.locator('[data-testid="booking-error"]').count()) !== 1) fail('rejected attempt hid the error');
  if (report.checks.changedField.recordCount !== 1 || report.checks.changedField.total !== 1) {
    fail(`changed field mutated records ${JSON.stringify(report.checks.changedField)}`);
  }

  await page.locator('[data-testid="slot-t_1-19:30"]').click();
  await page.locator('[data-testid="booking-summary"]').waitFor();
  if (!(await page.locator('[data-testid="booking-summary"]').innerText()).includes('19:30')) fail('second slot was not selected');
  if ((await page.locator('[data-testid="booking-party-size"]').inputValue()) !== '2') fail('new selection did not restore the searched party');
  const beforeSecond = posts.length;
  const secondResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && pathname(response.url()) === '/reservations',
    { timeout: 10000 },
  );
  await page.locator('[data-testid="booking-submit"]').click();
  const second = await secondResponse;
  const secondBody = await second.json();
  const secondPost = posts[beforeSecond];
  if (second.status() !== 201 || secondBody.reference === referenceA || secondPost.key === changedPost.key) {
    fail(`second booking ${second.status()} ${secondBody.reference ?? secondBody.error?.code ?? ''}`);
  }
  const referenceB = secondBody.reference;
  if ((await page.locator('[data-testid="confirmation-reference"]').innerText()) !== referenceB) fail('second reference');

  const rivalEmail = `bea.${Date.now()}@example.com`;
  const rivalToken = await signupApi(rivalEmail, 'Bea');
  const rival = await bookApi(rivalToken, 't_2', `${thursday}T18:00`, 2);
  if (rival.status !== 201) fail(`rival booking ${rival.status} ${rival.code}`);
  await page.locator('[data-testid="slot-t_2-18:00"]').click();
  if (!(await page.locator('[data-testid="booking-summary"]').innerText()).includes('Table 2')) fail('conflict selection missed table 2');
  await page.locator('[data-testid="booking-party-size"]').fill('4');
  const beforeConflict = posts.length;
  const conflictResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && pathname(response.url()) === '/reservations',
    { timeout: 10000 },
  );
  await page.locator('[data-testid="booking-submit"]').click();
  const conflict = await conflictResponse;
  const conflictPayload = await conflict.json();
  const conflictPost = posts[beforeConflict];
  await page.locator('[data-testid="booking-error"]').waitFor();
  await page.waitForFunction(() => document.querySelector('[data-testid="slot-t_2-18:00"]')?.getAttribute('data-available') === 'false');
  const partyKept = await page.locator('[data-testid="booking-party-size"]').inputValue();
  report.checks.conflict = {
    status: conflict.status(),
    code: conflictPayload.error?.code ?? '',
    partyKept,
    formKept: (await page.locator('[data-testid="booking-form"]').count()) === 1,
    confirmation: (await page.locator('[data-testid="confirmation"]').count()) > 0,
    table: conflictPost.body?.table_id ?? '',
    party: conflictPost.body?.party_size ?? null,
    newKey: conflictPost.key !== secondPost.key,
  };
  if (conflict.status() !== 409 || conflictPayload.error?.code !== 'table_unavailable' || partyKept !== '4' || report.checks.conflict.confirmation) {
    fail(`conflict ${JSON.stringify(report.checks.conflict)}`);
  }
  if (!report.checks.conflict.formKept || conflictPost.body?.table_id !== 't_2') fail(`conflict form ${JSON.stringify(report.checks.conflict)}`);
  await shotThemes(page, 'booking-error');

  await page.locator('[data-testid="slot-t_2-19:30"]').click();
  if (!(await page.locator('[data-testid="booking-summary"]').innerText()).includes('19:30')) fail('recovery slot was not selected');
  const beforeLostList = await ownedReferences(page);
  const lostGate = await loseNextCreate(page);
  const beforeLost = posts.length;
  await page.locator('[data-testid="booking-submit"]').click();
  await page.locator('[data-testid="booking-uncertain"]').waitFor({ timeout: 10000 });
  await lostGate.stop();
  const lostPost = posts[beforeLost];
  const uncertainText = (await page.locator('[data-testid="booking-uncertain"]').innerText()).trim();
  report.checks.lost = {
    serverStatus: lostGate.info.status,
    serverCode: lostGate.info.code,
    uncertain: uncertainText.length > 0,
    error: (await page.locator('[data-testid="booking-error"]').count()) > 0,
    confirmation: (await page.locator('[data-testid="confirmation"]').count()) > 0,
    sameRequest: lostPost.key === lostGate.info.key && JSON.stringify(lostPost.body) === JSON.stringify(lostGate.info.body),
  };
  if (lostGate.info.status !== 201 || !report.checks.lost.uncertain || report.checks.lost.error || report.checks.lost.confirmation) {
    fail(`lost response ${JSON.stringify(report.checks.lost)}`);
  }
  const during = await ownedReferences(page);
  const added = during.references.filter((item) => !beforeLostList.references.includes(item));
  if (added.length !== 1 || added[0] !== lostGate.info.reference) fail(`lost commit references ${added.join(',')}`);
  if ((await page.locator('[data-testid="confirmation-reference"]').count()) !== 0) fail('lost response painted a reference');
  await shotThemes(page, 'uncertain');

  const upgrade = await page.evaluate(async () => {
    const exported = await fetch('/_test/export');
    const text = await exported.text();
    let track = '';
    let formatVersion = null;
    try {
      const parsed = JSON.parse(text);
      track = parsed.track;
      formatVersion = parsed.format_version;
    } catch {
      track = 'unreadable';
    }
    const imported = await fetch('/_test/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: text,
    });
    const name = document.querySelector('[data-testid="current-user"]')?.textContent ?? '';
    return {
      exportStatus: exported.status,
      importStatus: imported.status,
      track,
      formatVersion,
      stillNamed: name.includes('Ada'),
      uncertain: Boolean(document.querySelector('[data-testid="booking-uncertain"]')),
      confirmation: Boolean(document.querySelector('[data-testid="confirmation"]')),
    };
  });
  report.checks.upgrade = upgrade;
  if (upgrade.exportStatus !== 200 || upgrade.importStatus !== 204 || upgrade.track !== 'tablekeeper' || upgrade.formatVersion !== 1) {
    fail(`upgrade ${upgrade.exportStatus}/${upgrade.importStatus}/${upgrade.track}/${upgrade.formatVersion}`);
  }
  if (!upgrade.stillNamed || !upgrade.uncertain || upgrade.confirmation) fail('upgrade disturbed the pending form');

  const beforeRetry = posts.length;
  const retryResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && pathname(response.url()) === '/reservations',
    { timeout: 10000 },
  );
  await page.locator('[data-testid="booking-submit"]').click();
  const retried = await retryResponse;
  const retryBody = await retried.json();
  const retryPost = posts[beforeRetry];
  await page.locator('[data-testid="confirmation-reference"]').waitFor();
  const shown = await page.locator('[data-testid="confirmation-reference"]').innerText();
  const afterRetry = await ownedReferences(page);
  report.checks.retry = {
    status: retried.status(),
    sameKey: retryPost.key === lostGate.info.key && retryPost.key.length > 0,
    sameBody: JSON.stringify(retryPost.body) === JSON.stringify(lostGate.info.body),
    reference: shown,
    matchesCommit: shown === lostGate.info.reference,
    recordCount: afterRetry.references.filter((item) => item === lostGate.info.reference).length,
    uncertain: (await page.locator('[data-testid="booking-uncertain"]').count()) > 0,
    error: (await page.locator('[data-testid="booking-error"]').count()) > 0,
  };
  if (retried.status() !== 200 || !report.checks.retry.sameKey || !report.checks.retry.sameBody || shown !== lostGate.info.reference) {
    fail(`retry ${JSON.stringify(report.checks.retry)}`);
  }
  if (report.checks.retry.recordCount !== 1 || report.checks.retry.uncertain || report.checks.retry.error) {
    fail(`retry residue ${JSON.stringify(report.checks.retry)}`);
  }

  await page.locator('a[href="/lookup"]').click();
  await page.locator('[data-testid="lookup-reference-input"]').fill(lostGate.info.reference);
  await page.locator('[data-testid="lookup-submit"]').click();
  await page.locator('[data-testid="reservation-detail"]').waitFor();
  if ((await page.locator('[data-testid="reservation-status"]').innerText()) !== 'confirmed') fail('lookup status');
  if (!(await page.locator('[data-testid="reservation-tables"]').innerText()).includes('2')) fail('lookup tables');
  if (!(await page.locator('[data-testid="reservation-detail"]').innerText()).includes('Zum Anker')) fail('lookup restaurant');
  if (!(await page.locator('[data-testid="current-user"]').innerText()).includes(displayName)) fail('name missing during lookup');
  await shotThemes(page, 'lookup');
  await page.locator('[data-testid="reservation-cancel-button"]').click();
  await page.waitForFunction(() => document.querySelector('[data-testid="reservation-status"]')?.textContent === 'cancelled');
  if ((await page.locator('[data-testid="reservation-cancel-button"]').count()) !== 0) fail('cancel button remained');
  await shotThemes(page, 'cancelled');

  await page.locator('nav a[href="/"]').click();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  await search(page, 'r_anker', pastThursday, 2);
  await page.locator('[data-testid="slot-t_1-18:00"]').click();
  const pastResponse = page.waitForResponse(
    (response) => response.request().method() === 'POST' && pathname(response.url()) === '/reservations',
    { timeout: 10000 },
  );
  await page.locator('[data-testid="booking-submit"]').click();
  const past = await pastResponse;
  const pastBody = await past.json();
  if (past.status() !== 201) fail(`past booking ${past.status()} ${pastBody.error?.code ?? ''}`);
  await page.locator('a[href="/lookup"]').click();
  await page.locator('[data-testid="lookup-reference-input"]').fill(pastBody.reference);
  await page.locator('[data-testid="lookup-submit"]').click();
  await page.locator('[data-testid="reservation-cancel-button"]').waitFor();
  const cancelResponse = page.waitForResponse((response) => pathname(response.url()).endsWith('/cancel'), { timeout: 10000 });
  await page.locator('[data-testid="reservation-cancel-button"]').click();
  const refused = await cancelResponse;
  const refusedBody = await refused.json();
  await page.locator('[data-testid="reservation-error"]').waitFor();
  report.checks.cutoff = {
    status: refused.status(),
    code: refusedBody.error?.code ?? '',
    still: await page.locator('[data-testid="reservation-status"]').innerText(),
    button: (await page.locator('[data-testid="reservation-cancel-button"]').count()) === 1,
  };
  if (refused.status() !== 409 || refusedBody.error?.code !== 'cutoff_passed' || report.checks.cutoff.still !== 'confirmed' || !report.checks.cutoff.button) {
    fail(`cutoff ${JSON.stringify(report.checks.cutoff)}`);
  }

  await page.locator('[data-testid="lookup-reference-input"]').fill('NOSUCH');
  await page.locator('[data-testid="lookup-submit"]').click();
  await page.locator('[data-testid="reservation-error"]').waitFor();
  if ((await page.locator('[data-testid="reservation-detail"]').count()) !== 0) fail('unknown reference showed a record');

  await page.locator('[data-testid="logout-button"]').click();
  await page.locator('[data-testid="current-user"]').waitFor({ state: 'detached' });
  await page.locator('a[href="/login"]').click();
  await page.locator('[data-testid="login-email"]').fill(rivalEmail);
  await page.locator('[data-testid="login-password"]').fill(password);
  await page.locator('[data-testid="login-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();
  if (!(await page.locator('[data-testid="current-user"]').innerText()).includes('Bea')) fail('rival name missing');
  await page.locator('a[href="/lookup"]').click();
  await page.locator('[data-testid="lookup-reference-input"]').fill(referenceA);
  await page.locator('[data-testid="lookup-submit"]').click();
  await page.locator('[data-testid="reservation-error"]').waitFor();
  report.checks.nonOwner = {
    error: (await page.locator('[data-testid="reservation-error"]').count()) === 1,
    detail: (await page.locator('[data-testid="reservation-detail"]').count()) > 0,
  };
  if (!report.checks.nonOwner.error || report.checks.nonOwner.detail) fail('non-owner lookup leaked a booking');
  await page.locator('[data-testid="logout-button"]').click();
  await page.locator('[data-testid="current-user"]').waitFor({ state: 'detached' });

  report.checks.pageErrors = errors.length;
  if (errors.length > 0) fail(`page errors ${errors.join(' | ')}`);
  report.checks.references = {
    first: referenceA,
    second: referenceB,
    recovered: lostGate.info.reference,
    past: pastBody.reference,
  };
}

async function captureStates(browser, width, theme) {
  const context = await browser.newContext({
    viewport: { width, height: width < 700 ? 900 : 1000 },
    deviceScaleFactor: 1,
    colorScheme: theme,
  });
  await context.addInitScript((mode) => {
    localStorage.setItem('chaaya-theme', mode);
  }, theme);
  const page = await context.newPage();
  const errors = watch(page);
  const prefix = `${width}-${theme}`;

  let releaseLoading = () => {};
  const loadingGate = new Promise((resolve) => {
    releaseLoading = resolve;
  });
  await page.route('**/*', async (route) => {
    if (pathname(route.request().url()) === '/restaurants') await loadingGate;
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
  await noHorizontalScroll(page, `${prefix} error`);
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
    const free = await page.locator('[data-testid="slot-t_1-18:00"]').innerText();
    if (!free.includes('Free') && !free.includes('Taken') && !free.includes('Held')) {
      fail(`375 cell text missing a state word: ${free}`);
    }
    report.checks.phoneCell = free.replace(/\s+/g, ' ').trim();
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
  const available = page.locator('[data-testid^="slot-"][data-available="true"]').first();
  await available.click();
  await page.locator('[data-testid="booking-form"]').waitFor();
  await shot(page, `${prefix}-selected`);
  await noHorizontalScroll(page, `${prefix} selected`);
  if (errors.length > 0) fail(`${prefix} page errors ${errors.join(' | ')}`);
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
  watch(page);
  const email = `film.${width}.${Date.now()}@example.com`;
  await page.goto(`${base}/signup`);
  await page.locator('[data-testid="signup-display-name"]').fill(displayName);
  await page.locator('[data-testid="signup-email"]').fill(email);
  await page.locator('[data-testid="signup-password"]').fill(password);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  await search(page, 'r_anker', thursday, 2);
  await page.locator('[data-testid^="slot-"][data-available="true"]').first().click();
  await page.locator('[data-testid="booking-form"]').waitFor();
  await page.locator('[data-testid="booking-submit"]').click();
  await page.locator('[data-testid="confirmation-reference"]').waitFor();
  const reference = await page.locator('[data-testid="confirmation-reference"]').innerText();
  await page.locator('a[href="/lookup"]').click();
  await page.locator('[data-testid="lookup-reference-input"]').fill(reference);
  await page.locator('[data-testid="lookup-submit"]').click();
  await page.locator('[data-testid="reservation-cancel-button"]').waitFor();
  await page.locator('[data-testid="reservation-cancel-button"]').click();
  await page.waitForFunction(() => document.querySelector('[data-testid="reservation-status"]')?.textContent === 'cancelled');
  await page.locator('[data-testid="logout-button"]').click();
  await page.locator('[data-testid="current-user"]').waitFor({ state: 'detached' });
  const video = page.video();
  await context.close();
  if (video) {
    const saved = await video.path();
    const bytes = await readFile(saved);
    await writeFile(join(dir, `main-flow-${width}.webm`), bytes);
    log(`video main-flow-${width} ${bytes.length}`);
  }
}

async function reducedMotion(browser) {
  const dir = join(evidence, 'video');
  await mkdir(dir, { recursive: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    reducedMotion: 'reduce',
    recordVideo: { dir, size: { width: 1280, height: 900 } },
  });
  const page = await context.newPage();
  watch(page);
  await page.goto(`${base}/signup`);
  await page.locator('[data-testid="signup-email"]').fill(`motion.${Date.now()}@example.com`);
  await page.locator('[data-testid="signup-password"]').fill(password);
  await page.locator('[data-testid="signup-display-name"]').fill(displayName);
  await page.locator('[data-testid="signup-submit"]').click();
  await page.locator('[data-testid="current-user"]').waitFor();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="restaurant-select"] option').length >= 2);
  await search(page, 'r_nord', thursday, 2);
  const cell = page.locator('[data-testid^="slot-"][data-available="true"]').first();
  const cellId = await cell.getAttribute('data-testid');
  await cell.click();
  const selected = await page.locator(`[data-testid="${cellId}"]`).getAttribute('data-selected');
  const tableId = /^slot-(.+)-\d{2}:\d{2}$/.exec(cellId ?? '')?.[1] ?? '';
  const plan = tableId ? await page.locator(`[data-testid="plan-${tableId}"]`).getAttribute('data-selected') : null;
  report.checks.reducedMotionSelected = { cell: selected, plan, cellId };
  if (selected !== 'true' || plan !== 'true') fail(`reduced motion selected cell ${selected} plan ${plan} ${cellId}`);
  await page.locator('[data-testid="booking-submit"]').click();
  await page.locator('[data-testid="confirmation-reference"]').waitFor();
  const reference = (await page.locator('[data-testid="confirmation-reference"]').innerText()).trim();
  report.checks.reducedMotionReference = /^[A-Z0-9]{6,12}$/.test(reference);
  if (!report.checks.reducedMotionReference) fail('reduced motion confirmation was not immediate');
  const video = page.video();
  await context.close();
  if (video) {
    const saved = await video.path();
    const bytes = await readFile(saved);
    await writeFile(join(dir, 'reduced-motion-1280.webm'), bytes);
    log(`video reduced-motion-1280 ${bytes.length}`);
  }
}

async function main() {
  await mkdir(evidence, { recursive: true });
  await scanDist();
  await waitHealth();
  await reset();
  const browser = await launch();
  try {
    const dir = join(evidence, 'video');
    await mkdir(dir, { recursive: true });
    const context = await browser.newContext({
      viewport: { width: 1280, height: 1000 },
      colorScheme: 'light',
      recordVideo: { dir, size: { width: 1280, height: 1000 } },
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    const posts = [];
    watchPosts(page, posts);
    await behavior(page, posts);
    const video = page.video();
    await context.close();
    if (video) {
      const saved = await video.path();
      const bytes = await readFile(saved);
      await writeFile(join(dir, 'main-flow-1280.webm'), bytes);
      log(`video main-flow-1280 ${bytes.length}`);
    }
    for (const width of [375, 1280]) {
      for (const theme of ['light', 'dark']) {
        await captureStates(browser, width, theme);
      }
    }
    await recordFlow(browser, 375);
    await reducedMotion(browser);
    if (report.offOrigin.length > 0) fail(`off-origin requests ${report.offOrigin.length}`);
    if (report.fontRequests.length === 0) fail('no local font requests');
    if (report.fontStatuses.some((status) => status !== 200)) fail(`font status ${report.fontStatuses.join(',')}`);
    if (report.publicAuthorization.some(Boolean)) fail('public read sent a bearer token');
    report.checks.fontRequestCount = report.fontRequests.length;
    report.checks.fontStatusCount = report.fontStatuses.length;
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
