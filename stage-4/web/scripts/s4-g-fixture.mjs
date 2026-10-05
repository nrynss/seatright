/**
 * FIXTURE TRANSPORT. Playwright answers the built stage-4 page.
 * This is not the Go service, not an applied plan on a live API, and not an image migration.
 *
 * Environment:
 *   PORT      preview port, default 4184
 *   EVIDENCE  output directory, default the S4-G evidence folder
 *   CHROME    Chromium executable, default the sandbox Playwright build
 *
 * Frames are named fixture-transport-* and the report label is FIXTURE TRANSPORT.
 */
import { spawn } from 'node:child_process';
import { chmod, mkdir, readdir, rm, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { chromium } from 'playwright';

process.umask(0o077);

const webDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const port = Number(process.env.PORT ?? 4184);
const origin = `http://127.0.0.1:${port}`;
const evidence =
  process.env.EVIDENCE ?? '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S4-G';
const frameDir = join(evidence, 'frames');
const videoDir = join(evidence, 'videos');
const rawDir = join(evidence, 'video-raw');
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const TOKEN = 'fixture-session';
const CLOCKS = ['18:00', '19:00', '20:30'];
const DATE = '2027-06-17';
const CLOCK_DATE = '2027-06-24';
const REFERENCE = 'REPAIR01';

const detail = {
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
    { id: 't_3', label: '3', capacity: 6 },
  ],
  combinable: [
    ['t_1', 't_2'],
    ['t_2', 't_3'],
  ],
};

const policy0 = {
  policy_version: 0,
  slot_minutes: 30,
  reservation_duration_minutes: 90,
  cancellation_cutoff_minutes: 120,
  opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
  capacities: { t_1: 2, t_2: 4, t_3: 6 },
};

const clockTerms = {
  policy_version: 1,
  slot_minutes: 30,
  reservation_duration_minutes: 60,
  cancellation_cutoff_minutes: 120,
  opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
  capacities: { t_1: 2, t_2: 4, t_3: 6 },
};

function slot(date, time, availableIds, options) {
  return {
    starts_at_local: `${date}T${time}`,
    starts_at: `${date}T${time}:00+02:00`,
    available_table_ids: [...availableIds],
    available_options: options.map((option) => ({ table_ids: [...option.table_ids], capacity: option.capacity })),
  };
}

const openOptions = [
  { table_ids: ['t_3'], capacity: 6 },
  { table_ids: ['t_1', 't_2'], capacity: 6 },
  { table_ids: ['t_2', 't_3'], capacity: 10 },
];

function beforeFloor() {
  return {
    restaurant_id: 'r_anker',
    date: DATE,
    timezone: 'Europe/Berlin',
    slots: CLOCKS.map((time) => slot(DATE, time, ['t_3'], openOptions)),
  };
}

function afterFloor() {
  return {
    restaurant_id: 'r_anker',
    date: DATE,
    timezone: 'Europe/Berlin',
    slots: [
      slot(DATE, '18:00', [], []),
      slot(DATE, '19:00', [], []),
      slot(DATE, '20:30', ['t_3'], openOptions),
    ],
  };
}

function pairReceipt() {
  return {
    reservation_id: 'res_repair',
    reference: REFERENCE,
    restaurant_id: 'r_anker',
    table_ids: ['t_1', 't_2'],
    party_size: 6,
    status: 'confirmed',
    starts_at_local: `${DATE}T19:00`,
    starts_at: `${DATE}T19:00:00+02:00`,
    ends_at: `${DATE}T20:30:00+02:00`,
    created_at: '2026-10-05T08:15:00+00:00',
    revision: 1,
    accepted_terms: policy0,
  };
}

function repairedReceipt() {
  return {
    reservation_id: 'res_repair',
    reference: REFERENCE,
    restaurant_id: 'r_anker',
    table_id: 't_3',
    table_ids: ['t_3'],
    party_size: 6,
    status: 'confirmed',
    starts_at_local: `${DATE}T19:00`,
    starts_at: `${DATE}T19:00:00+02:00`,
    ends_at: `${DATE}T20:30:00+02:00`,
    created_at: '2026-10-05T08:15:00+00:00',
    revision: 2,
    accepted_terms: policy0,
  };
}

function clockReceipt(overrides) {
  return {
    ...pairReceipt(),
    starts_at_local: `${CLOCK_DATE}T20:00`,
    starts_at: `${CLOCK_DATE}T20:00:00+02:00`,
    ends_at: `${CLOCK_DATE}T21:00:00+02:00`,
    revision: 3,
    accepted_terms: clockTerms,
    ...overrides,
  };
}

function lookupBody(kind) {
  if (kind === 'before') return pairReceipt();
  if (kind === 'after') return repairedReceipt();
  if (kind === 'clock') return clockReceipt();
  if (kind === 'cancelled') return clockReceipt({ status: 'cancelled', revision: 4 });
  return clockReceipt({
    starts_at_local: `${CLOCK_DATE}T18:30`,
    starts_at: `${CLOCK_DATE}T18:30:00+02:00`,
    ends_at: `${CLOCK_DATE}T19:30:00+02:00`,
    exception: true,
  });
}

function elapsedMinutes(startsAt, endsAt) {
  return (Date.parse(endsAt) - Date.parse(startsAt)) / 60000;
}

