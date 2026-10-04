/**
 * TEST ONLY. Opens the built stage-2 page and answers the restaurant API with
 * the S2-G pair fixture. This is not the Go service and not a live pair booking.
 *
 * Screenshots: results, selected, confirmed, uncertain, refused, lookup
 * at 375 and 1280, light and dark. Videos: selection and reduced-motion selection.
 *
 * Also records each Free/Taken/Held word's DOM Range against the cell's inner
 * box and border box, after transitions settle. BOUNDS_STRICT=0 records
 * overflow without failing. VIDEOS=0 skips the screen recordings.
 */
import { spawn } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const webDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const port = Number(process.env.PORT ?? 4174);
const origin = `http://127.0.0.1:${port}`;
const evidence =
  process.env.EVIDENCE ??
  '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S2-G/fixture-preview';
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const strictBounds = process.env.BOUNDS_STRICT !== '0';
const recordVideos = process.env.VIDEOS !== '0';

const times = ['18:00', '18:30', '19:00', '19:30', '20:00', '20:30', '21:00', '21:30'];

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

function freeFloor() {
  return {
    restaurant_id: 'r_anker',
    date: '2027-06-17',
    timezone: 'Europe/Berlin',
    slots: times.map((time) => ({
      starts_at_local: `2027-06-17T${time}`,
      starts_at: `2027-06-17T${time}:00+02:00`,
      available_table_ids: [],
      available_options: [
        { table_ids: ['t_2', 't_1'], capacity: 6 },
        { table_ids: ['t_2', 't_3'], capacity: 8 },
      ],
    })),
  };
}

function takenFloor() {
  const floor = freeFloor();
  for (const slot of floor.slots) {
    const time = slot.starts_at_local.slice(11);
    if (time === '18:00' || time === '18:30' || time === '19:00') slot.available_options = [];
  }
  return floor;
}

function pairReceipt(status = 'confirmed') {
  return {
    reservation_id: 'res_pair',
    reference: 'PAIR01',
    restaurant_id: 'r_anker',
    table_ids: ['t_1', 't_2'],
    party_size: 6,
    status,
    starts_at_local: '2027-06-17T18:00',
    starts_at: '2027-06-17T18:00:00+02:00',
    ends_at: '2027-06-17T19:30:00+02:00',
    created_at: '2026-10-04T22:00:00+00:00',
  };
}

function json(route, status, body) {
  return route.fulfill({
    status,
    contentType: 'application/json; charset=utf-8',
    body: JSON.stringify(body),
  });
}

const proof = {
  label: 'TEST ONLY fixture preview. Not the Go image and not a live pair backend.',
  origin,
  shots: [],
  posts: [],
  immediate: [],
  scroll: [],
  pageErrors: [],
  consoleErrors: [],
  offOrigin: [],
  fontRequests: [],
  bounds: [],
};

function log(message) {
  console.log(message);
}

async function waitForPreview(child) {
  const started = Date.now();
  let stderr = '';
  child.stderr?.on('data', (chunk) => {
    stderr += String(chunk);
  });
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

async function settle(page) {
  await page.evaluate(async () => {
    const deadline = performance.now() + 5000;
    while (performance.now() < deadline) {
      const running = document.getAnimations().filter((animation) => (
        animation.playState === 'running' || animation.playState === 'pending'
      ));
      if (running.length === 0) break;
      await new Promise((resolve) => setTimeout(resolve, 40));
    }
    await new Promise((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(resolve));
    });
  });
}

function boundsFail(summary) {
  if (!summary?.present) return false;
  return (
    summary.outsideInner > 0 ||
    summary.outsideBorder > 0 ||
    summary.clipped > 0 ||
    summary.collisions > 0 ||
    summary.whenOverlap > 0 ||
    summary.rowheadOutside > 0 ||
    summary.colheadOutside > 0 ||
    summary.gridScroll === true
  );
}

