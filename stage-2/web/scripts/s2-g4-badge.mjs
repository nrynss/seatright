/**
 * TEST ONLY. Measures SVG pair-badge ink on vite preview of the built page.
 * The restaurant API is stubbed. This is not the Go service and not a live pair booking.
 *
 * Labels: Window alcove, Garden corner, Hearth booth.
 * Party 6 keeps both declared pairs available. Party 9 keeps both unavailable.
 * Bounds compare text getBBox with the badge rect, then screen ink with tables, seats and the room.
 */
import { spawn } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const webDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const port = Number(process.env.PORT ?? 4176);
const origin = `http://127.0.0.1:${port}`;
const evidence =
  process.env.EVIDENCE ??
  '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S2-G4/badge';
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const strictBounds = process.env.BOUNDS_STRICT !== '0';
const recordVideos = process.env.VIDEOS !== '0';

const DATE = '2027-06-17';
const LABELS = {
  t_1: 'Window alcove',
  t_2: 'Garden corner',
  t_3: 'Hearth booth',
};

function detail() {
  return {
    id: 'r_anker',
    name: 'Zum Anker',
    timezone: 'Europe/Berlin',
    slot_minutes: 30,
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

function floor(party) {
  const options = party <= 8
    ? [
      { table_ids: ['t_1', 't_2'], capacity: 6 },
      { table_ids: ['t_2', 't_3'], capacity: 8 },
    ]
    : [];
  const times = [];
  for (let minute = 18 * 60; minute + 90 <= 23 * 60; minute += 30) {
    const hh = String(Math.floor(minute / 60)).padStart(2, '0');
    const mm = String(minute % 60).padStart(2, '0');
    times.push(`${hh}:${mm}`);
  }
  return {
    restaurant_id: 'r_anker',
    date: DATE,
    timezone: 'Europe/Berlin',
    slots: times.map((time) => ({
      starts_at_local: `${DATE}T${time}`,
      starts_at: `${DATE}T${time}:00+02:00`,
      available_table_ids: [],
      available_options: options,
    })),
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
  label: 'FIXTURE TRANSPORT ONLY. SVG pair-badge ink on vite preview. Not the Go image and not a live pair backend.',
  origin,
  shots: [],
  immediate: [],
  keyboard: [],
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
    summary.textOutsideRect > 0 ||
    summary.textOutsideRoom > 0 ||
    summary.badgeOutsideRoom > 0 ||
    summary.clipped > 0 ||
    summary.tableHits > 0 ||
    summary.seatHits > 0 ||
    summary.labelHits > 0 ||
    summary.badgeHits > 0 ||
    summary.pageOverflow === true ||
    summary.fontTooSmall === true
  );
}

async function measureBadges(page, name) {
  return page.evaluate((label) => {
    const round = (value) => Math.round(value * 100) / 100;
    const boxOf = (box) => ({
      x: round(box.x),
      y: round(box.y),
      w: round(box.width),
      h: round(box.height),
    });
    const clientOf = (rect) => ({
      x: round(rect.left),
      y: round(rect.top),
      w: round(rect.width),
      h: round(rect.height),
    });

    function textInk(element) {
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

    function hits(ink, rect, slack = 0.5) {
      if (!ink || !rect) return false;
      return ink.left < rect.right - slack
        && ink.right > rect.left + slack
        && ink.top < rect.bottom - slack
        && ink.bottom > rect.top + slack;
    }

    function outside(ink, bound) {
      return {
        l: round(ink.left - bound.left),
        r: round(bound.right - ink.right),
        t: round(ink.top - bound.top),
        b: round(bound.bottom - ink.bottom),
      };
    }

    function escaped(gap) {
      return gap.l < -0.5 || gap.r < -0.5 || gap.t < -0.5 || gap.b < -0.5;
    }

    const scene = document.querySelector('svg.room-scene');
    const pageBox = document.documentElement;
    if (!scene) {
      return {
        summary: {
          name: label,
          present: false,
          fail: true,
          pageOverflow: pageBox.scrollWidth > pageBox.clientWidth + 1,
        },
        badges: [],
      };
    }

    const sceneRect = scene.getBoundingClientRect();
    const view = scene.viewBox.baseVal;
    const scale = sceneRect.width / (view.width || 1);
    const tables = [...scene.querySelectorAll('.table-top')].map((node) => node.getBoundingClientRect());
    const seats = [...scene.querySelectorAll('.seat')].map((node) => node.getBoundingClientRect());
    const names = [...scene.querySelectorAll('.plate-label, .plan-name')].map((node) => {
      const ink = textInk(node);
      return ink;
    }).filter(Boolean);

    const badges = [...scene.querySelectorAll('.pair-badge')].map((badge) => {
      const rect = badge.querySelector('rect');
      const text = badge.querySelector('text');
      const rectBox = rect ? rect.getBBox() : null;
      const textBox = text ? text.getBBox() : null;
      const rectClient = rect ? rect.getBoundingClientRect() : null;
      const ink = text ? textInk(text) : null;
      const userGap = rectBox && textBox ? {
        l: round(textBox.x - rectBox.x),
        r: round((rectBox.x + rectBox.width) - (textBox.x + textBox.width)),
        t: round(textBox.y - rectBox.y),
        b: round((rectBox.y + rectBox.height) - (textBox.y + textBox.height)),
      } : null;
      const screenGap = ink && rectClient ? outside(ink, rectClient) : null;
      const roomGap = ink ? outside(ink, sceneRect) : null;
      const badgeRoomGap = rectClient ? outside({
        left: rectClient.left,
        right: rectClient.right,
        top: rectClient.top,
        bottom: rectClient.bottom,
      }, sceneRect) : null;
      const style = text ? getComputedStyle(text) : null;
      const fontPx = style ? parseFloat(style.fontSize) : 0;
      const tableHit = ink ? tables.filter((table) => hits(ink, table)).length : 0;
      const seatHit = ink ? seats.filter((seat) => hits(ink, seat)).length : 0;
      const labelHit = ink ? names.filter((name) => hits(ink, {
        left: name.left,
        right: name.right,
        top: name.top,
        bottom: name.bottom,
      })).length : 0;
      const badgeTableHit = rectClient ? tables.filter((table) => hits(rectClient, table)).length : 0;
      const badgeSeatHit = rectClient ? seats.filter((seat) => hits(rectClient, seat)).length : 0;
      return {
        id: badge.getAttribute('data-testid'),
        available: badge.getAttribute('data-available'),
        selected: badge.getAttribute('data-selected'),
        caption: text ? text.textContent.trim() : '',
        label: badge.getAttribute('aria-label') ?? '',
        fontPx: round(fontPx),
        rect: rectBox ? boxOf(rectBox) : null,
        text: textBox ? boxOf(textBox) : null,
        userGap,
        screenGap,
        roomGap,
        badgeRoomGap,
        textOutsideRect: userGap ? escaped(userGap) : true,
        textOutsideRoom: roomGap ? escaped(roomGap) : true,
        badgeOutsideRoom: badgeRoomGap ? escaped(badgeRoomGap) : true,
        tableHit,
        seatHit,
        labelHit,
        badgeTableHit,
        badgeSeatHit,
        lines: ink?.lines ?? 0,
      };
    });

    const summary = {
      name: label,
      present: badges.length > 0,
      badges: badges.length,
      scale: round(scale),
      view: { w: view.width, h: view.height },
      scene: clientOf(sceneRect),
      textOutsideRect: badges.filter((badge) => badge.textOutsideRect).length,
      textOutsideRoom: badges.filter((badge) => badge.textOutsideRoom).length,
      badgeOutsideRoom: badges.filter((badge) => badge.badgeOutsideRoom).length,
      clipped: badges.filter((badge) => badge.textOutsideRoom).length,
      tableHits: badges.reduce((sum, badge) => sum + badge.tableHit, 0),
      seatHits: badges.reduce((sum, badge) => sum + badge.seatHit, 0),
      labelHits: badges.reduce((sum, badge) => sum + badge.labelHit, 0),
      badgeHits: badges.reduce((sum, badge) => sum + badge.badgeTableHit + badge.badgeSeatHit, 0),
      minFontPx: badges.reduce((min, badge) => Math.min(min, badge.fontPx), Infinity),
      fontTooSmall: badges.some((badge) => badge.fontPx > 0 && badge.fontPx < 11),
      pageOverflow: pageBox.scrollWidth > pageBox.clientWidth + 1,
      page: { scrollWidth: pageBox.scrollWidth, clientWidth: pageBox.clientWidth },
      captions: badges.map((badge) => ({
        id: badge.id,
        available: badge.available,
        selected: badge.selected,
        caption: badge.caption,
        fontPx: badge.fontPx,
        rect: badge.rect,
        text: badge.text,
        userGap: badge.userGap,
        screenGap: badge.screenGap,
        textOutsideRect: badge.textOutsideRect,
        textOutsideRoom: badge.textOutsideRoom,
        tableHit: badge.tableHit,
        seatHit: badge.seatHit,
        labelHit: badge.labelHit,
        badgeTableHit: badge.badgeTableHit,
        badgeSeatHit: badge.badgeSeatHit,
      })),
    };
    if (!Number.isFinite(summary.minFontPx)) summary.minFontPx = null;
    summary.fail = (
      !summary.present ||
      summary.textOutsideRect > 0 ||
      summary.textOutsideRoom > 0 ||
      summary.badgeOutsideRoom > 0 ||
      summary.tableHits > 0 ||
      summary.seatHits > 0 ||
      summary.labelHits > 0 ||
      summary.badgeHits > 0 ||
      summary.pageOverflow ||
      summary.fontTooSmall
    );
    return { summary, badges };
  }, name);
}

async function prepare(page, theme) {
  await page.addInitScript((mode) => {
    localStorage.setItem('chaaya-theme', mode);
    localStorage.setItem(
      'tablekeeper-session',
      JSON.stringify({ token: 'fixture-session', displayName: 'Ada', userId: 'u_ada' }),
    );
  }, theme);
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
      await json(route, 200, detail());
      return;
    }
    if (path === '/availability') {
      const party = Number(url.searchParams.get('party_size') || '0');
      await json(route, 200, floor(party));
      return;
    }
    await json(route, 404, { error: { code: 'not_found', message: 'missing' } });
  });
}

