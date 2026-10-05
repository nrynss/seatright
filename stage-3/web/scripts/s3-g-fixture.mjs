/**
 * FIXTURE TRANSPORT. Playwright answers the built stage-3 page with the stage-3
 * policy fixture. This is not the Go service, not a published policy API, and
 * not a stage-1 to stage-3 image migration.
 *
 * Frames are named fixture-transport-* and the report label is FIXTURE TRANSPORT.
 */
import { spawn } from 'node:child_process';
import { mkdir, readdir, rm, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { chromium } from 'playwright';

const webDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const port = Number(process.env.PORT ?? 4183);
const origin = `http://127.0.0.1:${port}`;
const evidence =
  process.env.EVIDENCE ?? '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S3-G';
const frameDir = join(evidence, 'frames');
const videoDir = join(evidence, 'videos');
const rawDir = join(evidence, 'video-raw');
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const TOKEN = 'fixture-session';
const CLOCKS = ['18:00', '19:00', '20:00', '21:00', '22:00'];

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
    { id: 't_3', label: '3', capacity: 4 },
  ],
  combinable: [
    ['t_1', 't_2'],
    ['t_2', 't_3'],
  ],
};

const acceptedTerms = {
  policy_version: 1,
  slot_minutes: 60,
  reservation_duration_minutes: 60,
  cancellation_cutoff_minutes: 60,
  opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
  capacities: { t_1: 4, t_2: 6, t_3: 4 },
};

const laterTerms = {
  ...acceptedTerms,
  slot_minutes: 45,
  reservation_duration_minutes: 30,
  capacities: { t_1: 3, t_2: 3, t_3: 3 },
};

function policyFloor(party) {
  const singles =
    party === 4
      ? [
          { table_ids: ['t_1'], capacity: 4 },
          { table_ids: ['t_2'], capacity: 6 },
          { table_ids: ['t_3'], capacity: 4 },
        ]
      : [];
  const available = party === 4 ? ['t_1', 't_2', 't_3'] : [];
  return {
    restaurant_id: 'r_anker',
    date: '2027-06-17',
    timezone: 'Europe/Berlin',
    slots: CLOCKS.map((time) => ({
      starts_at_local: `2027-06-17T${time}`,
      starts_at: `2027-06-17T${time}:00+02:00`,
      available_table_ids: [...available],
      available_options: [
        ...singles.map((item) => ({ table_ids: [...item.table_ids], capacity: item.capacity })),
        { table_ids: ['t_1', 't_2'], capacity: 10 },
        { table_ids: ['t_2', 't_3'], capacity: 10 },
      ],
    })),
  };
}

function policyReceipt(revision, terms) {
  return {
    reservation_id: 'res_pair',
    reference: 'POLICY01',
    restaurant_id: 'r_anker',
    table_ids: ['t_1', 't_2'],
    party_size: 8,
    status: 'confirmed',
    starts_at_local: '2027-06-17T18:00',
    starts_at: '2027-06-17T18:00:00+02:00',
    ends_at: '2027-06-17T19:00:00+02:00',
    created_at: '2026-10-05T05:00:00+00:00',
    revision,
    accepted_terms: terms,
  };
}

