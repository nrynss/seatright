/** Geometry for one table top and the seats around it. Capacity changes both the top and the seat count. */

export interface SeatPoint {
  x: number;
  y: number;
  r: number;
}

export interface TableRect {
  x: number;
  y: number;
  w: number;
  h: number;
  rx: number;
}

export interface TableDrawing {
  kind: 'round' | 'rect';
  size: number;
  cx: number;
  cy: number;
  radius: number;
  rect: TableRect | null;
  seats: SeatPoint[];
  labelSize: number;
}

function pointOnRect(x: number, y: number, w: number, h: number, distance: number): { x: number; y: number } {
  const perimeter = 2 * (w + h);
  let rest = ((distance % perimeter) + perimeter) % perimeter;
  if (rest <= w) return { x: x + rest, y };
  rest -= w;
  if (rest <= h) return { x: x + w, y: y + rest };
  rest -= h;
  if (rest <= w) return { x: x + w - rest, y: y + h };
  rest -= w;
  return { x, y: y + h - rest };
}

function seatsAroundRect(x: number, y: number, w: number, h: number, count: number, seatR: number): SeatPoint[] {
  const perimeter = 2 * (w + h);
  const cx = x + w / 2;
  const cy = y + h / 2;
  const push = seatR + 2;
  const seats: SeatPoint[] = [];
  for (let index = 0; index < count; index += 1) {
    const point = pointOnRect(x, y, w, h, ((index + 0.5) * perimeter) / count);
    const vx = point.x - cx;
    const vy = point.y - cy;
    const length = Math.hypot(vx, vy) || 1;
    seats.push({
      x: point.x + (vx / length) * push,
      y: point.y + (vy / length) * push,
      r: seatR,
    });
  }
  return seats;
}

/** A two-top stays round. Larger tops become rounded rectangles and grow with each seat. */
export function drawTable(capacity: number): TableDrawing {
  const count = Math.max(0, Math.floor(capacity));
  const seatR = 7;
  if (count <= 2) {
    const radius = 20 + count * 8;
    const orbit = radius + 14;
    const pad = 6;
    const cx = orbit + seatR + pad;
    const cy = cx;
    const size = cx * 2;
    const seats = Array.from({ length: count }, (_, index) => {
      const turns = count === 0 ? 0 : index / count;
      const angle = -Math.PI / 2 + turns * Math.PI * 2;
      return {
        x: cx + orbit * Math.cos(angle),
        y: cy + orbit * Math.sin(angle),
        r: seatR,
      };
    });
    return {
      kind: 'round',
      size,
      cx,
      cy,
      radius,
      rect: null,
      seats,
      labelSize: Math.max(16, radius * 0.85),
    };
  }

  const w = 36 + count * 16;
  const h = 52;
  const margin = seatR * 2 + 12;
  const size = Math.max(w, h) + margin * 2;
  const x = (size - w) / 2;
  const y = (size - h) / 2;
  return {
    kind: 'rect',
    size,
    cx: size / 2,
    cy: size / 2,
    radius: Math.min(w, h) / 2,
    rect: { x, y, w, h, rx: 16 },
    seats: seatsAroundRect(x, y, w, h, count, seatR),
    labelSize: Math.max(16, h * 0.42),
  };
}

export interface RoomRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface PlacedTable {
  id: string;
  label: string;
  capacity: number;
  drawing: TableDrawing;
  x: number;
  y: number;
}

export interface RoomScene {
  width: number;
  height: number;
  tables: PlacedTable[];
  windows: RoomRect[];
  aisle: RoomRect;
  bar: RoomRect;
  door: RoomRect;
}

const LABEL_BAND = 28;

function overlaps(a: RoomRect, b: RoomRect): boolean {
  return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
}

/** Place every table in one dining room. Counts other than two use the same aisle, walls and gaps.
 * `maxColumns` above zero caps the grid so names stay readable on a narrow screen.
 * Gap and header options only open extra room for a pair chip. */
