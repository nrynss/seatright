/**
 * TEST ONLY. Measures dense availability grids on vite preview of the built page.
 * The restaurant API is stubbed. This is not the Go service and not a live pair booking.
 *
 * Ordinary Thursday 18:00–23:00, 90-minute seats:
 *   15-minute steps → 15 slots (18:00–21:30), 75 cells
 *   5-minute steps  → 43 slots (18:00–21:30), 215 cells
 * Three singles stay unavailable for party 6. Both declared pairs stay available.
 *
 * Bounds use the same DOM Range / getBoundingClientRect method as s2-g-capture.mjs.
 * An inner grid rail is a failure only when FAIL_ON_GRID_SCROLL=1.
 * Page-level horizontal overflow always fails. VIDEOS=0 skips recordings.
 */
import { spawn } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const webDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const port = Number(process.env.PORT ?? 4175);
const origin = `http://127.0.0.1:${port}`;
const evidence =
  process.env.EVIDENCE ??
  '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S2-G3/dense';
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const strictBounds = process.env.BOUNDS_STRICT !== '0';
const failOnGridScroll = process.env.FAIL_ON_GRID_SCROLL === '1';
const recordVideos = process.env.VIDEOS === '1';
const modes = new Set((process.env.MODES ?? 'results,selected,uncertain,reduced').split(',').map((item) => item.trim()));
const steps = (process.env.STEPS ?? '15,5').split(',').map((item) => Number(item.trim()));

const OPENS = 18 * 60;
const CLOSES = 23 * 60;
const DURATION = 90;
const DATE = '2027-06-17';

const LABELS = {
  t_1: 'Window alcove',
  t_2: 'Garden corner',
  t_3: 'Hearth booth',
};

const ROWS = [
  { id: 't_1', available: 'false' },
  { id: 't_2', available: 'false' },
  { id: 't_3', available: 'false' },
  { id: 't_1+t_2', available: 'true' },
  { id: 't_2+t_3', available: 'true' },
];

function timesFor(step) {
  const times = [];
  for (let minute = OPENS; minute + DURATION <= CLOSES; minute += step) {
    const hh = String(Math.floor(minute / 60)).padStart(2, '0');
    const mm = String(minute % 60).padStart(2, '0');
    times.push(`${hh}:${mm}`);
  }
  return times;
}

function detailFor(step) {
  return {
    id: 'r_anker',
    name: 'Zum Anker',
    timezone: 'Europe/Berlin',
    slot_minutes: step,
    reservation_duration_minutes: 90,
    cancellation_cutoff_minutes: 120,
    opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
    tables: [
      { id: 't_1', label: LABELS.t_1, capacity: 2 },
      { id: 't_2', label: LABELS.t_2, capacity: 4 },
      { id: 't_3', label: LABELS.t_3, capacity: 4 },
    ],
    combinable: [
      ['t_1', 't_2'],
      ['t_2', 't_3'],
    ],
  };
}

function floorFor(step) {
  return {
    restaurant_id: 'r_anker',
    date: DATE,
    timezone: 'Europe/Berlin',
    slots: timesFor(step).map((time) => ({
      starts_at_local: `${DATE}T${time}`,
      starts_at: `${DATE}T${time}:00+02:00`,
      available_table_ids: [],
      available_options: [
        { table_ids: ['t_1', 't_2'], capacity: 6 },
        { table_ids: ['t_2', 't_3'], capacity: 8 },
      ],
    })),
  };
}

