/**
 * TEST ONLY. Measures SVG table plate and caption ink on vite preview of the built page.
 * The restaurant API is stubbed. This is not the Go service and not a live pair booking.
 *
 * Labels: Window alcove, Garden corner, Hearth booth.
 * Party 6 keeps both declared pairs available. Party 9 keeps both unavailable.
 * Bounds compare plate and name getBBox with the table's own top, then screen ink
 * with neighboring tops, seats, labels, badges and the room.
 */
import { spawn } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const webDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const port = Number(process.env.PORT ?? 4178);
const origin = `http://127.0.0.1:${port}`;
const evidence =
  process.env.EVIDENCE ??
  '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S2-G5/labels';
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
  label: 'FIXTURE TRANSPORT ONLY. SVG plate and caption ink on vite preview. Not the Go image and not a live pair backend.',
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
    summary.ownTopEscapes > 0 ||
    summary.neighborTopHits > 0 ||
    summary.neighborSeatHits > 0 ||
    summary.neighborLabelHits > 0 ||
    summary.badgeHits > 0 ||
    summary.nameTopHits > 0 ||
    summary.nameSeatHits > 0 ||
    summary.nameLabelHits > 0 ||
    summary.nameBadgeHits > 0 ||
    summary.outsideRoom > 0 ||
    summary.pageOverflow === true ||
    summary.fontTooSmall === true ||
    summary.namesMissing === true
  );
}