async function search(page, party) {
  await page.goto(`${origin}/`, { waitUntil: 'networkidle' });
  await page.locator('[data-testid="restaurant-select"]').waitFor();
  await page.locator('[data-testid="restaurant-select"] option[value="r_anker"]').waitFor({ state: 'attached' });
  await page.locator('[data-testid="date-input"]').fill(DATE);
  await page.locator('[data-testid="party-size-input"]').fill(String(party));
  await page.locator('[data-testid="search-button"]').click();
  await page.locator('[data-testid="floor-plan"]').waitFor();
  await page.locator('[data-testid="current-user"]').waitFor();
  if (party <= 8) {
    await page.locator('[data-testid="plan-t_1+t_2"]').waitFor();
    await page.locator('[data-testid="plan-t_2+t_3"]').waitFor();
  }
}

function labelsOk(text) {
  return text.includes(LABELS.t_1)
    && text.includes(LABELS.t_2)
    && !text.includes('t_1')
    && !text.includes('t_2');
}

async function readNames(page) {
  return page.evaluate((labels) => {
    const badge = document.querySelector('[data-testid="plan-t_1+t_2"]');
    const other = document.querySelector('[data-testid="plan-t_2+t_3"]');
    const cards = [...document.querySelectorAll('.place-cards li')].map((node) => node.textContent ?? '');
    const heads = [...document.querySelectorAll('.rowhead')].map((node) => node.textContent ?? '');
    const summary = document.querySelector('[data-testid="booking-summary"]')?.textContent ?? '';
    return {
      aria: badge?.getAttribute('aria-label') ?? '',
      otherAria: other?.getAttribute('aria-label') ?? '',
      caption: badge?.querySelector('text')?.textContent ?? '',
      cards: cards.join(' | '),
      heads: heads.join(' | '),
      summary,
      sceneCount: document.querySelectorAll('svg.room-scene').length,
    };
  }, LABELS);
}