function pairReceipt(step, status = 'confirmed') {
  return {
    reservation_id: 'res_pair',
    reference: 'PAIR01',
    restaurant_id: 'r_anker',
    table_ids: ['t_1', 't_2'],
    party_size: 6,
    status,
    starts_at_local: `${DATE}T18:00`,
    starts_at: `${DATE}T18:00:00+02:00`,
    ends_at: `${DATE}T19:30:00+02:00`,
    created_at: '2026-10-04T22:00:00+00:00',
    slot_minutes: step,
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
  label: 'TEST ONLY dense fixture preview. Not the Go image and not a live pair backend.',
  origin,
  failOnGridScroll,
  shots: [],
  posts: [],
  immediate: [],
  scroll: [],
  audits: [],
  pageErrors: [],
  consoleErrors: [],
  offOrigin: [],
  fontRequests: [],
  bounds: [],
};

function log(message) {
  console.log(message);
}

function expectedCount(step) {
  if (step === 15) return 15;
  if (step === 5) return 43;
  return timesFor(step).length;
}

for (const step of steps) {
  const times = timesFor(step);
  const count = expectedCount(step);
  if (times.length !== count || times[0] !== '18:00' || times[times.length - 1] !== '21:30') {
    throw new Error(`slot generator for ${step} produced ${times.length}: ${times[0]}..${times[times.length - 1]}`);
  }
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
  const left = await page.evaluate(async () => {
    const deadline = performance.now() + 20000;
    let running = [];
    while (performance.now() < deadline) {
      running = document.getAnimations().filter((animation) => (
        animation.playState === 'running' || animation.playState === 'pending'
      ));
      if (running.length === 0) break;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    await new Promise((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(resolve));
    });
    return running.length;
  });
  if (left > 0) log(`settle left ${left} animations`);
  return left;
}

function boundsFail(summary) {
  if (!summary?.present) return true;
  return (
    summary.outsideInner > 0 ||
    summary.outsideBorder > 0 ||
    summary.clipped > 0 ||
    summary.collisions > 0 ||
    summary.whenOverlap > 0 ||
    summary.rowheadOutside > 0 ||
    summary.colheadOutside > 0 ||
    (failOnGridScroll && summary.gridScroll === true)
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
      return { summary: { name: label, present: false, fail: true }, cells: [], rowheads: [], colheads: [] };
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
          (edge.overflowX === 'hidden' || edge.overflowX === 'clip' || wordStyle?.overflowX === 'hidden' || wordStyle?.overflowX === 'clip') &&
          word.scrollWidth > word.clientWidth + 1,
      );
      const when = cell.querySelector('.when');
      const whenShown = Boolean(when && getComputedStyle(when).display !== 'none');
      const whenText = whenShown ? textRect(when) : null;
      let whenGap = null;
      if (text && whenText) whenGap = round(text.left - whenText.right);
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
    const page = document.documentElement;
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
      wrap: { scrollWidth: wrap.scrollWidth, clientWidth: wrap.clientWidth },
      page: { scrollWidth: page.scrollWidth, clientWidth: page.clientWidth },
      rowheadLines: rowheads.map((record) => ({
        text: record.text,
        lines: record.textBox?.lines ?? 0,
        w: record.cell.w,
        h: record.cell.h,
        outside: record.outsideBorder,
      })),
      colheadSample: colheads.slice(0, 3).map((record) => ({
        text: record.text,
        lines: record.textBox?.lines ?? 0,
        w: record.cell.w,
        outside: record.outsideBorder,
      })),
      wordWidths: {
        Free: finite(Math.max(0, ...records.filter((record) => record.word === 'Free' && record.text).map((record) => record.text.w))) || null,
        Taken: finite(Math.max(0, ...records.filter((record) => record.word === 'Taken' && record.text).map((record) => record.text.w))) || null,
        Held: finite(Math.max(0, ...records.filter((record) => record.word === 'Held' && record.text).map((record) => record.text.w))) || null,
      },
      cellWidth: finite(records[0]?.cell?.w ?? NaN),
      worst: records
        .filter((record) => record.outsideInner || record.outsideBorder || record.clipped || record.whenOverlap)
        .slice(0, 6)
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
      collisionSample: collisions.slice(0, 4),
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

async function auditGrid(page, step) {
  const times = timesFor(step);
  return page.evaluate(({ times: expectedTimes, rows }) => {
    const cells = [...document.querySelectorAll('[data-testid="availability-grid"] .cell')];
    const seen = new Map();
    const problems = [];
    for (const cell of cells) {
      const id = cell.getAttribute('data-testid') ?? '';
      const available = cell.getAttribute('data-available');
      const match = /^slot-(.+)-(\d{2}:\d{2})$/.exec(id);
      if (!match) {
        problems.push(`bad id ${id}`);
        continue;
      }
      const row = rows.find((item) => item.id === match[1]);
      if (!row) problems.push(`unexpected row ${id}`);
      if (!expectedTimes.includes(match[2])) problems.push(`unexpected time ${id}`);
      if (row && available !== row.available) problems.push(`${id} available=${available}`);
      seen.set(id, (seen.get(id) ?? 0) + 1);
    }
    const missing = [];
    for (const row of rows) {
      for (const time of expectedTimes) {
        const id = `slot-${row.id}-${time}`;
        const count = seen.get(id) ?? 0;
        if (count !== 1) missing.push(`${id}:${count}`);
      }
    }
    const heads = [...document.querySelectorAll('[data-testid="availability-grid"] .rowhead')].map((element) => element.innerText.replace(/\s+/g, ' ').trim());
    return {
      cells: cells.length,
      distinct: seen.size,
      problems: problems.slice(0, 12),
      problemCount: problems.length,
      missing: missing.slice(0, 12),
      missingCount: missing.length,
      heads,
    };
  }, { times, rows: ROWS });
}

function headsOk(heads) {
  const wanted = [
    `Table ${LABELS.t_1}`,
    `Table ${LABELS.t_2}`,
    `Table ${LABELS.t_3}`,
    `Table ${LABELS.t_1} and Table ${LABELS.t_2}`,
    `Table ${LABELS.t_2} and Table ${LABELS.t_3}`,
  ];
  return wanted.every((label) => heads.some((head) => head.includes(label) && !head.includes('…') && !head.includes('...')));
}

async function shoot(page, name) {
  const animationsLeft = await settle(page);
  await page.locator('[data-testid="availability-grid"]').scrollIntoViewIfNeeded();
  const measured = await measureGrid(page, name);
  measured.summary.animationsLeft = animationsLeft;
  proof.bounds.push(measured);
  const summary = measured.summary;
  if (summary.present) {
    log(
      `bounds ${name} cells=${summary.cells} Free=${summary.counts.Free} Taken=${summary.counts.Taken} Held=${summary.counts.Held} ` +
        `outsideInner=${summary.outsideInner} outsideBorder=${summary.outsideBorder} clipped=${summary.clipped} ` +
        `collisions=${summary.collisions} whenOverlap=${summary.whenOverlap} gridScroll=${summary.gridScroll} ` +
        `minInner=${summary.minInnerClearance} minBorder=${summary.minBorderClearance} minGap=${summary.minNeighborGap} ` +
        `rowheadOut=${summary.rowheadOutside} colheadOut=${summary.colheadOutside} cellW=${summary.cellWidth} ` +
        `wrap=${summary.wrap.scrollWidth}/${summary.wrap.clientWidth} widths=${JSON.stringify(summary.wordWidths)} ` +
        `heads=${JSON.stringify(summary.rowheadLines)}`,
    );
  } else {
    log(`bounds ${name} no grid`);
  }
  const file = join(evidence, `${name}.png`);
  await page.screenshot({ path: file });
  const clip = await page.evaluate(() => {
    const grid = document.querySelector('[data-testid="availability-grid"]');
    if (!grid) return null;
    const rect = grid.getBoundingClientRect();
    const x = Math.max(0, rect.left);
    const y = Math.max(0, rect.top);
    const width = Math.min(rect.width, window.innerWidth - x);
    const height = Math.min(rect.height, 680, window.innerHeight - y);
    if (width < 2 || height < 2) return null;
    return { x, y, width, height };
  });
  if (clip) await page.screenshot({ path: join(evidence, `${name}-grid.png`), clip });
  if (summary.gridScroll) {
    await page.evaluate(() => {
      const wrap = document.querySelector('[data-testid="availability-grid"]');
      if (wrap) wrap.scrollLeft = wrap.scrollWidth;
    });
    await page.screenshot({ path: join(evidence, `${name}-right.png`) });
    await page.evaluate(() => {
      const wrap = document.querySelector('[data-testid="availability-grid"]');
      if (wrap) wrap.scrollLeft = 0;
    });
  }
  const note = page.locator('[data-testid="booking-uncertain"]');
  if (await note.count()) {
    await note.scrollIntoViewIfNeeded();
    await page.screenshot({ path: join(evidence, `${name}-note.png`) });
  }
  const box = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
    theme: document.documentElement.dataset.theme ?? '',
  }));
  const overflow = box.scrollWidth > box.clientWidth + 1;
  proof.shots.push({ name, file, ...box, overflow });
  proof.scroll.push({ name, ...box, overflow });
  log(`${name} pageOverflow=${overflow} ${box.clientWidth}x scroll ${box.scrollWidth} theme=${box.theme}`);
  return overflow;
}