async function measureLabels(page, name) {
  return page.evaluate((payload) => {
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
      if (rects.length === 0) {
        const box = element.getBoundingClientRect();
        if (box.width < 0.5 || box.height < 0.5) return null;
        return { left: box.left, right: box.right, top: box.top, bottom: box.bottom, width: box.width, height: box.height, lines: 1 };
      }
      const left = Math.min(...rects.map((rect) => rect.left));
      const right = Math.max(...rects.map((rect) => rect.right));
      const top = Math.min(...rects.map((rect) => rect.top));
      const bottom = Math.max(...rects.map((rect) => rect.bottom));
      return { left, right, top, bottom, width: right - left, height: bottom - top, lines: rects.length };
    }

    function unionInk(parts) {
      const inks = parts.filter(Boolean);
      if (inks.length === 0) return null;
      const left = Math.min(...inks.map((ink) => ink.left));
      const right = Math.max(...inks.map((ink) => ink.right));
      const top = Math.min(...inks.map((ink) => ink.top));
      const bottom = Math.max(...inks.map((ink) => ink.bottom));
      return { left, right, top, bottom, width: right - left, height: bottom - top, lines: inks.length };
    }

    function unionBBox(parts) {
      const boxes = parts.filter(Boolean);
      if (boxes.length === 0) return null;
      const x = Math.min(...boxes.map((box) => box.x));
      const y = Math.min(...boxes.map((box) => box.y));
      const right = Math.max(...boxes.map((box) => box.x + box.width));
      const bottom = Math.max(...boxes.map((box) => box.y + box.height));
      return { x, y, width: right - x, height: bottom - y };
    }

    function ellipseOf(node) {
      const rect = node.getBoundingClientRect();
      return {
        cx: rect.left + rect.width / 2,
        cy: rect.top + rect.height / 2,
        rx: rect.width / 2,
        ry: rect.height / 2,
      };
    }

    function hitsEllipse(ink, ellipse, slack = 0.5) {
      if (!ink) return false;
      const rx = Math.max(1, ellipse.rx - slack);
      const ry = Math.max(1, ellipse.ry - slack);
      const x = Math.max(ink.left, Math.min(ellipse.cx, ink.right));
      const y = Math.max(ink.top, Math.min(ellipse.cy, ink.bottom));
      const nx = (x - ellipse.cx) / rx;
      const ny = (y - ellipse.cy) / ry;
      return nx * nx + ny * ny < 1;
    }

    function hitsRect(ink, rect, slack = 0.5) {
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

    function insideOwnTop(bbox, top) {
      if (!bbox || !top) return false;
      const corners = [
        [bbox.x, bbox.y],
        [bbox.x + bbox.width, bbox.y],
        [bbox.x, bbox.y + bbox.height],
        [bbox.x + bbox.width, bbox.y + bbox.height],
      ];
      if (top.tagName.toLowerCase() === 'circle') {
        const cx = Number(top.getAttribute('cx'));
        const cy = Number(top.getAttribute('cy'));
        const radius = Number(top.getAttribute('r'));
        return corners.every(([x, y]) => Math.hypot(x - cx, y - cy) <= radius - 1);
      }
      const x = Number(top.getAttribute('x'));
      const y = Number(top.getAttribute('y'));
      const w = Number(top.getAttribute('width'));
      const h = Number(top.getAttribute('height'));
      return corners.every(([px, py]) => px >= x + 1 && py >= y + 1 && px <= x + w - 1 && py <= y + h - 1);
    }

    const scene = document.querySelector('svg.room-scene');
    const pageBox = document.documentElement;
    if (!scene) {
      return {
        summary: {
          name: payload.name,
          present: false,
          fail: true,
          pageOverflow: pageBox.scrollWidth > pageBox.clientWidth + 1,
        },
        tables: [],
      };
    }

    const sceneRect = scene.getBoundingClientRect();
    const view = scene.viewBox.baseVal;
    const scale = sceneRect.width / (view.width || 1);
    const groups = [...scene.querySelectorAll('.table-on-plan')];
    const badges = [...scene.querySelectorAll('.pair-badge')].map((badge) => {
      const rect = badge.querySelector('rect');
      const text = badge.querySelector('text');
      return {
        id: badge.getAttribute('data-testid'),
        caption: text ? text.textContent.trim() : '',
        rect: rect ? rect.getBoundingClientRect() : null,
        text: text ? text.getBBox() : null,
      };
    });

    const tables = groups.map((group) => {
      const id = group.getAttribute('data-testid') ?? '';
      const key = id.replace('plan-', '');
      const label = payload.labels[key] ?? '';
      const top = group.querySelector('.table-top');
      const plates = [...group.querySelectorAll('.plate-label')];
      const names = [...group.querySelectorAll('.plan-name')];
      const seats = [...group.querySelectorAll('.seat')];
      const plateBoxes = plates.map((node) => node.getBBox());
      const nameBoxes = names.map((node) => node.getBBox());
      const plateBBox = unionBBox(plateBoxes);
      const nameBBox = unionBBox(nameBoxes);
      const plateInk = unionInk(plates.map((node) => textInk(node)));
      const nameInk = unionInk(names.map((node) => textInk(node)));
      const plateStyle = plates[0] ? getComputedStyle(plates[0]) : null;
      const nameStyle = names[0] ? getComputedStyle(names[0]) : null;
      const plateFont = plateStyle ? parseFloat(plateStyle.fontSize) : 0;
      const nameFont = nameStyle ? parseFloat(nameStyle.fontSize) : 0;
      const ownSeats = seats.map((node) => ellipseOf(node));
      const topEllipse = top ? ellipseOf(top) : null;
      const others = groups.filter((other) => other !== group);
      const otherTops = others.map((other) => other.querySelector('.table-top')).filter(Boolean);
      const otherSeats = others.flatMap((other) => [...other.querySelectorAll('.seat')]);
      const otherPlates = others.flatMap((other) => [...other.querySelectorAll('.plate-label')]).map((node) => textInk(node)).filter(Boolean);
      const otherNames = others.flatMap((other) => [...other.querySelectorAll('.plan-name')]).map((node) => textInk(node)).filter(Boolean);
      const hitTop = (ink, node) => {
        if (!ink || !node) return false;
        if (node.tagName.toLowerCase() === 'circle') return hitsEllipse(ink, ellipseOf(node));
        return hitsRect(ink, node.getBoundingClientRect());
      };
      const neighborTopHits = plateInk ? otherTops.filter((node) => hitTop(plateInk, node)).length : 0;
      const neighborSeatHits = plateInk ? otherSeats.filter((node) => hitsEllipse(plateInk, ellipseOf(node))).length : 0;
      const ownSeatHits = plateInk ? ownSeats.filter((seat) => hitsEllipse(plateInk, seat)).length : 0;
      const neighborLabelHits = plateInk
        ? otherPlates.filter((ink) => hitsRect(plateInk, ink)).length + otherNames.filter((ink) => hitsRect(plateInk, ink)).length
        : 0;
      const badgeHits = plateInk ? badges.filter((badge) => badge.rect && hitsRect(plateInk, badge.rect)).length : 0;
      const nameTopHits = nameInk ? otherTops.filter((node) => hitTop(nameInk, node)).length : 0;
      const nameSeatHits = nameInk
        ? otherSeats.filter((node) => hitsEllipse(nameInk, ellipseOf(node))).length
          + ownSeats.filter((seat) => hitsEllipse(nameInk, seat)).length
        : 0;
      const nameOwnTop = nameInk && top ? hitTop(nameInk, top) : false;
      const nameLabelHits = nameInk
        ? otherPlates.filter((ink) => hitsRect(nameInk, ink)).length + otherNames.filter((ink) => hitsRect(nameInk, ink)).length
        : 0;
      const nameBadgeHits = nameInk ? badges.filter((badge) => badge.rect && hitsRect(nameInk, badge.rect)).length : 0;
      const plateRoom = plateInk ? outside(plateInk, sceneRect) : null;
      const nameRoom = nameInk ? outside(nameInk, sceneRect) : null;
      const plateText = plates.map((node) => node.textContent.trim()).filter(Boolean).join(' ');
      const nameText = names.map((node) => node.textContent.trim()).filter(Boolean).join(' ');
      const aria = group.getAttribute('aria-label') ?? '';
      const fullOnPlate = plateText === label;
      const fullOnName = nameText === `Table ${label}`;
      const fullOnAria = aria.includes(label);
      return {
        id,
        available: group.getAttribute('data-available'),
        selected: group.getAttribute('data-selected'),
        capacity: group.getAttribute('data-capacity'),
        label,
        aria,
        plateText,
        nameText,
        fullOnPlate,
        fullOnName,
        fullOnAria,
        lines: plates.length,
        plateFont: round(plateFont),
        nameFont: round(nameFont),
        plateAttr: plates[0]?.getAttribute('font-size') ?? '',
        nameAttr: names[0]?.getAttribute('font-size') ?? '',
        plate: plateBBox ? boxOf(plateBBox) : null,
        name: nameBBox ? boxOf(nameBBox) : null,
        plateScreen: plateInk ? clientOf(plateInk) : null,
        nameScreen: nameInk ? clientOf(nameInk) : null,
        top: top ? boxOf(top.getBBox()) : null,
        topKind: top ? top.tagName.toLowerCase() : '',
        topRadius: top && top.tagName.toLowerCase() === 'circle' ? Number(top.getAttribute('r')) : null,
        ownTopEscape: plateBBox && top ? !insideOwnTop(plateBBox, top) : true,
        ownSeatHits,
        neighborTopHits,
        neighborSeatHits,
        neighborLabelHits,
        badgeHits,
        nameTopHits: nameTopHits + (nameOwnTop ? 1 : 0),
        nameSeatHits,
        nameLabelHits,
        nameBadgeHits,
        plateOutsideRoom: plateRoom ? escaped(plateRoom) : true,
        nameOutsideRoom: nameRoom ? escaped(nameRoom) : true,
        plateRoom: plateRoom,
        nameRoom: nameRoom,
      };
    });

    const cards = [...document.querySelectorAll('.place-cards li')].map((node) => node.textContent ?? '').join(' | ');
    const heads = [...document.querySelectorAll('.rowhead')].map((node) => node.textContent ?? '').join(' | ');
    const summaryText = document.querySelector('[data-testid="booking-summary"]')?.textContent ?? '';
    const namesMissing = tables.some((table) => (
      !table.fullOnPlate || !table.fullOnName || !table.fullOnAria || !cards.includes(table.label) || !heads.includes(table.label)
    ));
    const fonts = tables.flatMap((table) => [table.plateFont, table.nameFont]).filter((font) => font > 0);
    const minFontPx = fonts.length ? Math.min(...fonts) : null;
    const summary = {
      name: payload.name,
      present: tables.length === 3,
      tables: tables.length,
      scale: round(scale),
      view: { w: view.width, h: view.height },
      scene: clientOf(sceneRect),
      ownTopEscapes: tables.filter((table) => table.ownTopEscape).length,
      neighborTopHits: tables.reduce((sum, table) => sum + table.neighborTopHits, 0),
      neighborSeatHits: tables.reduce((sum, table) => sum + table.neighborSeatHits + table.ownSeatHits, 0),
      neighborLabelHits: tables.reduce((sum, table) => sum + table.neighborLabelHits, 0),
      badgeHits: tables.reduce((sum, table) => sum + table.badgeHits, 0),
      nameTopHits: tables.reduce((sum, table) => sum + table.nameTopHits, 0),
      nameSeatHits: tables.reduce((sum, table) => sum + table.nameSeatHits, 0),
      nameLabelHits: tables.reduce((sum, table) => sum + table.nameLabelHits, 0),
      nameBadgeHits: tables.reduce((sum, table) => sum + table.nameBadgeHits, 0),
      outsideRoom: tables.filter((table) => table.plateOutsideRoom || table.nameOutsideRoom).length,
      minFontPx,
      fontTooSmall: fonts.some((font) => font < 11),
      namesMissing,
      pageOverflow: pageBox.scrollWidth > pageBox.clientWidth + 1,
      page: { scrollWidth: pageBox.scrollWidth, clientWidth: pageBox.clientWidth },
      cards,
      heads,
      summaryText,
      badges: badges.map((badge) => ({
        id: badge.id,
        caption: badge.caption,
        text: badge.text ? boxOf(badge.text) : null,
      })),
      places: tables.map((table) => ({
        id: table.id,
        available: table.available,
        selected: table.selected,
        plateText: table.plateText,
        nameText: table.nameText,
        lines: table.lines,
        plateFont: table.plateFont,
        nameFont: table.nameFont,
        plateAttr: table.plateAttr,
        nameAttr: table.nameAttr,
        plate: table.plate,
        name: table.name,
        top: table.top,
        topKind: table.topKind,
        topRadius: table.topRadius,
        ownTopEscape: table.ownTopEscape,
        ownSeatHits: table.ownSeatHits,
        neighborTopHits: table.neighborTopHits,
        neighborSeatHits: table.neighborSeatHits,
        neighborLabelHits: table.neighborLabelHits,
        badgeHits: table.badgeHits,
        nameTopHits: table.nameTopHits,
        nameSeatHits: table.nameSeatHits,
        nameLabelHits: table.nameLabelHits,
        nameBadgeHits: table.nameBadgeHits,
        plateOutsideRoom: table.plateOutsideRoom,
        nameOutsideRoom: table.nameOutsideRoom,
        plateScreen: table.plateScreen,
        nameScreen: table.nameScreen,
      })),
    };
    summary.fail = (
      !summary.present ||
      summary.ownTopEscapes > 0 ||
      summary.neighborTopHits > 0 ||
      summary.neighborSeatHits > 0 ||
      summary.neighborLabelHits > 0 ||
      summary.badgeHits > 0 ||
      summary.nameTopHits > 0 ||
      summary.nameSeatHits > 0 ||
      summary.nameLabelHits > 0 ||
      summary.nameBadgeHits > 0 ||
      summary.outsideRoom > 0 ||
      summary.pageOverflow ||
      summary.fontTooSmall ||
      summary.namesMissing
    );
    return { summary, tables };
  }, { name, labels: LABELS });
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
  await page.locator('[data-testid="plan-t_1"]').waitFor();
}