export function layoutRoom(
  tables: readonly { id: string; label: string; capacity: number }[],
  options?: { maxColumns?: number; gapX?: number; gapY?: number; padT?: number },
): RoomScene {
  const items = tables.map((table) => ({ ...table, drawing: drawTable(table.capacity) }));
  const count = items.length;
  let cols = count <= 1 ? 1 : count <= 4 ? Math.min(count, 2) : Math.ceil(Math.sqrt(count));
  const cap = options?.maxColumns ?? 0;
  if (cap > 0) cols = Math.max(1, Math.min(cols, cap));
  const rows = Math.max(1, Math.ceil(count / Math.max(cols, 1)));
  const gapX = options?.gapX ?? 44;
  const gapY = options?.gapY ?? 40;
  const padL = 96;
  const padR = 72;
  const padT = options?.padT ?? 88;
  const padB = 84;
  const colW = Array.from({ length: cols }, () => 0);
  const rowH = Array.from({ length: rows }, () => 0);
  items.forEach((item, index) => {
    const col = index % cols;
    const row = Math.floor(index / cols);
    colW[col] = Math.max(colW[col], item.drawing.size);
    rowH[row] = Math.max(rowH[row], item.drawing.size + LABEL_BAND);
  });
  const innerW = colW.reduce((sum, value) => sum + value, 0) + gapX * Math.max(0, cols - 1);
  const innerH = rowH.reduce((sum, value) => sum + value, 0) + gapY * Math.max(0, rows - 1);
  const width = padL + Math.max(innerW, 200) + padR;
  const height = padT + Math.max(innerH, 140) + padB;
  const placed = items.map((item, index) => {
    const col = index % cols;
    const row = Math.floor(index / cols);
    const x =
      padL +
      colW.slice(0, col).reduce((sum, value) => sum + value, 0) +
      gapX * col +
      (colW[col] - item.drawing.size) / 2;
    const y = padT + rowH.slice(0, row).reduce((sum, value) => sum + value, 0) + gapY * row;
    return {
      id: item.id,
      label: item.label,
      capacity: item.capacity,
      drawing: item.drawing,
      x,
      y,
    };
  });
  const windows: RoomRect[] = [];
  const winW = 48;
  const winH = 22;
  for (let x = 64; x + winW < width - 56; x += winW + 26) {
    windows.push({ x, y: 22, w: winW, h: winH });
  }
  return {
    width,
    height,
    tables: placed,
    windows,
    aisle: { x: 34, y: padT - 6, w: 30, h: Math.max(100, innerH) },
    bar: { x: width - 46, y: padT, w: 18, h: Math.min(150, Math.max(96, innerH)) },
    door: { x: width / 2 - 28, y: height - 24, w: 56, h: 16 },
  };
}

/** True when a table's drawn box, including its name, meets another table or the room fittings. */
export function roomCollision(scene: RoomScene): string | null {
  const boxes = scene.tables.map((table) => ({
    id: table.id,
    x: table.x,
    y: table.y,
    w: table.drawing.size,
    h: table.drawing.size + LABEL_BAND,
  }));
  for (const box of boxes) {
    if (box.x < 8 || box.y < 8 || box.x + box.w > scene.width - 8 || box.y + box.h > scene.height - 8) {
      return `${box.id} outside the room`;
    }
    for (const fitting of [scene.aisle, scene.bar, scene.door, ...scene.windows]) {
      if (overlaps(box, fitting)) return `${box.id} meets a fitting`;
    }
  }
  for (let i = 0; i < boxes.length; i += 1) {
    for (let j = i + 1; j < boxes.length; j += 1) {
      if (overlaps(boxes[i], boxes[j])) return `${boxes[i].id} overlaps ${boxes[j].id}`;
    }
  }
  return null;
}

/** Selection spring on a pair chip. Placement reserves this growth. */
export const PAIR_BADGE_SCALE = 1.06;

const PAIR_BADGE_PAD_X = 8;
const PAIR_BADGE_H = 28;
const PAIR_INK_H = 20;
const COMPACT_CAPTION = 'Together';