async function selectPair(page, testId) {
  return page.evaluate((id) => new Promise((resolve) => {
    const badge = document.querySelector(`[data-testid="${id}"]`);
    badge.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    queueMicrotask(() => {
      const ids = id.replace('plan-', '').split('+');
      resolve({
        badge: badge.getAttribute('data-selected'),
        available: badge.getAttribute('data-available'),
        members: ids.map((tableId) => document.querySelector(`[data-testid="plan-${tableId}"]`)?.getAttribute('data-selected') ?? null),
        other: document.querySelector('[data-testid="plan-t_2+t_3"]')?.getAttribute('data-selected') ?? null,
        cell: document.querySelector(`[data-testid="slot-${id.replace('plan-', '')}-18:00"]`)?.getAttribute('data-selected') ?? null,
        summary: document.querySelector('[data-testid="booking-summary"]')?.textContent ?? '',
      });
    });
  }), testId);
}

async function selectByKey(page, testId) {
  return page.evaluate((id) => new Promise((resolve) => {
    const badge = document.querySelector(`[data-testid="${id}"]`);
    badge.focus();
    const event = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true });
    badge.dispatchEvent(event);
    queueMicrotask(() => {
      resolve({
        badge: badge.getAttribute('data-selected'),
        available: badge.getAttribute('data-available'),
        plan2: document.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-selected') ?? null,
        plan3: document.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-selected') ?? null,
        first: document.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('data-selected') ?? null,
      });
    });
  }), testId);
}