async function prepare(page, theme, step) {
  await page.addInitScript((mode) => {
    localStorage.setItem('chaaya-theme', mode);
    localStorage.setItem(
      'tablekeeper-session',
      JSON.stringify({ token: 'fixture-session', displayName: 'Ada', userId: 'u_ada' }),
    );
  }, theme);
  const posts = [];
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
      await json(route, 200, { restaurants: [{ id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' }] });
      return;
    }
    if (path === '/restaurants/r_anker') {
      await json(route, 200, detailFor(step));
      return;
    }
    if (path === '/availability') {
      await json(route, 200, floorFor(step));
      return;
    }
    if (path === '/reservations' && request.method() === 'POST') {
      const body = JSON.parse(request.postData() || '{}');
      posts.push({
        keys: Object.keys(body).sort(),
        table_ids: body.table_ids ?? null,
        hasTableId: Object.prototype.hasOwnProperty.call(body, 'table_id'),
        party_size: body.party_size,
        starts_at_local: body.starts_at_local,
        keyLength: (request.headers()['idempotency-key'] ?? '').length,
      });
      if ((page.__postMode ?? 'confirm') === 'uncertain') {
        await route.abort('failed');
        return;
      }
      await json(route, 201, pairReceipt(step));
      return;
    }
    await json(route, 404, { error: { code: 'not_found', message: 'missing' } });
  });
  return posts;
}

async function search(page, step) {
  const last = timesFor(step).at(-1);
  await page.goto(`${origin}/`, { waitUntil: 'networkidle' });
  await page.locator('[data-testid="restaurant-select"]').waitFor();
  await page.locator('[data-testid="restaurant-select"] option[value="r_anker"]').waitFor({ state: 'attached' });
  await page.locator('[data-testid="date-input"]').fill(DATE);
  await page.locator('[data-testid="party-size-input"]').fill('6');
  await page.locator('[data-testid="search-button"]').click();
  await page.locator(`[data-testid="slot-t_2+t_3-${last}"]`).waitFor();
  await page.locator('[data-testid="current-user"]').waitFor();
  const audit = await auditGrid(page, step);
  const ok = audit.cells === timesFor(step).length * ROWS.length && audit.missingCount === 0 && audit.problemCount === 0 && headsOk(audit.heads);
  proof.audits.push({ step, ok, ...audit });
  log(`audit step=${step} cells=${audit.cells} missing=${audit.missingCount} problems=${audit.problemCount} headsOk=${headsOk(audit.heads)} ok=${ok}`);
  if (!ok) throw new Error(`grid audit failed for ${step}m: ${JSON.stringify({ cells: audit.cells, missing: audit.missing, problems: audit.problems, heads: audit.heads })}`);
}

async function selectPair(page) {
  return page.evaluate(() => new Promise((resolve) => {
    const cell = document.querySelector('[data-testid="slot-t_1+t_2-18:00"]');
    cell.click();
    queueMicrotask(() => {
      resolve({
        cell: cell.getAttribute('data-selected'),
        available: cell.getAttribute('data-available'),
        plan1: document.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected') ?? null,
        plan2: document.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-selected') ?? null,
        plan3: document.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-selected') ?? null,
        badge: document.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('data-selected') ?? null,
        summary: document.querySelector('[data-testid="booking-summary"]')?.textContent ?? '',
      });
    });
  }));
}

function summaryOk(summary) {
  return summary.includes('Window alcove')
    && summary.includes('Garden corner')
    && summary.includes('Zum Anker')
    && summary.includes('18:00')
    && !summary.includes('t_1')
    && !summary.includes('t_2');
}

async function run() {
  await mkdir(evidence, { recursive: true });
  await writeFile(
    join(evidence, 'LABEL.txt'),
    [
      'TEST ONLY.',
      'Playwright stubs the restaurant API on vite preview of the built page.',
      'Dense grids: 15-minute (15 slots, 75 cells) and 5-minute (43 slots, 215 cells).',
      'Party 6. Singles unavailable. Declared pairs t_1+t_2 and t_2+t_3 available.',
      'Labels: Window alcove, Garden corner, Hearth booth.',
      'Not the copied Go service. Real pair booking waits for S2-B and S2-H.',
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
  const pageOverflows = [];
  try {
    await waitForPreview(preview);
    log(`preview ${origin} steps=${steps.join(',')} modes=${[...modes].join(',')} failOnGridScroll=${failOnGridScroll}`);
    const widths = [375, 1280];
    const themes = ['light', 'dark'];
    for (const step of steps) {
      for (const width of widths) {
        for (const theme of themes) {
          const context = await browser.newContext({
            viewport: { width, height: width === 375 ? 812 : 800 },
            deviceScaleFactor: 1,
          });
          const page = await context.newPage();
          page.setDefaultTimeout(60000);
          await prepare(page, theme, step);
          await search(page, step);
          if (modes.has('results')) {
            if (await shoot(page, `m${step}-${theme}-${width}-results`)) pageOverflows.push(`m${step}-${theme}-${width}-results`);
          }
          if (modes.has('selected')) {
            const selected = await selectPair(page);
            proof.immediate.push({ name: `m${step}-${theme}-${width}-selected`, reduced: false, ...selected });
            log(`selected m${step} ${theme}-${width} cell=${selected.cell} plans=${selected.plan1}/${selected.plan2}/${selected.plan3} badge=${selected.badge}`);
            if (selected.cell !== 'true' || selected.available !== 'true' || selected.plan1 !== 'true' || selected.plan2 !== 'true' || selected.plan3 === 'true' || selected.badge !== 'true') {
              throw new Error(`selection was not immediate for ${step} ${theme} ${width}`);
            }
            if (!summaryOk(selected.summary)) throw new Error(`summary labels were wrong: ${selected.summary}`);
            if (await shoot(page, `m${step}-${theme}-${width}-selected`)) pageOverflows.push(`m${step}-${theme}-${width}-selected`);
          }
          await context.close();

          if (modes.has('uncertain')) {
            const next = await browser.newContext({
              viewport: { width, height: width === 375 ? 812 : 800 },
              deviceScaleFactor: 1,
            });
            const view = await next.newPage();
            view.setDefaultTimeout(60000);
            view.__postMode = 'uncertain';
            const sent = await prepare(view, theme, step);
            await search(view, step);
            await view.locator('[data-testid="slot-t_1+t_2-18:00"]').click();
            await view.locator('[data-testid="booking-submit"]').click();
            await view.locator('[data-testid="booking-uncertain"]').waitFor();
            const note = await view.locator('[data-testid="booking-uncertain"]').innerText();
            if (!note.trim()) throw new Error('uncertain text was empty');
            if (await view.locator('[data-testid="booking-error"]').count()) throw new Error('uncertain showed booking-error');
            if (await view.locator('[data-testid="confirmation"]').count()) throw new Error('uncertain showed confirmation');
            const still = await auditGrid(view, step);
            if (still.cells !== timesFor(step).length * ROWS.length || still.missingCount !== 0) {
              throw new Error(`uncertain grid changed: ${still.cells}`);
            }
            if (await shoot(view, `m${step}-${theme}-${width}-uncertain`)) pageOverflows.push(`m${step}-${theme}-${width}-uncertain`);
            proof.posts.push(...sent.map((post) => ({ step, theme, width, state: 'uncertain', ...post })));
            await next.close();
          }
        }
      }

      if (modes.has('reduced')) {
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
          page.setDefaultTimeout(60000);
          await prepare(page, 'light', step);
          await search(page, step);
          const selected = await selectPair(page);
          proof.immediate.push({ name: `m${step}-light-${width}-reduced`, reduced: true, ...selected });
          log(`reduced m${step} ${width} cell=${selected.cell}`);
          if (selected.cell !== 'true' || selected.plan1 !== 'true' || selected.plan2 !== 'true' || selected.badge !== 'true') {
            throw new Error(`reduced-motion selection lagged at ${step} ${width}`);
          }
          if (!summaryOk(selected.summary)) throw new Error(`reduced summary labels were wrong: ${selected.summary}`);
          if (await shoot(page, `m${step}-light-${width}-reduced-selected`)) pageOverflows.push(`m${step}-light-${width}-reduced-selected`);
          const video = recordVideos ? page.video() : null;
          await context.close();
          if (video) await video.saveAs(join(evidence, 'videos', `m${step}-light-${width}-reduced-selection.webm`));
        }
      }
    }

    if (proof.pageErrors.length || proof.offOrigin.length) {
      throw new Error(`pageErrors=${proof.pageErrors.length} offOrigin=${proof.offOrigin.length}`);
    }
    const badPost = proof.posts.find((post) => (
      post.hasTableId ||
      JSON.stringify(post.table_ids) !== JSON.stringify(['t_1', 't_2']) ||
      post.party_size !== 6 ||
      post.starts_at_local !== `${DATE}T18:00` ||
      post.keyLength < 1
    ));
    if (badPost) throw new Error(`unexpected post ${JSON.stringify(badPost)}`);
    const failedBounds = proof.bounds.map((entry) => entry.summary).filter((summary) => boundsFail(summary));
    proof.boundsSummary = proof.bounds.map((entry) => entry.summary);
    proof.pageOverflows = pageOverflows;
    await writeFile(join(evidence, 'proof.json'), JSON.stringify(proof, null, 2));
    await writeFile(join(evidence, 'bounds-summary.json'), JSON.stringify(proof.boundsSummary, null, 2));
    log(`shots ${proof.shots.length} posts ${proof.posts.length} boundsFail=${failedBounds.length} pageOverflows=${pageOverflows.length}`);
    if (pageOverflows.length) {
      throw new Error(`page scrolls sideways: ${pageOverflows.join(', ')}`);
    }
    if (strictBounds && failedBounds.length > 0) {
      throw new Error(
        `state words do not fit: ${failedBounds.map((summary) => `${summary.name} inner=${summary.outsideInner} border=${summary.outsideBorder} collisions=${summary.collisions} row=${summary.rowheadOutside} col=${summary.colheadOutside} scroll=${summary.gridScroll}`).join('; ')}`,
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
      proof.boundsSummary = proof.bounds.map((entry) => entry.summary);
      await writeFile(join(evidence, 'proof.json'), JSON.stringify({ ...proof, failed: String(error) }, null, 2));
      await writeFile(join(evidence, 'bounds-summary.json'), JSON.stringify(proof.boundsSummary, null, 2));
    } catch {
      // The process exit is the signal that matters.
    }
    process.exit(1);
  });