function labelsOk(text) {
  return text.includes(LABELS.t_1)
    && text.includes(LABELS.t_2)
    && !text.includes('t_1')
    && !text.includes('t_2');
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
  const measured = await measureLabels(page, name);
  measured.summary.animationsLeft = animationsLeft;
  proof.bounds.push(measured);
  const summary = measured.summary;
  log(
    `bounds ${name} tables=${summary.tables} ownTop=${summary.ownTopEscapes} neighborTops=${summary.neighborTopHits} ` +
      `seats=${summary.neighborSeatHits} labels=${summary.neighborLabelHits} badges=${summary.badgeHits} ` +
      `nameTops=${summary.nameTopHits} nameSeats=${summary.nameSeatHits} nameLabels=${summary.nameLabelHits} ` +
      `nameBadges=${summary.nameBadgeHits} room=${summary.outsideRoom} font=${summary.minFontPx} ` +
      `scale=${summary.scale} view=${summary.view?.w}x${summary.view?.h} ` +
      `page=${summary.page?.clientWidth}/${summary.page?.scrollWidth} namesMissing=${summary.namesMissing} fail=${summary.fail}`,
  );
  for (const place of summary.places ?? []) {
    log(
      `  ${place.id} sel=${place.selected} lines=${place.lines} plateFont=${place.plateFont} nameFont=${place.nameFont} ` +
        `plate=${place.plate?.w}x${place.plate?.h} name=${place.name?.w}x${place.name?.h} ` +
        `top=${place.topKind}:${place.top?.w}x${place.top?.h} r=${place.topRadius} ` +
        `escape=${place.ownTopEscape} tops=${place.neighborTopHits} seats=${place.neighborSeatHits}/${place.ownSeatHits} ` +
        `labels=${place.neighborLabelHits} badges=${place.badgeHits} nameHits=${place.nameTopHits}/${place.nameSeatHits}/${place.nameLabelHits}/${place.nameBadgeHits} ` +
        `plate="${place.plateText}" name="${place.nameText}"`,
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
      'Measures SVG plate and caption ink for Window alcove, Garden corner and Hearth booth.',
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
        const plan = closed.locator('[data-testid="plan-t_1"]');
        const planAvailable = await plan.getAttribute('data-available');
        if (planAvailable !== 'false') throw new Error(`unavailable table was ${planAvailable}`);
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
        `plate ink does not fit: ${failedBounds.map((summary) => `${summary.name} own=${summary.ownTopEscapes} tops=${summary.neighborTopHits} seats=${summary.neighborSeatHits} labels=${summary.neighborLabelHits} badges=${summary.badgeHits} nameTops=${summary.nameTopHits} font=${summary.minFontPx}`).join('; ')}`,
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