async function shoot(page, name) {
  const animationsLeft = await settle(page);
  await page.locator('[data-testid="floor-plan"]').scrollIntoViewIfNeeded();
  const measured = await measureBadges(page, name);
  measured.summary.animationsLeft = animationsLeft;
  proof.bounds.push(measured);
  const summary = measured.summary;
  log(
    `bounds ${name} badges=${summary.badges} outsideRect=${summary.textOutsideRect} outsideRoom=${summary.textOutsideRoom} ` +
      `badgeRoom=${summary.badgeOutsideRoom} tables=${summary.tableHits} seats=${summary.seatHits} labels=${summary.labelHits} ` +
      `badgeHits=${summary.badgeHits} font=${summary.minFontPx} scale=${summary.scale} view=${summary.view?.w}x${summary.view?.h} ` +
      `page=${summary.page?.clientWidth}/${summary.page?.scrollWidth} fail=${summary.fail}`,
  );
  for (const caption of summary.captions ?? []) {
    log(
      `  ${caption.id} sel=${caption.selected} avail=${caption.available} font=${caption.fontPx} ` +
        `rect=${caption.rect?.w}x${caption.rect?.h} text=${caption.text?.w}x${caption.text?.h} ` +
        `gap=${JSON.stringify(caption.userGap)} screen=${JSON.stringify(caption.screenGap)} ` +
        `hits=${caption.tableHit}/${caption.seatHit}/${caption.labelHit} badgeHits=${caption.badgeTableHit}/${caption.badgeSeatHit} ` +
        `caption=${JSON.stringify(caption.caption)}`,
    );
  }
  const file = join(evidence, `${name}.png`);
  await page.screenshot({ path: file });
  const clip = await page.evaluate(() => {
    const floor = document.querySelector('[data-testid="floor-plan"]');
    if (!floor) return null;
    const rect = floor.getBoundingClientRect();
    const x = Math.max(0, rect.left);
    const y = Math.max(0, rect.top);
    const width = Math.min(rect.width, window.innerWidth - x);
    const height = Math.min(rect.height, 720, window.innerHeight - y);
    if (width < 2 || height < 2) return null;
    return { x, y, width, height };
  });
  if (clip) await page.screenshot({ path: join(evidence, `${name}-floor.png`), clip });
  const box = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
    theme: document.documentElement.dataset.theme ?? '',
  }));
  const overflow = box.scrollWidth > box.clientWidth + 1;
  proof.shots.push({ name, file, ...box, overflow });
  log(`${name} pageOverflow=${overflow} ${box.clientWidth}x scroll ${box.scrollWidth} theme=${box.theme}`);
  return overflow;
}