/** Advances at 13px Source Serif 4, weight 650. A lone space has no ink, so its advance is taken from a phrase. */
const ADVANCE: Record<string, number> = {
  ' ': 3.2,
  '·': 4.06,
  '-': 4.58,
  '–': 7.06,
  '—': 11,
  '&': 10,
  "'": 3.06,
  '’': 3.23,
  '/': 5.08,
  '0': 7.41,
  '1': 7.41,
  '2': 7.41,
  '3': 7.41,
  '4': 8,
  '5': 7.41,
  '6': 8,
  '7': 7.41,
  '8': 7.41,
  '9': 8,
  A: 10,
  B: 9,
  C: 9,
  D: 10,
  E: 8.2,
  F: 8,
  G: 10,
  H: 11,
  I: 5.2,
  J: 6.33,
  K: 10,
  L: 8,
  M: 12.03,
  N: 10,
  O: 10,
  P: 9,
  Q: 10,
  R: 10,
  S: 8,
  T: 9,
  U: 10,
  V: 10,
  W: 14,
  X: 9,
  Y: 9,
  Z: 8,
  a: 8,
  b: 8.36,
  c: 7,
  d: 8.25,
  e: 7.13,
  f: 7,
  g: 8,
  h: 9,
  i: 5,
  j: 6.23,
  k: 9,
  l: 5,
  m: 13,
  n: 9,
  o: 8,
  p: 8.38,
  q: 8.14,
  r: 7,
  s: 6.34,
  t: 5,
  u: 9,
  v: 8,
  w: 12,
  x: 8,
  y: 8,
  z: 7,
};

function advanceOf(ch: string): number {
  return ADVANCE[ch] ?? 14;
}

/** Width of pair-badge text at 13px. The slack covers side bearings so the chip contains the ink. */
export function pairCaptionWidth(caption: string): number {
  let width = 0;
  for (const ch of caption) width += advanceOf(ch);
  return Math.ceil(width + 6);
}

export interface PairBadgeBox {
  x: number;
  y: number;
  w: number;
  h: number;
  text: string;
}

interface PairCaption {
  ids: readonly string[];
  caption: string;
}

function chipFor(text: string): { text: string; w: number; h: number } {
  return {
    text,
    w: Math.max(56, pairCaptionWidth(text) + PAIR_BADGE_PAD_X * 2),
    h: PAIR_BADGE_H,
  };
}

function grown(box: RoomRect): RoomRect {
  const w = box.w * PAIR_BADGE_SCALE;
  const h = box.h * PAIR_BADGE_SCALE;
  return { x: box.x - (w - box.w) / 2, y: box.y - (h - box.h) / 2, w, h };
}

function textBand(x: number, y: number, width: number, height: number): RoomRect {
  return { x: x - width / 2, y, w: width, h: height };
}

function tableObstacles(table: PlacedTable): RoomRect[] {
  const boxes: RoomRect[] = [];
  const drawing = table.drawing;
  if (drawing.kind === 'round') {
    boxes.push({
      x: table.x + drawing.cx - drawing.radius,
      y: table.y + drawing.cy - drawing.radius,
      w: drawing.radius * 2,
      h: drawing.radius * 2,
    });
  } else if (drawing.rect) {
    boxes.push({
      x: table.x + drawing.rect.x,
      y: table.y + drawing.rect.y,
      w: drawing.rect.w,
      h: drawing.rect.h,
    });
  }
  for (const seat of drawing.seats) {
    boxes.push({
      x: table.x + seat.x - seat.r,
      y: table.y + seat.y - seat.r,
      w: seat.r * 2,
      h: seat.r * 2,
    });
  }
  const plateW = pairCaptionWidth(table.label) * (drawing.labelSize / 13);
  const plateH = PAIR_INK_H * (drawing.labelSize / 13);
  boxes.push(textBand(table.x + drawing.cx, table.y + drawing.cy - plateH / 2, plateW, plateH));
  const name = `Table ${table.label}`;
  const nameW = pairCaptionWidth(name) * (16 / 13);
  const nameH = PAIR_INK_H * (16 / 13);
  boxes.push(textBand(table.x + drawing.cx, table.y + drawing.size + 6, nameW, nameH));
  return boxes;
}

function roomObstacles(scene: RoomScene): RoomRect[] {
  return [
    ...scene.tables.flatMap((table) => tableObstacles(table)),
    scene.aisle,
    scene.bar,
    scene.door,
    ...scene.windows,
  ];
}

function chipBox(x: number, y: number, w: number, h: number): RoomRect {
  return { x: x - w / 2, y: y - h / 2, w, h };
}

function insideRoom(box: RoomRect, scene: RoomScene): boolean {
  return box.x >= 8 && box.y >= 8 && box.x + box.w <= scene.width - 8 && box.y + box.h <= scene.height - 8;
}

function blocked(box: RoomRect, obstacles: readonly RoomRect[]): boolean {
  return obstacles.some((obstacle) => overlaps(box, obstacle));
}