const proof = {
  label: 'FIXTURE TRANSPORT',
  note: 'Route interception on the built preview. Not real Go policies and not a stage upgrade.',
  origin,
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
    if (path === '/restaurants' && request.method() === 'GET') {
      if (state.holdCatalog) await state.gate;
      return json(route, 200, {
        restaurants: [{ id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' }],
      });
    }
    if (path === '/restaurants/r_anker' && request.method() === 'GET') {
      state.detailReads += 1;
      return json(route, 200, detail);
    }
    if (path === '/availability' && request.method() === 'GET') {
      const party = Number(url.searchParams.get('party_size'));
      state.availabilityReads.push({ party, explain: url.searchParams.get('explain') });
      if (state.floor === 'empty') {
        return json(route, 200, {
          restaurant_id: 'r_anker',
          date: '2027-06-17',
          timezone: 'Europe/Berlin',
          slots: [],
        });
      }
      if (state.floor === 'error') {
        return json(route, 422, {
          error: { code: 'validation_failed', message: 'The floor could not be read.' },
        });
      }
      return json(route, 200, policyFloor(party === 4 ? 4 : 8));
    }
    if (path === '/reservations' && request.method() === 'POST') {
      const body = JSON.parse(request.postData() || '{}');
      const key = request.headers()['idempotency-key'] || '';
      const hadAuthorization = Boolean(request.headers().authorization);
      const outcome = state.posts.length === 0 ? 'dropped' : 'confirmed';
      state.posts.push({ body, key, hadAuthorization, outcome });
      if (outcome === 'dropped') {
        await route.abort('failed');
        return;
      }
      return json(route, 201, policyReceipt(1, acceptedTerms));
    }
    if (path === '/reservations/POLICY01' && request.method() === 'GET') {
      state.lookupReads += 1;
      return json(route, 200, policyReceipt(2, laterTerms));
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

async function searchParty(page, state, party) {
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
    const badgeText = badge ? (badge.textContent || '').replace(/\s+/g, ' ').trim() : '';
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
      badgeText,
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
  const tableOne = (paint.seatLines || []).find((line) => line.text.startsWith('2 '));
  if (!tableOne || tableOne.font < 13) fail(`${label} table 1 seat ink ${JSON.stringify(paint.seatLines)}`);
  if (!(paint.seatLines || []).some((line) => line.text === '2 seats')) fail(`${label} table 1 was resized`);
  if (!String(paint.aria).includes('10 seats')) fail(`${label} badge aria ${paint.aria}`);
  if (paint.pageOverflow) fail(`${label} page overflow ${paint.scrollWidth}/${paint.clientWidth}`);
  for (const hit of paint.hits || []) {
    if (/pair-link/.test(hit.hit)) fail(`${label} connector covers ${hit.text}`);
  }

  const grid = await page.evaluate(() => {
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
    const clocks = [...root.querySelectorAll('[data-testid^="slot-t_1-"]')].map((node) =>
      (node.getAttribute('data-testid') || '').replace('slot-t_1-', ''),
    );
    const pairHead = [...root.querySelectorAll('.rowhead')].map((node) => (node.textContent || '').replace(/\s+/g, ' ').trim());
    return {
      present: true,
      cells: root.querySelectorAll('.cell').length,
      outside,
      clocks,
      pairHead,
      selected: document.querySelector('[data-testid="slot-t_1+t_2-18:00"]')?.getAttribute('data-selected'),
      available: document.querySelector('[data-testid="slot-t_1+t_2-18:00"]')?.getAttribute('data-available'),
    };
  });
  proof.bounds.push({ label, ...grid });
  if (!grid.present) fail(`${label} grid missing`);
  if ((grid.outside || []).length) fail(`${label} state-word ink outside ${grid.outside.join(',')}`);
  if ((grid.clocks || []).join(',') !== CLOCKS.join(',')) fail(`${label} clocks ${(grid.clocks || []).join(',')}`);
  if (!(grid.pairHead || []).some((text) => text.includes('10 seats together'))) fail(`${label} pair seat text`);
  if ((grid.pairHead || []).some((text) => text.includes('6 seats together'))) fail(`${label} stale pair sum still shown`);
  if (grid.selected !== 'true' || grid.available !== 'true') fail(`${label} selected pair flags ${grid.selected}/${grid.available}`);
  else pass(`grid-${label}`, `${grid.cells} cells, ink inside, 10 seats`);

  const painted = await page.evaluate((id) => {
    const cell = document.querySelector(`[data-testid="${id}"]`);
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
      fontSize: style.fontSize,
    };
  }, 'slot-t_1+t_2-18:00');
  const ratio = painted ? contrastRatio(painted.color, painted.background) : null;
  proof.contrast.push({
    label,
    word: painted?.word ?? '',
    foreground: painted ? hexOf(painted.color) : '',
    background: painted ? hexOf(painted.background) : '',
    ratio: ratio == null ? null : Math.round(ratio * 1000) / 1000,
  });
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
    floor: 'policy',
    posts: [],
    detailReads: 0,
    lookupReads: 0,
    availabilityReads: [],
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
    await searchParty(page, state, 8);
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
    const errorText = await page.getByTestId('search-error').evaluate((node) => node.textContent || '');
    if (!errorText.includes('could not be read')) fail(`${tag} error copy`);
    await shot(page, `error-${tag}`);

    state.floor = 'policy';
    await knownScroll(page);
    await page.getByTestId('party-size-input').fill('4');
    await page.getByTestId('search-button').click();
    await page.getByTestId('slot-t_1-18:00').waitFor();
    await settledScroll(page);
    const party4 = await page.evaluate(() => ({
      t1: document.querySelector('[data-testid="slot-t_1-18:00"]')?.getAttribute('data-available'),
      t2: document.querySelector('[data-testid="slot-t_2-18:00"]')?.getAttribute('data-available'),
      t3: document.querySelector('[data-testid="slot-t_3-18:00"]')?.getAttribute('data-available'),
      seats: document.querySelector('[data-testid="plan-t_1"] .plan-seats')?.textContent?.trim() ?? '',
      half: Boolean(document.querySelector('[data-testid="slot-t_1-18:30"]')),
    }));
    if (party4.t1 !== 'true' || party4.t2 !== 'true' || party4.t3 !== 'true' || party4.seats !== '2 seats' || party4.half) {
      fail(`${tag} party 4 membership ${JSON.stringify(party4)}`);
    } else pass(`party4-${tag}`, 'singles available, table 1 still 2 seats');

    await knownScroll(page);
    await page.getByTestId('party-size-input').fill('8');
    await page.getByTestId('search-button').click();
    await page.getByTestId('slot-t_1+t_2-18:00').waitFor();
    await settledScroll(page);
    const opened = Date.now();
    await page.getByTestId('slot-t_1+t_2-18:00').click();
    await page.getByTestId('booking-form').waitFor();
    const selectMs = Date.now() - opened;
    await settledScroll(page);
    await assertRevealed(page, `selected-${tag}`, '[data-testid="booking-form"]', 24);
    await measureSelected(page, tag);
    proof.layout.push({ label: tag, selectMs });
    await shot(page, `selected-${tag}`);

    const postsBefore = state.posts.length;
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
    const reference = await page.getByTestId('confirmation-reference').evaluate((node) => (node.textContent || '').trim());
    const confirmation = await page.getByTestId('confirmation').evaluate((node) => node.textContent || '');
    if (reference !== 'POLICY01') fail(`${tag} reference ${reference}`);
    if (confirmation.includes('policy_version') || confirmation.includes('19:30') || confirmation.includes('19:00:00')) {
      fail(`${tag} confirmation reconstructed server fields`);
    }
    await assertRevealed(page, `confirmed-${tag}`, '[data-testid="confirmation"]', 24);
    await shot(page, `confirmed-${tag}`);
    if (state.posts.length !== postsBefore + 2) fail(`${tag} post count ${state.posts.length}`);
    const redacted = state.posts.map((entry, index) => redactPost(entry, index, state.posts));
    proof.posts.push({ label: tag, posts: redacted });
    if (!redacted.every((entry) => entry.sameKeyAsFirst && entry.sameBodyAsFirst && entry.hadAuthorization)) {
      fail(`${tag} retry identity ${JSON.stringify(redacted)}`);
    }
    if (redacted[0].outcome !== 'dropped' || redacted[1].outcome !== 'confirmed') fail(`${tag} outcomes`);
    if (JSON.stringify(redacted[0].table_ids) !== JSON.stringify(['t_1', 't_2']) || redacted[0].party_size !== 8) {
      fail(`${tag} pair body ${JSON.stringify(redacted[0])}`);
    } else pass(`retry-${tag}`, 'same body and key after the dropped response');

    const postsAtLookup = state.posts.length;
    await page.getByRole('link', { name: 'Look up' }).click();
    await page.getByTestId('lookup-reference-input').fill('POLICY01');
    await page.getByTestId('lookup-submit').click();
    await page.getByTestId('reservation-detail').waitFor();
    await settledScroll(page);
    const status = await page.getByTestId('reservation-status').evaluate((node) => (node.textContent || '').trim());
    const tables = await page.getByTestId('reservation-tables').evaluate((node) => node.textContent || '');
    if (status !== 'confirmed' || !tables.includes('1') || !tables.includes('2')) fail(`${tag} lookup ${status} ${tables}`);
    if (state.lookupReads !== 1 || state.posts.length !== postsAtLookup) fail(`${tag} lookup rewrote the pending booking`);
    await assertRevealed(page, `lookup-${tag}`, '[data-testid="reservation-detail"]', 24);
    await assertChrome(page, `lookup-${tag}`);
    await shot(page, `lookup-${tag}`);
    pass(`lookup-${tag}`, 'revision 2 terms differ and no new booking was posted');
    if (state.detailReads < 1) fail(`${tag} detail was not read`);
    if (state.availabilityReads.some((read) => read.explain != null)) fail(`${tag} explain was requested`);
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
    await searchParty(page, state, 8);
    state.floor = 'policy';
    await page.getByTestId('slot-t_1+t_2-18:00').waitFor();
    await settledScroll(page);
    await knownScroll(page);
    await page.evaluate(() => {
      const node = document.querySelector('[data-testid="slot-t_1+t_2-18:00"]');
      if (node instanceof HTMLElement) node.focus({ focusVisible: true });
    });
    await page.keyboard.press('Enter');
    await page.getByTestId('booking-form').waitFor();
    const focused = await page.evaluate(() => document.activeElement?.getAttribute('data-testid') || '');
    if (focused !== 'slot-t_1+t_2-18:00' && !(await page.getByTestId('booking-form').isVisible())) {
      fail(`keyboard focus landed on ${focused}`);
    }
    pass('keyboard-1280', 'Enter on the pair opens the form');
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
    await searchParty(page, state, 8);
    await page.getByTestId('slot-t_1+t_2-18:00').waitFor();
    await settledScroll(page);
    await knownScroll(page);
    const started = Date.now();
    await page.getByTestId('slot-t_1+t_2-18:00').click();
    await page.waitForFunction(
      () => document.querySelector('[data-testid="slot-t_1+t_2-18:00"]')?.getAttribute('data-selected') === 'true',
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
  } finally {
    await closeContext(context, page, 'fixture-transport-reduced-motion-1280.webm');
  }
}

function readyState() {
  return {
    holdCatalog: false,
    gate: Promise.resolve(),
    floor: 'policy',
    posts: [],
    detailReads: 0,
    lookupReads: 0,
    availabilityReads: [],
  };
}

function unexpectedConsole(text) {
  return !/favicon|Failed to load resource|net::ERR_|ERR_FAILED|ERR_ABORTED|409|422/.test(text);
}

async function main() {
  await mkdir(frameDir, { recursive: true });
  await mkdir(videoDir, { recursive: true });
  await mkdir(rawDir, { recursive: true });
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
    const named = ['loading', 'empty', 'error', 'selected', 'uncertain', 'confirmed', 'lookup'];
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
    const serialized = JSON.stringify(proof, null, 2);
    if (serialized.includes(TOKEN)) fail('report contained the fixture session token');
    proof.tokenPresent = false;
    await writeFile(join(evidence, 'fixture-transport-report.json'), `${JSON.stringify(proof, null, 2)}\n`);
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