async function measureGrid(page, name) {
  return page.evaluate((label) => {
    const round = (value) => Math.round(value * 100) / 100;
    const finite = (value) => (Number.isFinite(value) ? round(value) : null);
    const rectOf = (rect) => ({
      x: round(rect.left),
      y: round(rect.top),
      w: round(rect.width),
      h: round(rect.height),
    });

    function textRect(element) {
      const range = document.createRange();
      range.selectNodeContents(element);
      const rects = [...range.getClientRects()].filter((rect) => rect.width > 0.5 && rect.height > 0.5);
      if (rects.length === 0) return null;
      const left = Math.min(...rects.map((rect) => rect.left));
      const right = Math.max(...rects.map((rect) => rect.right));
      const top = Math.min(...rects.map((rect) => rect.top));
      const bottom = Math.max(...rects.map((rect) => rect.bottom));
      return { left, right, top, bottom, width: right - left, height: bottom - top, lines: rects.length };
    }

    function edges(element) {
      const rect = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      const borderLeft = parseFloat(style.borderLeftWidth) || 0;
      const borderRight = parseFloat(style.borderRightWidth) || 0;
      const borderTop = parseFloat(style.borderTopWidth) || 0;
      const borderBottom = parseFloat(style.borderBottomWidth) || 0;
      const padLeft = parseFloat(style.paddingLeft) || 0;
      const padRight = parseFloat(style.paddingRight) || 0;
      const padTop = parseFloat(style.paddingTop) || 0;
      const padBottom = parseFloat(style.paddingBottom) || 0;
      return {
        border: rect,
        inner: {
          left: rect.left + borderLeft + padLeft,
          right: rect.right - borderRight - padRight,
          top: rect.top + borderTop + padTop,
          bottom: rect.bottom - borderBottom - padBottom,
        },
        overflowX: style.overflowX,
      };
    }

    function inset(text, bound) {
      return {
        l: round(text.left - bound.left),
        r: round(bound.right - text.right),
        t: round(text.top - bound.top),
        b: round(bound.bottom - text.bottom),
      };
    }

    function outside(gap) {
      return gap.l < -0.5 || gap.r < -0.5 || gap.t < -0.5 || gap.b < -0.5;
    }

    function shown(element) {
      const style = getComputedStyle(element);
      if (style.display === 'none' || style.visibility === 'hidden') return false;
      const rect = element.getBoundingClientRect();
      return rect.width > 0.5 && rect.height > 0.5;
    }

    const root = document.querySelector('[data-testid="availability-grid"]');
    if (!root) {
      return {
        summary: { name: label, present: false, fail: false },
        cells: [],
        rowheads: [],
        colheads: [],
      };
    }

    const records = [...root.querySelectorAll('.cell')].map((cell) => {
      const word = cell.querySelector('.state-word');
      const text = word ? textRect(word) : null;
      const edge = edges(cell);
      const innerInset = text ? inset(text, edge.inner) : null;
      const borderInset = text ? inset(text, edge.border) : null;
      const wordStyle = word ? getComputedStyle(word) : null;
      const clipped = Boolean(
        word &&
          (edge.overflowX === 'hidden' || edge.overflowX === 'clip' || wordStyle.overflowX === 'hidden' || wordStyle.overflowX === 'clip') &&
          word.scrollWidth > word.clientWidth + 1,
      );
      const when = cell.querySelector('.when');
      const whenShown = when && getComputedStyle(when).display !== 'none';
      const whenText = whenShown ? textRect(when) : null;
      let whenGap = null;
      if (text && whenText) {
        whenGap = round(text.left - whenText.right);
      }
      return {
        id: cell.getAttribute('data-testid'),
        word: word ? word.textContent.trim() : '',
        available: cell.getAttribute('data-available'),
        selected: cell.getAttribute('data-selected'),
        cell: rectOf(edge.border),
        inner: {
          x: round(edge.inner.left),
          y: round(edge.inner.top),
          w: round(edge.inner.right - edge.inner.left),
          h: round(edge.inner.bottom - edge.inner.top),
        },
        text: text ? { x: round(text.left), y: round(text.top), w: round(text.width), h: round(text.height), lines: text.lines } : null,
        innerInset,
        borderInset,
        outsideInner: innerInset ? outside(innerInset) : true,
        outsideBorder: borderInset ? outside(borderInset) : true,
        clipped,
        whenGap,
        whenOverlap: whenGap != null && whenGap < 0.5,
      };
    });

    const rows = new Map();
    for (const record of records) {
      const key = Math.round((record.cell?.y ?? 0) / 2);
      if (!rows.has(key)) rows.set(key, []);
      rows.get(key).push(record);
    }
    const collisions = [];
    let minNeighbor = Infinity;
    for (const row of rows.values()) {
      const visible = row.filter((record) => record.text);
      visible.sort((left, right) => left.text.x - right.text.x);
      for (let index = 1; index < visible.length; index += 1) {
        const previous = visible[index - 1];
        const current = visible[index];
        const gap = round(current.text.x - (previous.text.x + previous.text.w));
        current.neighborGap = gap;
        if (gap < minNeighbor) minNeighbor = gap;
        if (gap < 0.5) collisions.push({ left: previous.id, right: current.id, gap, words: `${previous.word}/${current.word}` });
      }
    }

    function labelRecords(selector) {
      return [...root.querySelectorAll(selector)].filter(shown).map((element) => {
        const text = textRect(element);
        const edge = edges(element);
        const innerInset = text ? inset(text, edge.inner) : null;
        const borderInset = text ? inset(text, edge.border) : null;
        return {
          text: element.innerText.replace(/\s+/g, ' ').trim(),
          cell: rectOf(edge.border),
          textBox: text ? { x: round(text.left), y: round(text.top), w: round(text.width), h: round(text.height), lines: text.lines } : null,
          innerInset,
          borderInset,
          outsideInner: innerInset ? outside(innerInset) : true,
          outsideBorder: borderInset ? outside(borderInset) : true,
        };
      });
    }

    const rowheads = labelRecords('.rowhead');
    const colheads = labelRecords('.colhead');
    const wrap = root.closest('.matrix-wrap') ?? root;
    const gridScroll = wrap.scrollWidth > wrap.clientWidth + 1;
    const minInner = records.reduce((min, record) => {
      if (!record.innerInset) return min;
      return Math.min(min, record.innerInset.l, record.innerInset.r, record.innerInset.t, record.innerInset.b);
    }, Infinity);
    const minBorder = records.reduce((min, record) => {
      if (!record.borderInset) return min;
      return Math.min(min, record.borderInset.l, record.borderInset.r, record.borderInset.t, record.borderInset.b);
    }, Infinity);
    const summary = {
      name: label,
      present: true,
      cells: records.length,
      counts: {
        Free: records.filter((record) => record.word === 'Free').length,
        Taken: records.filter((record) => record.word === 'Taken').length,
        Held: records.filter((record) => record.word === 'Held').length,
      },
      outsideInner: records.filter((record) => record.outsideInner).length,
      outsideBorder: records.filter((record) => record.outsideBorder).length,
      clipped: records.filter((record) => record.clipped).length,
      collisions: collisions.length,
      whenOverlap: records.filter((record) => record.whenOverlap).length,
      minNeighborGap: finite(minNeighbor),
      minInnerClearance: finite(minInner),
      minBorderClearance: finite(minBorder),
      rowheadOutside: rowheads.filter((record) => record.outsideBorder).length,
      colheadOutside: colheads.filter((record) => record.outsideBorder).length,
      rowheadInner: rowheads.filter((record) => record.outsideInner).length,
      colheadInner: colheads.filter((record) => record.outsideInner).length,
      gridScroll,
      rowheadLines: rowheads.map((record) => ({ text: record.text, lines: record.textBox?.lines ?? 0, w: record.cell.w })),
      wordWidths: {
        Free: finite(Math.max(...records.filter((record) => record.word === 'Free' && record.text).map((record) => record.text.w), 0)) || null,
        Taken: finite(Math.max(...records.filter((record) => record.word === 'Taken' && record.text).map((record) => record.text.w), 0)) || null,
        Held: finite(Math.max(...records.filter((record) => record.word === 'Held' && record.text).map((record) => record.text.w), 0)) || null,
      },
      worst: records
        .filter((record) => record.outsideInner || record.outsideBorder || record.clipped || record.whenOverlap)
        .slice(0, 8)
        .map((record) => ({
          id: record.id,
          word: record.word,
          cell: record.cell,
          inner: record.inner,
          text: record.text,
          innerInset: record.innerInset,
          borderInset: record.borderInset,
          neighborGap: record.neighborGap ?? null,
        })),
      collisionSample: collisions.slice(0, 6),
      rowheadWorst: rowheads.filter((record) => record.outsideBorder || record.outsideInner).slice(0, 4),
      colheadWorst: colheads.filter((record) => record.outsideBorder || record.outsideInner).slice(0, 4),
    };
    summary.fail = (
      summary.outsideInner > 0 ||
      summary.outsideBorder > 0 ||
      summary.clipped > 0 ||
      summary.collisions > 0 ||
      summary.whenOverlap > 0 ||
      summary.rowheadOutside > 0 ||
      summary.colheadOutside > 0 ||
      summary.gridScroll
    );
    return { summary, cells: records, rowheads, colheads };
  }, name);
}