function findSpot(
  scene: RoomScene,
  midX: number,
  midY: number,
  chip: { w: number; h: number },
  obstacles: readonly RoomRect[],
): RoomRect | null {
  const clear = (dx: number, dy: number): RoomRect | null => {
    const box = chipBox(midX + dx, midY + dy, chip.w, chip.h);
    const reserved = grown(box);
    if (!insideRoom(reserved, scene) || blocked(reserved, obstacles)) return null;
    return box;
  };
  const origin = clear(0, 0);
  if (origin) return origin;
  for (let radius = 4; radius <= 200; radius += 4) {
    let best: RoomRect | null = null;
    let bestScore = Infinity;
    for (let dx = -radius; dx <= radius; dx += 4) {
      for (let dy = -radius; dy <= radius; dy += 4) {
        if (Math.abs(dx) !== radius && Math.abs(dy) !== radius) continue;
        const box = clear(dx, dy);
        if (!box) continue;
        const score = dx * dx + dy * dy;
        if (score < bestScore) {
          best = box;
          bestScore = score;
        }
      }
    }
    if (best) return best;
  }
  return null;
}

function placeOne(
  scene: RoomScene,
  pair: PairCaption,
  obstacles: RoomRect[],
): PairBadgeBox {
  const left = scene.tables.find((table) => table.id === pair.ids[0]);
  const right = scene.tables.find((table) => table.id === pair.ids[1]);
  const midX = left && right ? (left.x + left.drawing.size / 2 + right.x + right.drawing.size / 2) / 2 : scene.width / 2;
  const midY = left && right ? (left.y + left.drawing.size / 2 + right.y + right.drawing.size / 2) / 2 : scene.height / 2;
  const full = chipFor(pair.caption);
  const found = left && right ? findSpot(scene, midX, midY, full, obstacles) : null;
  const compact = chipFor(COMPACT_CAPTION);
  const spot = found ?? (left && right ? findSpot(scene, midX, midY, compact, obstacles) : null);
  const chip = found ? full : compact;
  const box = spot ?? chipBox(midX, midY, chip.w, chip.h);
  return { x: box.x, y: box.y, w: box.w, h: box.h, text: chip.text };
}

/** Move each pair chip off tabletops, seats and names. Open the room only when the caption needs the space. */
export function roomForPairs(
  tables: readonly { id: string; label: string; capacity: number }[],
  pairs: readonly PairCaption[],
  options?: { maxColumns?: number },
): { scene: RoomScene; badges: PairBadgeBox[] } {
  const gapX = [44, 44, 44, 44, 92, 140];
  const gapY = [40, 40, 76, 112, 112, 112];
  const padT = [88, 136, 136, 184, 184, 184];
  let scene = layoutRoom(tables, options);
  let badges: PairBadgeBox[] = [];
  for (let attempt = 0; attempt < gapX.length; attempt += 1) {
    scene = layoutRoom(tables, {
      maxColumns: options?.maxColumns,
      gapX: gapX[attempt],
      gapY: gapY[attempt],
      padT: padT[attempt],
    });
    const obstacles = roomObstacles(scene);
    badges = pairs.map((pair) => {
      const badge = placeOne(scene, pair, obstacles);
      obstacles.push(grown({ x: badge.x, y: badge.y, w: badge.w, h: badge.h }));
      return badge;
    });
    const missing = badges.some((badge, index) => badge.text !== pairs[index]?.caption);
    if (!missing && pairBadgeCollision(scene, badges) === null) return { scene, badges };
  }
  return { scene, badges };
}

/** Null when every grown chip stays in the room and off tables, seats, names, fittings and other chips. */
export function pairBadgeCollision(scene: RoomScene, badges: readonly PairBadgeBox[]): string | null {
  const obstacles = roomObstacles(scene);
  for (let index = 0; index < badges.length; index += 1) {
    const badge = badges[index];
    const reserved = grown({ x: badge.x, y: badge.y, w: badge.w, h: badge.h });
    if (!insideRoom(reserved, scene)) return `badge ${index} leaves the room`;
    if (blocked(reserved, obstacles)) return `badge ${index} meets the room`;
    for (let other = index + 1; other < badges.length; other += 1) {
      const next = grown({
        x: badges[other].x,
        y: badges[other].y,
        w: badges[other].w,
        h: badges[other].h,
      });
      if (overlaps(reserved, next)) return `badge ${index} meets badge ${other}`;
    }
  }
  return null;
}