function assertFixtureClocks() {
  const expectedKeys = [
    'cancellation_cutoff_minutes',
    'capacities',
    'opening_hours',
    'policy_version',
    'reservation_duration_minutes',
    'slot_minutes',
  ];
  const cases = [
    ['amended', lookupBody('clock'), 1, 60, `${CLOCK_DATE}T20:00`, `${CLOCK_DATE}T21:00:00+02:00`],
    ['cancelled', lookupBody('cancelled'), 1, 60, `${CLOCK_DATE}T20:00`, `${CLOCK_DATE}T21:00:00+02:00`],
    ['exception', lookupBody('exception'), 1, 60, `${CLOCK_DATE}T18:30`, `${CLOCK_DATE}T19:30:00+02:00`],
    ['pair-receipt', pairReceipt(), 0, 90, `${DATE}T19:00`, `${DATE}T20:30:00+02:00`],
    ['repair', repairedReceipt(), 0, 90, `${DATE}T19:00`, `${DATE}T20:30:00+02:00`],
  ];
  let ok = true;
  for (const [name, record, version, duration, startLocal, endStamp] of cases) {
    const terms = record.accepted_terms;
    const keys = Object.keys(terms).sort();
    const elapsed = elapsedMinutes(record.starts_at, record.ends_at);
    const same =
      JSON.stringify(keys) === JSON.stringify(expectedKeys) &&
      !Object.hasOwn(terms, 'effective_from') &&
      terms.policy_version === version &&
      terms.reservation_duration_minutes === duration &&
      record.starts_at_local === startLocal &&
      record.starts_at === `${startLocal}:00+02:00` &&
      record.ends_at === endStamp &&
      elapsed === duration;
    if (!same) {
      ok = false;
      fail(
        `${name} record version=${terms.policy_version} duration=${terms.reservation_duration_minutes} elapsed=${elapsed} start=${record.starts_at_local} end=${record.ends_at}`,
      );
    }
  }
  const capacity = (id) => detail.tables.find((table) => table.id === id)?.capacity ?? 0;
  const pairCapacity = capacity('t_1') + capacity('t_2');
  const exception = lookupBody('exception');
  if (pairCapacity < 5 || capacity('t_3') < 6) {
    ok = false;
    fail(`fixture capacity pair=${pairCapacity} table3=${capacity('t_3')}`);
  }
  if (
    JSON.stringify(exception.table_ids) !== JSON.stringify(['t_1', 't_2']) ||
    exception.revision !== 3 ||
    exception.exception !== true ||
    exception.status !== 'confirmed'
  ) {
    ok = false;
    fail(`exception record tables=${JSON.stringify(exception.table_ids)} revision=${exception.revision}`);
  }
  if (ok) {
    pass(
      'clock-fidelity',
      'amended and cancelled terms are version 1 for 60 minutes; pair receipt and repair stay version 0 for 90; exception ends 60 minutes after 18:30; pair capacity holds 5 and table 3 holds 6',
    );
  }
}

const proof = {
  label: 'FIXTURE TRANSPORT',
  note: 'Route interception on the built preview. Not a live seating repair and not a stage upgrade.',
  origin,
  port,
  shots: [],
  videos: [],
  posts: [],
  contrast: [],
  reveal: [],
  paint: [],
  bounds: [],
  layout: [],
  checks: [],
  pageErrors: [],
  consoleErrors: [],
  offOrigin: [],
  failures: [],
};

function fail(message) {
  proof.failures.push(message);
  console.error(`FAIL ${message}`);
}

function pass(name, detailText) {
  proof.checks.push({ name, detail: detailText });
  console.log(`PASS ${name} ${detailText}`);
}

function json(route, status, body) {
  return route.fulfill({
    status,
    contentType: 'application/json; charset=utf-8',
    body: JSON.stringify(body),
  });
}

function redactPost(entry, index, posts) {
  return {
    index,
    method: 'POST',
    path: '/reservations',
    party_size: entry.body.party_size,
    table_id: entry.body.table_id ?? null,
    table_ids: entry.body.table_ids ?? null,
    starts_at_local: entry.body.starts_at_local,
    restaurant_id: entry.body.restaurant_id,
    hadAuthorization: entry.hadAuthorization,
    sameKeyAsFirst: index === 0 ? true : entry.key === posts[0].key,
    sameBodyAsFirst: index === 0 ? true : JSON.stringify(entry.body) === JSON.stringify(posts[0].body),
    outcome: entry.outcome,
  };
}