async function run() {
  await mkdir(evidence, { recursive: true });
  await writeFile(
    join(evidence, 'LABEL.txt'),
    [
      'FIXTURE TRANSPORT ONLY.',
      'Playwright stubs the restaurant API on vite preview of the built page.',
      'Measures SVG pair-badge ink for Window alcove, Garden corner and Hearth booth.',
      'Available party 6, selected combinations, unavailable party 9, light and dark, 375 and 1280.',
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
    log(`preview ${origin}`);
    const widths = [375, 1280];
    const themes = ['light', 'dark'];
    for (const width of widths) {
      for (const theme of themes) {
        const context = await browser.newContext({
          viewport: { width, height: width === 375 ? 812 : 900 },
          deviceScaleFactor: 1,
        });
        const page = await context.newPage();
        page.setDefaultTimeout(60000);
        await prepare(page, theme);
        await search(page, 6);
        const names = await readNames(page);
        if (!labelsOk(names.aria) || !labelsOk(names.cards) || !names.heads.includes(LABELS.t_1) || names.sceneCount !== 1) {
          throw new Error(`names missing before selection ${theme} ${width}: ${JSON.stringify(names)}`);
        }
        if (await shoot(page, `${theme}-${width}-available`)) pageOverflows.push(`${theme}-${width}-available`);

        const selected = await selectPair(page, 'plan-t_1+t_2');
        proof.immediate.push({ name: `${theme}-${width}-selected`, reduced: false, ...selected });
        log(`selected ${theme}-${width} badge=${selected.badge} members=${selected.members.join('/')} cell=${selected.cell}`);
        if (selected.badge !== 'true' || selected.available !== 'true' || selected.members.some((value) => value !== 'true') || selected.cell !== 'true') {
          throw new Error(`selection was not immediate for ${theme} ${width}`);
        }
        if (!labelsOk(selected.summary) || !selected.summary.includes('Zum Anker') || !selected.summary.includes('18:00')) {
          throw new Error(`summary labels were wrong: ${selected.summary}`);
        }
        const keyed = await selectByKey(page, 'plan-t_2+t_3');
        proof.keyboard.push({ name: `${theme}-${width}-keyboard`, ...keyed });
        log(`keyboard ${theme}-${width} badge=${keyed.badge} plan2=${keyed.plan2} plan3=${keyed.plan3} first=${keyed.first}`);
        if (keyed.badge !== 'true' || keyed.available !== 'true' || keyed.plan2 !== 'true' || keyed.plan3 !== 'true' || keyed.first !== 'false') {
          throw new Error(`keyboard selection was not immediate for ${theme} ${width}`);
        }
        if (await shoot(page, `${theme}-${width}-selected`)) pageOverflows.push(`${theme}-${width}-selected`);
        await context.close();

        const unavailable = await browser.newContext({
          viewport: { width, height: width === 375 ? 812 : 900 },
          deviceScaleFactor: 1,
        });
        const closed = await unavailable.newPage();
        closed.setDefaultTimeout(60000);
        await prepare(closed, theme);
        await search(closed, 9);
        const closedBadge = closed.locator('[data-testid="plan-t_1+t_2"]');
        if (await closedBadge.count()) {
          const available = await closedBadge.getAttribute('data-available');
          const pressed = await closedBadge.getAttribute('data-selected');
          if (available !== 'false' || pressed !== 'false') {
            throw new Error(`unavailable badge was ${available}/${pressed}`);
          }
        }
        if (await shoot(closed, `${theme}-${width}-unavailable`)) pageOverflows.push(`${theme}-${width}-unavailable`);
        await unavailable.close();
      }
    }

    for (const width of widths) {
      const context = await browser.newContext({
        viewport: { width, height: width === 375 ? 812 : 900 },
        deviceScaleFactor: 1,
        reducedMotion: 'reduce',
        ...(recordVideos
          ? { recordVideo: { dir: join(evidence, 'videos'), size: { width, height: width === 375 ? 812 : 900 } } }
          : {}),
      });
      const page = await context.newPage();
      page.setDefaultTimeout(60000);
      await prepare(page, 'light');
      await search(page, 6);
      const selected = await selectPair(page, 'plan-t_1+t_2');
      proof.immediate.push({ name: `light-${width}-reduced`, reduced: true, ...selected });
      log(`reduced ${width} badge=${selected.badge} members=${selected.members.join('/')}`);
      if (selected.badge !== 'true' || selected.members.some((value) => value !== 'true') || selected.cell !== 'true') {
        throw new Error(`reduced-motion selection lagged at ${width}`);
      }
      if (!labelsOk(selected.summary)) throw new Error(`reduced summary labels were wrong: ${selected.summary}`);
      if (await shoot(page, `light-${width}-reduced-selected`)) pageOverflows.push(`light-${width}-reduced-selected`);
      const video = recordVideos ? page.video() : null;
      await context.close();
      if (video) await video.saveAs(join(evidence, 'videos', `light-${width}-reduced-selection.webm`));
    }

    if (proof.pageErrors.length || proof.offOrigin.length) {
      throw new Error(`pageErrors=${proof.pageErrors.length} offOrigin=${proof.offOrigin.length}`);
    }
    const failedBounds = proof.bounds.map((entry) => entry.summary).filter((summary) => boundsFail(summary));
    proof.boundsSummary = proof.bounds.map((entry) => entry.summary);
    proof.pageOverflows = pageOverflows;
    await writeFile(join(evidence, 'proof.json'), JSON.stringify(proof, null, 2));
    await writeFile(join(evidence, 'bounds-summary.json'), JSON.stringify(proof.boundsSummary, null, 2));
    log(`shots ${proof.shots.length} boundsFail=${failedBounds.length} pageOverflows=${pageOverflows.length}`);
    if (pageOverflows.length) throw new Error(`page scrolls sideways: ${pageOverflows.join(', ')}`);
    if (strictBounds && failedBounds.length > 0) {
      throw new Error(
        `badge ink does not fit: ${failedBounds.map((summary) => `${summary.name} rect=${summary.textOutsideRect} room=${summary.textOutsideRoom} tables=${summary.tableHits} seats=${summary.seatHits} labels=${summary.labelHits} badgeHits=${summary.badgeHits} font=${summary.minFontPx}`).join('; ')}`,
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