async function shoot(page, name) {
  await settle(page);
  const measured = await measureGrid(page, name);
  proof.bounds.push(measured);
  const summary = measured.summary;
  if (summary.present) {
    log(
      `bounds ${name} cells=${summary.cells} Free=${summary.counts.Free} Taken=${summary.counts.Taken} Held=${summary.counts.Held} ` +
        `outsideInner=${summary.outsideInner} outsideBorder=${summary.outsideBorder} clipped=${summary.clipped} ` +
        `collisions=${summary.collisions} whenOverlap=${summary.whenOverlap} gridScroll=${summary.gridScroll} ` +
        `minInner=${summary.minInnerClearance} minBorder=${summary.minBorderClearance} minGap=${summary.minNeighborGap} ` +
        `rowheadOut=${summary.rowheadOutside} colheadOut=${summary.colheadOutside} ` +
        `widths=${JSON.stringify(summary.wordWidths)} fail=${summary.fail}`,
    );
  } else {
    log(`bounds ${name} no grid`);
  }
  const file = join(evidence, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  const box = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
    theme: document.documentElement.dataset.theme ?? '',
    title: document.title,
  }));
  const overflow = box.scrollWidth > box.clientWidth + 1;
  proof.shots.push({ name, file, ...box, overflow });
  proof.scroll.push({ name, ...box, overflow });
  log(`${name} overflow=${overflow} ${box.clientWidth}x scroll ${box.scrollWidth} theme=${box.theme}`);
  if (overflow) throw new Error(`${name} scrolls sideways`);
}