async function waitForPreview(child) {
  const started = Date.now();
  let stderr = '';
  child.stderr?.on('data', (chunk) => {
    stderr += String(chunk);
  });
  child.stdout?.on('data', () => {});
  while (Date.now() - started < 20000) {
    if (child.exitCode != null) throw new Error(`preview exited ${child.exitCode}: ${stderr}`);
    try {
      const response = await fetch(origin);
      if (response.ok) return;
    } catch {
      // The preview port is still opening.
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`preview did not answer: ${stderr}`);
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
  proof.reveal.push({ label, selector, ...box });
  console.log(
    `REVEAL ${label} present=${box.present} scrollY=${box.scrollY} top=${box.top} visible=${box.visible} view=${box.viewW}x${box.viewH}`,
  );
  if (!box.present) fail(`${label} missing ${selector}`);
  else if (box.visible < minVisible || box.top >= box.viewH - 8) fail(`${label} not in view visible=${box.visible} top=${box.top}`);
  else pass(`reveal-${label}`, `visible ${box.visible}px`);
  return box;
}

async function shot(page, name) {
  const file = join(frameDir, `fixture-transport-${name}.png`);
  await page.screenshot({ path: file });
  proof.shots.push(file);
  console.log(`SHOT ${name}`);
}

function install(page, state) {
  return page.route('**/*', async (route) => {
    const request = route.request();
    let url;
    try {
      url = new URL(request.url());
    } catch {
      await route.abort();
      return;
    }
    if (url.origin !== origin) {
      proof.offOrigin.push(url.href);
      await route.abort();
      return;
    }
    if (/fonts\.googleapis|cdn\.|unpkg|jsdelivr|fontshare|typekit/i.test(url.href)) {
      proof.offOrigin.push(url.href);
      await route.abort();
      return;
    }
    const path = url.pathname;
    const forbidden = path.includes('/policies') || path.includes('/series') || path.includes('replan');
    if (forbidden) {
      state.forbidden += 1;
      return json(route, 404, { error: { code: 'not_found', message: 'Not part of this fixture.' } });
    }
    const api =
      path === '/restaurants' ||
      path.startsWith('/restaurants/') ||
      path === '/availability' ||
      path === '/reservations' ||
      path.startsWith('/reservations/');
    if (!api) {
      await route.continue();
      return;
    }
    const authorization = request.headers().authorization || '';
    if (path === '/restaurants' && request.method() === 'GET') {
      if (authorization) state.publicBearer += 1;
      if (state.holdCatalog) await state.gate;
      return json(route, 200, {
        restaurants: [{ id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' }],
      });
    }
    if (path === '/restaurants/r_anker' && request.method() === 'GET') {
      if (authorization) state.publicBearer += 1;
      state.detailReads += 1;
      return json(route, 200, detail);
    }
    if (path === '/availability' && request.method() === 'GET') {
      if (authorization) state.publicBearer += 1;
      const party = url.searchParams.get('party_size');
      const explain = url.searchParams.get('explain');
      state.availabilityReads.push({ party, explain, floor: state.floor });
      if (explain != null) state.forbidden += 1;
      if (state.floor === 'empty') {
        return json(route, 200, {
          restaurant_id: 'r_anker',
          date: DATE,
          timezone: 'Europe/Berlin',
          slots: [],
        });
      }
      if (state.floor === 'error') {
        return json(route, 422, {
          error: { code: 'validation_failed', message: 'The floor could not be read.' },
        });
      }
      return json(route, 200, state.floor === 'after' ? afterFloor() : beforeFloor());
    }
    if (path === '/reservations' && request.method() === 'POST') {
      const body = JSON.parse(request.postData() || '{}');
      const key = request.headers()['idempotency-key'] || '';
      const hadAuthorization = Boolean(authorization);
      const outcome = state.nextPost;
      state.posts.push({ body, key, hadAuthorization, outcome });
      if (outcome === 'drop') {
        state.nextPost = 'receipt';
        await route.abort('failed');
        return;
      }
      if (outcome === 'refuse') {
        return json(route, 409, { error: { code: 'table_unavailable', message: 'That seating was just taken.' } });
      }
      return json(route, 200, pairReceipt());
    }
    if (path === `/reservations/${REFERENCE}` && request.method() === 'GET') {
      if (!authorization) state.lookupUnauthenticated += 1;
      state.lookupReads += 1;
      return json(route, 200, lookupBody(state.lookupKind));
    }
    return json(route, 404, { error: { code: 'not_found', message: 'Not part of this fixture.' } });
  });
}

async function openContext(browser, { width, height, theme, reduced = false, videoName = '' }) {
  const context = await browser.newContext({
    viewport: { width, height },
    reducedMotion: reduced ? 'reduce' : 'no-preference',
    recordVideo: videoName ? { dir: rawDir, size: { width, height } } : undefined,
    serviceWorkers: 'block',
  });
  await context.addInitScript(
    ({ mode, session }) => {
      localStorage.setItem('chaaya-theme', mode);
      localStorage.setItem('tablekeeper-session', JSON.stringify(session));
    },
    { mode: theme, session: { token: TOKEN, displayName: '', userId: 'u_blank' } },
  );
  const page = await context.newPage();
  page.setDefaultTimeout(15000);
  page.on('pageerror', (error) => proof.pageErrors.push(String(error.message ?? error)));
  page.on('console', (message) => {
    if (message.type() === 'error') proof.consoleErrors.push(message.text());
  });
  return { context, page };
}

async function closeContext(context, page, videoName) {
  const video = page.video();
  await context.close();
  if (!video || !videoName) return;
  const target = join(videoDir, videoName);
  await video.saveAs(target);
  proof.videos.push(target);
  console.log(`VIDEO ${videoName}`);
}

async function assertChrome(page, label) {
  const chromeState = await page.evaluate(() => {
    const nav = document.querySelector('nav[aria-label="Primary"]');
    const labels = nav ? [...nav.querySelectorAll('a')].map((link) => (link.textContent || '').trim()) : [];
    const user = document.querySelector('[data-testid="current-user"]');
    return {
      labels,
      userText: user ? user.textContent ?? '' : null,
      logout: Boolean(document.querySelector('[data-testid="logout-button"]')),
      themes: ['theme-light', 'theme-dark', 'theme-system'].every((id) => document.querySelector(`[data-testid="${id}"]`)),
    };
  });
  if (chromeState.labels.join('|') !== 'Search|Look up') fail(`${label} nav ${chromeState.labels.join('|')}`);
  if (chromeState.userText !== '') fail(`${label} current-user ${JSON.stringify(chromeState.userText)}`);
  if (!chromeState.logout || !chromeState.themes) fail(`${label} chrome controls missing`);
  else pass(`chrome-${label}`, 'blank signed-in name, no auth links');
}

async function searchParty(page, party) {
  await knownScroll(page);
  await page.getByTestId('party-size-input').fill(String(party));
  await page.getByTestId('search-button').click();
}

async function measureSelected(page, label) {
  const paint = await page.evaluate(() => {
    const svg = document.querySelector('svg.room-scene');
    if (!svg) return { missing: true };
    const nodes = [...svg.querySelectorAll('*')];
    const indexOf = (selector) => nodes.findIndex((node) => node.matches(selector));
    const planNames = [...svg.querySelectorAll('.plan-name')].map((node) => (node.textContent || '').trim());
    const plates = [...svg.querySelectorAll('.plate-label')].map((node) => {
      const rect = node.getBoundingClientRect();
      return {
        text: (node.textContent || '').trim(),
        font: Number(node.getAttribute('font-size')),
        w: rect.width,
        h: rect.height,
      };
    });
    const seatLines = [...svg.querySelectorAll('.plan-seats')].map((node) => ({
      text: (node.textContent || '').trim(),
      font: Number(node.getAttribute('font-size')),
    }));
    const badge = document.querySelector('[data-testid="plan-t_1+t_2"]');
    const badgeRect = badge ? badge.getBoundingClientRect() : null;
    const hits = [];
    for (const node of svg.querySelectorAll('.plate-label')) {
      const rect = node.getBoundingClientRect();
      if (rect.width < 2 || rect.bottom < 0 || rect.top > window.innerHeight) continue;
      const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
      hits.push({ text: (node.textContent || '').trim(), hit: hit ? hit.getAttribute('class') || hit.tagName : '' });
    }
    const pageOverflow = document.documentElement.scrollWidth > document.documentElement.clientWidth + 1;
    return {
      link: indexOf('.pair-link'),
      top: indexOf('.table-top'),
      plate: indexOf('.plate-label'),
      badge: indexOf('.pair-badge'),
      planNames,
      plates,
      seatLines,
      badgeBox: badgeRect
        ? { w: Math.round(badgeRect.width * 10) / 10, h: Math.round(badgeRect.height * 10) / 10 }
        : null,
      aria: badge ? badge.getAttribute('aria-label') : '',
      hits,
      pageOverflow,
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    };
  });
  proof.paint.push({ label, ...paint });
  if (paint.missing) fail(`${label} floor missing`);
  else if (!(paint.link >= 0 && paint.top > paint.link && paint.plate > paint.top && paint.badge > paint.plate)) {
    fail(`${label} paint order ${paint.link}/${paint.top}/${paint.plate}/${paint.badge}`);
  } else pass(`paint-${label}`, `link ${paint.link} top ${paint.top} plate ${paint.plate} badge ${paint.badge}`);
  if ((paint.planNames || []).length !== 0) fail(`${label} plan-name captions`);
  for (const plate of paint.plates || []) {
    if (plate.font < 13 || plate.w < 2) fail(`${label} plate ${plate.text} font ${plate.font}`);
  }
  const lines = paint.seatLines || [];
  if (!lines.some((line) => line.text === '2 seats' && line.font >= 13)) fail(`${label} table 1 seat ink`);
  if (!lines.some((line) => line.text === '4 seats')) fail(`${label} table 2 seat ink`);
  if (!lines.some((line) => line.text === '6 seats')) fail(`${label} table 3 seat ink`);
  if (!String(paint.aria).includes('6 seats')) fail(`${label} badge aria ${paint.aria}`);
  if (paint.pageOverflow) fail(`${label} page overflow ${paint.scrollWidth}/${paint.clientWidth}`);
  else pass(`overflow-${label}`, `${paint.clientWidth}px`);
  for (const hit of paint.hits || []) {
    if (/pair-link/.test(hit.hit)) fail(`${label} connector covers ${hit.text}`);
  }

  const grid = await page.evaluate((clocks) => {
    const root = document.querySelector('[data-testid="availability-grid"]');
    if (!root) return { present: false };
    const outside = [];
    for (const cell of root.querySelectorAll('.cell')) {
      const word = cell.querySelector('.state-word');
      if (!word) continue;
      const range = document.createRange();
      range.selectNodeContents(word);
      const rects = [...range.getClientRects()].filter((rect) => rect.width > 0.5 && rect.height > 0.5);
      const box = cell.getBoundingClientRect();
      for (const rect of rects) {
        if (rect.left < box.left - 0.5 || rect.right > box.right + 0.5 || rect.top < box.top - 0.5 || rect.bottom > box.bottom + 0.5) {
          outside.push(cell.getAttribute('data-testid'));
        }
      }
    }
    const clocksFound = [...root.querySelectorAll('[data-testid^="slot-t_1-"]')]
      .map((node) => (node.getAttribute('data-testid') || '').replace('slot-t_1-', ''))
      .filter((time) => !time.includes('+'));
    const pairHead = [...root.querySelectorAll('.rowhead')].map((node) => (node.textContent || '').replace(/\s+/g, ' ').trim());
    const flags = (id) => document.querySelector(`[data-testid="${id}"]`);
    const pair = flags('slot-t_1+t_2-19:00');
    return {
      present: true,
      cells: root.querySelectorAll('.cell').length,
      outside,
      clocks: clocksFound,
      expected: clocks,
      pairHead,
      selected: pair?.getAttribute('data-selected'),
      available: pair?.getAttribute('data-available'),
      pageOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    };
  }, CLOCKS);
  proof.bounds.push({ label, ...grid });
  if (!grid.present) fail(`${label} grid missing`);
  if ((grid.outside || []).length) fail(`${label} state-word ink outside ${grid.outside.join(',')}`);
  if ((grid.clocks || []).join(',') !== CLOCKS.join(',')) fail(`${label} clocks ${(grid.clocks || []).join(',')}`);
  if (!(grid.pairHead || []).some((text) => text.includes('Table 1 and Table 2') && text.includes('6 seats together'))) {
    fail(`${label} pair seat text`);
  }
  if (grid.selected !== 'true' || grid.available !== 'true') fail(`${label} selected pair flags ${grid.selected}/${grid.available}`);
  else pass(`grid-${label}`, `${grid.cells} cells, ink inside, 6 seats together`);

  const painted = await page.evaluate(() => {
    const cell = document.querySelector('[data-testid="slot-t_1+t_2-19:00"]');
    const word = cell?.querySelector('.state-word');
    if (!cell || !word) return null;
    const style = getComputedStyle(word);
    let node = word;
    let background = '';
    while (node) {
      const color = getComputedStyle(node).backgroundColor;
      const match = color.match(/rgba?\(\s*[\d.]+[,\s]+[\d.]+[,\s]+[\d.]+(?:[,\s/]+([\d.]+))?\s*\)/);
      const alpha = match && match[1] !== undefined ? Number(match[1]) : match ? 1 : 0;
      if (color && color !== 'transparent' && alpha !== 0) {
        background = color;
        break;
      }
      node = node.parentElement;
    }
    return {
      word: (word.textContent || '').trim(),
      color: style.color,
      background,
    };
  });
  const ratio = painted ? contrastRatio(painted.color, painted.background) : null;
  proof.contrast.push({
    label,
    word: painted?.word ?? '',
    foreground: painted ? hexOf(painted.color) : '',
    background: painted ? hexOf(painted.background) : '',
    ratio: ratio == null ? null : Math.round(ratio * 1000) / 1000,
  });
  if (painted?.word !== 'Held') fail(`${label} selected word ${painted?.word ?? ''}`);
  if (ratio == null || ratio < 4.5) fail(`${label} selected pair contrast ${ratio}`);
  else pass(`contrast-${label}`, `${proof.contrast[proof.contrast.length - 1].ratio}`);
}

async function runWalk(browser, { width, height, theme, videoName = '' }) {
  let release = () => {};
  const gate = new Promise((resolve) => {
    release = resolve;
  });
  const state = {
    holdCatalog: true,
    gate,
    floor: 'before',
    nextPost: 'drop',
    lookupKind: 'before',
    posts: [],
    detailReads: 0,
    lookupReads: 0,
    availabilityReads: [],
    publicBearer: 0,
    lookupUnauthenticated: 0,
    forbidden: 0,
  };
  const { context, page } = await openContext(browser, { width, height, theme, videoName });
  const tag = `${width}-${theme}`;
  try {
    await install(page, state);
    await page.goto(`${origin}/`, { waitUntil: 'domcontentloaded' });
    await page.getByTestId('loading-state').waitFor();
    await assertRevealed(page, `loading-${tag}`, '[data-testid="loading-state"]', 24);
    await shot(page, `loading-${tag}`);
    release();
    state.holdCatalog = false;
    await page.getByTestId('restaurant-select').waitFor();
    await assertChrome(page, tag);

    state.floor = 'empty';
    await searchParty(page, 6);
    await page.getByTestId('no-slots').waitFor();
    await settledScroll(page);
    await assertRevealed(page, `empty-${tag}`, '[data-testid="no-slots"]', 24);
    await shot(page, `empty-${tag}`);

    state.floor = 'error';
    await knownScroll(page);
    await page.getByTestId('search-button').click();
    await page.getByTestId('search-error').waitFor();
    await settledScroll(page);
    await assertRevealed(page, `error-${tag}`, '[data-testid="search-error"]', 24);
    await shot(page, `error-${tag}`);

    state.floor = 'before';
    await knownScroll(page);
    await searchParty(page, 6);
    await page.getByTestId('slot-t_1+t_2-19:00').waitFor();
    await settledScroll(page);
    const opened = Date.now();
    await page.getByTestId('slot-t_2-19:00').click();
    const inert = await page.locator('[data-testid="booking-form"]').count();
    if (inert !== 0) fail(`${tag} unavailable cell opened the form`);
    await page.getByTestId('slot-t_1+t_2-19:00').click();
    await page.getByTestId('booking-form').waitFor();
    const selectMs = Date.now() - opened;
    await settledScroll(page);
    const summary = await page.getByTestId('booking-summary').innerText();
    if (!summary.includes('Table 1') || !summary.includes('Table 2') || !summary.includes('7:00 PM') || !summary.includes('(19:00)')) {
      fail(`${tag} summary ${summary.replace(/\s+/g, ' ')}`);
    }
    const party = await page.getByTestId('booking-party-size').inputValue();
    if (party !== '6') fail(`${tag} party ${party}`);
    await assertRevealed(page, `selected-${tag}`, '[data-testid="booking-form"]', 24);
    await measureSelected(page, tag);
    proof.layout.push({ label: tag, selectMs });
    await shot(page, `selected-${tag}`);
    pass(`selected-${tag}`, 'pair hold names both tables at 19:00');

    await knownScroll(page);
    await page.getByTestId('booking-submit').click();
    await page.getByTestId('booking-uncertain').waitFor();
    await settledScroll(page);
    const uncertainOnly = await page.evaluate(() => ({
      uncertain: (document.querySelector('[data-testid="booking-uncertain"]')?.textContent || '').trim().length,
      error: Boolean(document.querySelector('[data-testid="booking-error"]')),
      confirmation: Boolean(document.querySelector('[data-testid="confirmation"]')),
    }));
    if (uncertainOnly.uncertain < 1 || uncertainOnly.error || uncertainOnly.confirmation) {
      fail(`${tag} uncertain distinction ${JSON.stringify(uncertainOnly)}`);
    }
    await assertRevealed(page, `uncertain-${tag}`, '[data-testid="booking-uncertain"]', 16);
    await shot(page, `uncertain-${tag}`);

    await page.getByTestId('booking-submit').click();
    await page.getByTestId('confirmation-reference').waitFor();
    await settledScroll(page);
    const reference = (await page.getByTestId('confirmation-reference').innerText()).trim();
    const tables = await page.getByTestId('confirmation-tables').innerText();
    const details = await page.getByTestId('confirmation-details').innerText();
    if (reference !== REFERENCE) fail(`${tag} reference ${reference}`);
    if (!tables.includes('Table 1') || !tables.includes('Table 2') || tables.trim() === 'Table 3') fail(`${tag} confirmation tables ${tables}`);
    if (!details.includes('7:00 PM') || !details.includes('(19:00)') || details.includes('(20:00)')) fail(`${tag} confirmation time`);
    await assertRevealed(page, `confirmed-${tag}`, '[data-testid="confirmation"]', 24);
    await shot(page, `confirmed-${tag}`);
    const recovery = state.posts.slice(0, 2).map((entry, index) => redactPost(entry, index, state.posts));
    proof.posts.push({ label: tag, posts: recovery });
    if (state.posts.length !== 2) fail(`${tag} post count ${state.posts.length}`);
    if (!recovery.every((entry) => entry.sameKeyAsFirst && entry.sameBodyAsFirst && entry.hadAuthorization)) {
      fail(`${tag} retry identity`);
    }
    if (JSON.stringify(recovery[0].table_ids) !== JSON.stringify(['t_1', 't_2']) || recovery[0].table_id !== null || recovery[0].party_size !== 6) {
      fail(`${tag} pair body`);
    }
    if (recovery[0].outcome !== 'drop' || recovery[1].outcome !== 'receipt') fail(`${tag} outcomes`);
    else pass(`retry-${tag}`, 'same pair body and key, original receipt');

    await page.getByRole('link', { name: 'Look up' }).click();
    await page.getByTestId('lookup-reference-input').fill(REFERENCE);
    await page.getByTestId('lookup-submit').click();
    await page.getByTestId('reservation-detail').waitFor();
    await settledScroll(page);
    const beforeTables = (await page.getByTestId('reservation-tables').innerText()).trim();
    const beforeStatus = (await page.getByTestId('reservation-status').innerText()).trim();
    const beforeDetail = await page.getByTestId('reservation-detail').innerText();
    if (beforeStatus !== 'confirmed' || beforeTables !== 'Table 1 and Table 2') fail(`${tag} lookup before ${beforeStatus} ${beforeTables}`);
    if (!beforeDetail.includes('7:00 PM') || !beforeDetail.includes('(19:00)')) fail(`${tag} lookup before time`);
    await assertRevealed(page, `lookup-before-${tag}`, '[data-testid="reservation-detail"]', 24);
    await assertChrome(page, `lookup-before-${tag}`);
    await shot(page, `lookup-before-${tag}`);
    pass(`lookup-before-${tag}`, 'pair still named');

    state.lookupKind = 'after';
    await page.getByTestId('lookup-submit').click();
    await page.waitForFunction(
      () => (document.querySelector('[data-testid="reservation-tables"]')?.textContent || '').trim() === 'Table 3',
    );
    await settledScroll(page);
    const afterTables = (await page.getByTestId('reservation-tables').innerText()).trim();
    const afterDetail = await page.getByTestId('reservation-detail').innerText();
    const afterStatus = (await page.getByTestId('reservation-status').innerText()).trim();
    if (afterStatus !== 'confirmed' || afterTables !== 'Table 3') fail(`${tag} lookup after ${afterStatus} ${afterTables}`);
    if (afterDetail.includes('Table 1') || afterDetail.includes('Table 2')) fail(`${tag} lookup after still names the pair`);
    if (!afterDetail.includes('7:00 PM') || !afterDetail.includes('(19:00)')) fail(`${tag} lookup after changed the time`);
    if (state.posts.length !== 2) fail(`${tag} lookup posted a booking`);
    await assertRevealed(page, `lookup-after-${tag}`, '[data-testid="reservation-detail"]', 24);
    await shot(page, `lookup-after-${tag}`);
    pass(`lookup-after-${tag}`, 'Table 3, same time, no new booking');

    state.lookupKind = 'clock';
    await page.getByTestId('lookup-submit').click();
    await page.waitForFunction(() => (document.querySelector('[data-testid="reservation-detail"]')?.textContent || '').includes('(20:00)'));
    const clockText = await page.getByTestId('reservation-detail').innerText();
    const clockTables = (await page.getByTestId('reservation-tables').innerText()).trim();
    if (clockTables !== 'Table 1 and Table 2') fail(`${tag} clock tables ${clockTables}`);
    if (!clockText.includes('Thursday 24 June 2027') || !clockText.includes('8:00 PM') || !clockText.includes('(20:00)')) {
      fail(`${tag} clock text`);
    }
    if (clockText.includes('(19:00)') || clockText.includes('21:00')) fail(`${tag} clock invented a neighbouring time`);
    await shot(page, `lookup-clock-${tag}`);

    state.lookupKind = 'cancelled';
    await page.getByTestId('lookup-submit').click();
    await page.waitForFunction(
      () => (document.querySelector('[data-testid="reservation-status"]')?.textContent || '').trim() === 'cancelled',
    );
    const cancelledText = await page.getByTestId('reservation-detail').innerText();
    if (await page.getByTestId('reservation-cancel-button').count()) fail(`${tag} cancel button remained`);
    if (!cancelledText.includes('(20:00)') || cancelledText.includes('(19:00)')) fail(`${tag} cancelled clock`);
    await shot(page, `lookup-cancelled-${tag}`);

    state.lookupKind = 'exception';
    await page.getByTestId('lookup-submit').click();
    await page.waitForFunction(() => (document.querySelector('[data-testid="reservation-detail"]')?.textContent || '').includes('(18:30)'));
    const exceptionText = await page.getByTestId('reservation-detail').innerText();
    const exceptionStatus = (await page.getByTestId('reservation-status').innerText()).trim();
    if (exceptionStatus !== 'confirmed') fail(`${tag} exception status ${exceptionStatus}`);
    if (!exceptionText.includes('6:30 PM') || exceptionText.includes('(20:00)') || exceptionText.includes('(19:00)')) {
      fail(`${tag} exception clock`);
    }
    if ((await page.getByTestId('reservation-tables').innerText()).trim() !== 'Table 1 and Table 2') fail(`${tag} exception tables`);
    await shot(page, `lookup-exception-${tag}`);
    pass(`clock-${tag}`, 'server clocks shown for amended, cancelled and exception records');

    state.floor = 'after';
    await page.getByRole('link', { name: 'Search' }).click();
    await page.getByTestId('restaurant-select').waitFor();
    await searchParty(page, 6);
    await page.getByTestId('slot-t_3-20:30').waitFor();
    await settledScroll(page);
    const repaired = await page.evaluate(() => {
      const flag = (id) => document.querySelector(`[data-testid="${id}"]`)?.getAttribute('data-available');
      return {
        t2: flag('slot-t_2-19:00'),
        t3: flag('slot-t_3-19:00'),
        pair: flag('slot-t_1+t_2-19:00'),
        other: flag('slot-t_2+t_3-19:00'),
        laterSingle: flag('slot-t_3-20:30'),
        laterPair: flag('slot-t_1+t_2-20:30'),
        laterOther: flag('slot-t_2+t_3-20:30'),
        laterSmall: flag('slot-t_1-20:30'),
        laterClosedSingle: flag('slot-t_2-20:30'),
        seats: document.querySelector('[data-testid="plan-t_3"] .plan-seats')?.textContent?.trim() ?? '',
      };
    });
    const repairedOk =
      repaired.t2 === 'false' &&
      repaired.t3 === 'false' &&
      repaired.pair === 'false' &&
      repaired.other === 'false' &&
      repaired.laterSingle === 'true' &&
      repaired.laterPair === 'true' &&
      repaired.laterOther === 'true' &&
      repaired.laterSmall === 'false' &&
      repaired.laterClosedSingle === 'false' &&
      repaired.seats === '6 seats';
    const repairedRead = state.availabilityReads[state.availabilityReads.length - 1];
    if (!repairedOk || repairedRead?.party !== '6' || repairedRead?.floor !== 'after') {
      fail(`${tag} repaired grid ${JSON.stringify(repaired)} party=${repairedRead?.party ?? ''}`);
    } else pass(`repaired-${tag}`, 'closure blocks 19:00 and 20:30 returns the later options');
    await page.getByTestId('slot-t_1+t_2-19:00').click();
    if (await page.locator('[data-testid="booking-form"]').count()) fail(`${tag} closed pair opened the form`);
    await page.getByTestId('slot-t_3-20:30').click();
    await page.getByTestId('booking-form').waitFor();
    const laterSummary = await page.getByTestId('booking-summary').innerText();
    if (!laterSummary.includes('Table 3') || laterSummary.includes('Table 1') || !laterSummary.includes('(20:30)')) {
      fail(`${tag} later summary ${laterSummary.replace(/\s+/g, ' ')}`);
    }
    await settledScroll(page);
    await shot(page, `repaired-${tag}`);

    state.nextPost = 'refuse';
    await page.getByTestId('booking-submit').click();
    await page.getByTestId('booking-error').waitFor();
    const refused = await page.evaluate(() => ({
      form: Boolean(document.querySelector('[data-testid="booking-form"]')),
      party: document.querySelector('[data-testid="booking-party-size"]')?.value ?? '',
      error: Boolean(document.querySelector('[data-testid="booking-error"]')),
      uncertain: Boolean(document.querySelector('[data-testid="booking-uncertain"]')),
      confirmation: Boolean(document.querySelector('[data-testid="confirmation"]')),
    }));
    if (!refused.form || refused.party !== '6' || !refused.error || refused.uncertain || refused.confirmation) {
      fail(`${tag} refused form ${JSON.stringify(refused)}`);
    } else pass(`refused-${tag}`, '409 kept the form');
    await shot(page, `refused-${tag}`);

    if (state.publicBearer !== 0) fail(`${tag} public read sent a bearer`);
    if (state.lookupUnauthenticated !== 0) fail(`${tag} lookup omitted the bearer`);
    if (state.forbidden !== 0) fail(`${tag} unexpected operator or explain request`);
    if (state.availabilityReads.some((read) => read.explain != null)) fail(`${tag} explain was requested`);
    if (state.lookupReads !== 5) fail(`${tag} lookup reads ${state.lookupReads}`);
  } finally {
    release();
    await closeContext(context, page, videoName);
  }
}

async function runKeyboard(browser) {
  const state = readyState();
  const { context, page } = await openContext(browser, {
    width: 1280,
    height: 900,
    theme: 'light',
    videoName: 'fixture-transport-keyboard-1280.webm',
  });
  try {
    await install(page, state);
    await page.goto(`${origin}/`, { waitUntil: 'domcontentloaded' });
    await page.getByTestId('restaurant-select').waitFor();
    await searchParty(page, 6);
    await page.getByTestId('slot-t_1+t_2-19:00').waitFor();
    await settledScroll(page);
    await knownScroll(page);
    await page.evaluate(() => {
      const node = document.querySelector('[data-testid="slot-t_1+t_2-19:00"]');
      if (node instanceof HTMLElement) node.focus({ preventScroll: true });
    });
    await page.keyboard.press('Enter');
    await page.getByTestId('booking-form').waitFor();
    const summary = await page.getByTestId('booking-summary').innerText();
    if (!summary.includes('Table 1') || !summary.includes('Table 2')) fail(`keyboard summary ${summary}`);
    else pass('keyboard-1280', 'Enter on the pair opens the form');
    await shot(page, 'keyboard-1280-light');
  } finally {
    await closeContext(context, page, 'fixture-transport-keyboard-1280.webm');
  }
}

async function runReduced(browser) {
  const state = readyState();
  const { context, page } = await openContext(browser, {
    width: 1280,
    height: 900,
    theme: 'dark',
    reduced: true,
    videoName: 'fixture-transport-reduced-motion-1280.webm',
  });
  try {
    await install(page, state);
    await page.goto(`${origin}/`, { waitUntil: 'domcontentloaded' });
    await page.getByTestId('restaurant-select').waitFor();
    await searchParty(page, 6);
    await page.getByTestId('slot-t_1+t_2-19:00').waitFor();
    await settledScroll(page);
    await knownScroll(page);
    const started = Date.now();
    await page.getByTestId('slot-t_1+t_2-19:00').click();
    await page.waitForFunction(
      () => document.querySelector('[data-testid="slot-t_1+t_2-19:00"]')?.getAttribute('data-selected') === 'true',
    );
    const elapsed = Date.now() - started;
    await page.getByTestId('booking-form').waitFor();
    const motion = await page.evaluate(() => ({
      reduced: window.matchMedia('(prefers-reduced-motion: reduce)').matches,
      running: document.getAnimations().filter((animation) => animation.playState === 'running').length,
    }));
    proof.layout.push({ label: 'reduced-1280-dark', selectMs: elapsed, ...motion });
    if (!motion.reduced || elapsed > 400) fail(`reduced selection ${elapsed}ms reduced=${motion.reduced}`);
    else pass('reduced-1280', `selected in ${elapsed}ms`);
    await measureSelected(page, 'reduced-1280-dark');
    await shot(page, 'reduced-1280-dark');
  } finally {
    await closeContext(context, page, 'fixture-transport-reduced-motion-1280.webm');
  }
}

function readyState() {
  return {
    holdCatalog: false,
    gate: Promise.resolve(),
    floor: 'before',
    nextPost: 'drop',
    lookupKind: 'before',
    posts: [],
    detailReads: 0,
    lookupReads: 0,
    availabilityReads: [],
    publicBearer: 0,
    lookupUnauthenticated: 0,
    forbidden: 0,
  };
}

function unexpectedConsole(text) {
  return !/favicon|Failed to load resource|net::ERR_|ERR_FAILED|ERR_ABORTED|409|422/.test(text);
}

async function tighten(dir) {
  await chmod(dir, 0o700).catch(() => {});
  const names = await readdir(dir, { withFileTypes: true }).catch(() => []);
  for (const name of names) {
    const path = join(dir, name.name);
    if (name.isDirectory()) await tighten(path);
    else await chmod(path, 0o600).catch(() => {});
  }
}

async function main() {
  assertFixtureClocks();
  await mkdir(frameDir, { recursive: true, mode: 0o700 });
  await mkdir(videoDir, { recursive: true, mode: 0o700 });
  await mkdir(rawDir, { recursive: true, mode: 0o700 });
  const child = spawn(
    process.execPath,
    [join(webDir, 'node_modules/vite/bin/vite.js'), 'preview', '--host', '127.0.0.1', '--port', String(port), '--strictPort'],
    { cwd: webDir, stdio: ['ignore', 'pipe', 'pipe'] },
  );
  const browser = await chromium.launch({ executablePath: chrome, headless: true });
  try {
    await waitForPreview(child);
    for (const theme of ['light', 'dark']) {
      for (const size of [
        { width: 1280, height: 900 },
        { width: 375, height: 812 },
      ]) {
        const videoName = theme === 'light' ? `fixture-transport-booking-retry-${size.width}.webm` : '';
        await runWalk(browser, { ...size, theme, videoName });
      }
    }
    await runKeyboard(browser);
    await runReduced(browser);
    const noisy = proof.consoleErrors.filter(unexpectedConsole);
    if (noisy.length) fail(`unexpected console errors ${noisy.length}`);
    if (proof.pageErrors.length) fail(`page errors ${proof.pageErrors.join(' | ')}`);
    if (proof.offOrigin.length) fail(`off-origin requests ${proof.offOrigin.join(' | ')}`);
    const named = [
      'selected',
      'uncertain',
      'confirmed',
      'lookup-before',
      'lookup-after',
      'lookup-clock',
      'lookup-cancelled',
      'lookup-exception',
    ];
    for (const theme of ['light', 'dark']) {
      for (const width of [1280, 375]) {
        for (const state of named) {
          const file = join(frameDir, `fixture-transport-${state}-${width}-${theme}.png`);
          if (!proof.shots.includes(file)) fail(`missing frame ${state}-${width}-${theme}`);
        }
      }
    }
  } finally {
    await browser.close();
    child.kill('SIGTERM');
    await rm(rawDir, { recursive: true, force: true }).catch(() => {});
    const leftovers = await readdir(videoDir).catch(() => []);
    for (const name of leftovers) {
      if (name.startsWith('page@')) await rm(join(videoDir, name), { force: true });
    }
    const serialized = JSON.stringify(proof);
    if (serialized.includes(TOKEN)) fail('report contained the fixture session token');
    proof.tokenPresent = false;
    await writeFile(join(evidence, 'fixture-transport-report.json'), `${JSON.stringify(proof, null, 2)}\n`, { mode: 0o600 });
    await tighten(evidence);
  }
  if (proof.failures.length) {
    console.error(`FIXTURE TRANSPORT failures: ${proof.failures.length}`);
    process.exitCode = 1;
    return;
  }
  console.log(`FIXTURE TRANSPORT ok shots=${proof.shots.length} videos=${proof.videos.length}`);
}

const invoked = process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href;
if (invoked) {
  main().catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
}