async function prepare(page, theme) {
  await page.addInitScript((mode) => {
    localStorage.setItem('chaaya-theme', mode);
    localStorage.setItem(
      'tablekeeper-session',
      JSON.stringify({ token: 'fixture-session', displayName: 'Ada', userId: 'u_ada' }),
    );
  }, theme);
  const posts = [];
  let availabilityCount = 0;
  page.on('pageerror', (error) => proof.pageErrors.push(String(error)));
  page.on('console', (message) => {
    if (message.type() === 'error') proof.consoleErrors.push(message.text());
  });
  page.on('request', (request) => {
    const url = request.url();
    if (url.startsWith('data:')) return;
    let parsed;
    try {
      parsed = new URL(url);
    } catch {
      proof.offOrigin.push(url);
      return;
    }
    if (parsed.origin !== origin) proof.offOrigin.push(parsed.origin + parsed.pathname);
    if (parsed.pathname.includes('/fonts/')) proof.fontRequests.push(parsed.pathname);
  });
  await page.route(/\/(restaurants|availability|reservations)(?:[/?]|$)/, async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    if (path === '/restaurants') {
      await json(route, 200, {
        restaurants: [{ id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' }],
      });
      return;
    }
    if (path === '/restaurants/r_anker') {
      await json(route, 200, detail);
      return;
    }
    if (path === '/availability') {
      availabilityCount += 1;
      await json(route, 200, availabilityCount === 1 ? freeFloor() : takenFloor());
      return;
    }
    if (path === '/reservations' && request.method() === 'POST') {
      const body = JSON.parse(request.postData() || '{}');
      posts.push({
        keys: Object.keys(body).sort(),
        table_ids: body.table_ids ?? null,
        table_id: Object.prototype.hasOwnProperty.call(body, 'table_id'),
        party_size: body.party_size,
        starts_at_local: body.starts_at_local,
        keyLength: (request.headers()['idempotency-key'] ?? '').length,
      });
      const mode = page.__postMode ?? 'confirm';
      if (mode === 'uncertain') {
        await route.abort('failed');
        return;
      }
      if (mode === 'refused') {
        await json(route, 409, { error: { code: 'table_unavailable', message: 'Those tables were just taken.' } });
        return;
      }
      await json(route, 201, pairReceipt());
      return;
    }
    if (path === '/reservations/PAIR01' && request.method() === 'GET') {
      await json(route, 200, pairReceipt());
      return;
    }
    if (path === '/reservations/PAIR01/cancel') {
      await json(route, 200, pairReceipt('cancelled'));
      return;
    }
    await json(route, 404, { error: { code: 'not_found', message: 'missing' } });
  });
  return posts;
}

async function search(page) {
  await page.goto(`${origin}/`, { waitUntil: 'networkidle' });
  await page.locator('[data-testid="theme-light"], [data-testid="theme-dark"]').first().waitFor();
  const theme = await page.evaluate(() => document.documentElement.dataset.theme);
  if (theme !== 'light' && theme !== 'dark') {
    throw new Error(`theme was not applied: ${theme ?? ''}`);
  }
  await page.locator('[data-testid="party-size-input"]').fill('6');
  await page.locator('[data-testid="search-button"]').click();
  await page.locator('[data-testid="slot-t_1+t_2-18:00"]').waitFor();
  await page.locator('[data-testid="current-user"]').waitFor();
}

async function selectPair(page) {
  return page.evaluate(() => new Promise((resolve) => {
    const cell = document.querySelector('[data-testid="slot-t_1+t_2-18:00"]');
    cell.click();
    queueMicrotask(() => {
      resolve({
        cell: cell.getAttribute('data-selected'),
        plan1: document.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected') ?? null,
        plan2: document.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-selected') ?? null,
        plan3: document.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-selected') ?? null,
        badge: document.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('data-selected') ?? null,
        summary: document.querySelector('[data-testid="booking-summary"]')?.textContent ?? '',
      });
    });
  }));
}

async function run() {
  await mkdir(join(evidence, 'videos'), { recursive: true });
  await writeFile(
    join(evidence, 'LABEL.txt'),
    [
      'TEST ONLY.',
      'Playwright stubs the restaurant API on vite preview of the built page.',
      'The stub is the S2-G pair fixture. It is not the copied Go service.',
      'Pair booking against a real image waits for backend item S2-B and frontend item S2-H.',
      '',
    ].join('\n'),
  );
  const preview = spawn(
    join(webDir, 'node_modules/.bin/vite'),
    ['preview', '--host', '127.0.0.1', '--port', String(port), '--strictPort'],
    { cwd: webDir, stdio: ['ignore', 'pipe', 'pipe'] },
  );
  const browser = await chromium.launch({
    executablePath: chrome,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });
  try {
    await waitForPreview(preview);
    log(`preview ${origin}`);
    const widths = [375, 1280];
    const themes = ['light', 'dark'];
    for (const width of widths) {
      for (const theme of themes) {
        const context = await browser.newContext({
          viewport: { width, height: width === 375 ? 812 : 800 },
          deviceScaleFactor: 1,
        });
        const page = await context.newPage();
        const posts = await prepare(page, theme);
        await search(page);
        await shoot(page, `${theme}-${width}-results`);
        const selected = await selectPair(page);
        proof.immediate.push({ name: `${theme}-${width}-selected`, reduced: false, ...selected });
        log(`selected ${theme}-${width} ${JSON.stringify(selected)}`);
        if (selected.cell !== 'true' || selected.plan1 !== 'true' || selected.plan2 !== 'true' || selected.badge !== 'true') {
          throw new Error(`selection was not true immediately for ${theme} ${width}`);
        }
        if (!selected.summary.includes('Table 1') || !selected.summary.includes('Table 2') || selected.summary.includes('t_1')) {
          throw new Error(`summary labels were wrong: ${selected.summary}`);
        }
        await page.waitForTimeout(400);
        await shoot(page, `${theme}-${width}-selected`);
        await context.close();
        proof.posts.push(...posts.map((post) => ({ theme, width, state: 'selected', ...post })));

        for (const state of ['confirmed', 'uncertain', 'refused']) {
          const next = await browser.newContext({
            viewport: { width, height: width === 375 ? 812 : 800 },
            deviceScaleFactor: 1,
          });
          const view = await next.newPage();
          view.__postMode = state === 'confirmed' ? 'confirm' : state;
          const sent = await prepare(view, theme);
          await search(view);
          await view.locator('[data-testid="slot-t_1+t_2-18:00"]').click();
          await view.locator('[data-testid="booking-submit"]').click();
          if (state === 'confirmed') {
            await view.locator('[data-testid="confirmation-reference"]').waitFor();
            const reference = await view.locator('[data-testid="confirmation-reference"]').innerText();
            if (reference !== 'PAIR01') throw new Error(`reference ${reference}`);
          } else if (state === 'uncertain') {
            await view.locator('[data-testid="booking-uncertain"]').waitFor();
            if (await view.locator('[data-testid="booking-error"]').count()) throw new Error('uncertain showed booking-error');
            if (await view.locator('[data-testid="confirmation"]').count()) throw new Error('uncertain showed confirmation');
          } else {
            await view.locator('[data-testid="booking-error"]').waitFor();
            await view.locator('[data-testid="slot-t_1+t_2-18:00"][data-available="false"]').waitFor();
            if (await view.locator('[data-testid="confirmation"]').count()) throw new Error('refused showed confirmation');
          }
          await shoot(view, `${theme}-${width}-${state}`);
          proof.posts.push(...sent.map((post) => ({ theme, width, state, ...post })));
          await next.close();
        }

        const lookupContext = await browser.newContext({
          viewport: { width, height: width === 375 ? 812 : 800 },
          deviceScaleFactor: 1,
        });
        const lookup = await lookupContext.newPage();
        await prepare(lookup, theme);
        await lookup.goto(`${origin}/lookup`, { waitUntil: 'networkidle' });
        await lookup.locator('[data-testid="lookup-reference-input"]').fill('PAIR01');
        await lookup.locator('[data-testid="lookup-submit"]').click();
        await lookup.locator('[data-testid="reservation-status"]').waitFor();
        const status = await lookup.locator('[data-testid="reservation-status"]').innerText();
        const tables = await lookup.locator('[data-testid="reservation-tables"]').innerText();
        if (status !== 'confirmed') throw new Error(`lookup status ${status}`);
        if (!tables.includes('Table 1') || !tables.includes('Table 2')) throw new Error(`lookup tables ${tables}`);
        await shoot(lookup, `${theme}-${width}-lookup`);
        await lookupContext.close();
      }
    }

    for (const width of widths) {
      const context = await browser.newContext({
        viewport: { width, height: width === 375 ? 812 : 800 },
        deviceScaleFactor: 1,
        reducedMotion: 'reduce',
        ...(recordVideos
          ? { recordVideo: { dir: join(evidence, 'videos'), size: { width, height: width === 375 ? 812 : 800 } } }
          : {}),
      });
      const page = await context.newPage();
      await prepare(page, 'light');
      await search(page);
      const selected = await selectPair(page);
      proof.immediate.push({ name: `light-${width}-reduced`, reduced: true, ...selected });
      log(`reduced ${width} ${JSON.stringify(selected)}`);
      if (selected.cell !== 'true' || selected.plan1 !== 'true' || selected.plan2 !== 'true') {
        throw new Error(`reduced-motion selection lagged at ${width}`);
      }
      await shoot(page, `light-${width}-reduced-selected`);
      await page.waitForTimeout(300);
      const video = recordVideos ? page.video() : null;
      await context.close();
      if (video) await video.saveAs(join(evidence, 'videos', `light-${width}-reduced-selection.webm`));
    }

    if (recordVideos) for (const width of widths) {
      const context = await browser.newContext({
        viewport: { width, height: width === 375 ? 812 : 800 },
        deviceScaleFactor: 1,
        recordVideo: { dir: join(evidence, 'videos'), size: { width, height: width === 375 ? 812 : 800 } },
      });
      const page = await context.newPage();
      await prepare(page, 'light');
      await search(page);
      await selectPair(page);
      await page.waitForTimeout(700);
      await page.locator('[data-testid="booking-submit"]').click();
      await page.locator('[data-testid="confirmation-reference"]').waitFor();
      await page.waitForTimeout(600);
      const video = page.video();
      await context.close();
      if (video) await video.saveAs(join(evidence, 'videos', `light-${width}-book-pair.webm`));
    }

    if (proof.pageErrors.length || proof.offOrigin.length) {
      throw new Error(`pageErrors=${proof.pageErrors.length} offOrigin=${proof.offOrigin.length}`);
    }
    const badPost = proof.posts.find(
      (post) =>
        post.keys.includes('table_id') ||
        JSON.stringify(post.table_ids) !== JSON.stringify(['t_1', 't_2']) ||
        post.party_size !== 6 ||
        post.keyLength < 1,
    );
    if (badPost) throw new Error(`unexpected post ${JSON.stringify(badPost)}`);
    const failedBounds = proof.bounds.map((entry) => entry.summary).filter((summary) => boundsFail(summary));
    proof.boundsSummary = proof.bounds.map((entry) => entry.summary);
    await writeFile(join(evidence, 'proof.json'), JSON.stringify(proof, null, 2));
    await writeFile(join(evidence, 'bounds-summary.json'), JSON.stringify(proof.boundsSummary, null, 2));
    log(`shots ${proof.shots.length} posts ${proof.posts.length} fonts ${proof.fontRequests.length} boundsFail=${failedBounds.length}`);
    if (strictBounds && failedBounds.length > 0) {
      throw new Error(
        `state words do not fit: ${failedBounds.map((summary) => `${summary.name} inner=${summary.outsideInner} border=${summary.outsideBorder} collisions=${summary.collisions}`).join('; ')}`,
      );
    }
  } finally {
    await browser.close();
    preview.kill('SIGTERM');
  }
}

run()
  .then(() => process.exit(0))
  .catch(async (error) => {
    console.error(error);
    try {
      await mkdir(evidence, { recursive: true });
      await writeFile(join(evidence, 'proof.json'), JSON.stringify({ ...proof, failed: String(error) }, null, 2));
    } catch {
      // The process exit is the signal that matters.
    }
    process.exit(1);
  });
