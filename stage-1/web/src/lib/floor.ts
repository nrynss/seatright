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
 * `maxColumns` above zero caps the grid so names stay readable on a narrow screen. */
export function layoutRoom(
  tables: readonly { id: string; label: string; capacity: number }[],
  options?: { maxColumns?: number },
): RoomScene {
  const items = tables.map((table) => ({ ...table, drawing: drawTable(table.capacity) }));
  const count = items.length;
  let cols = count <= 1 ? 1 : count <= 4 ? Math.min(count, 2) : Math.ceil(Math.sqrt(count));
  const cap = options?.maxColumns ?? 0;
  if (cap > 0) cols = Math.max(1, Math.min(cols, cap));
  const rows = Math.max(1, Math.ceil(count / Math.max(cols, 1)));
  const gapX = 44;
  const gapY = 40;
  const padL = 96;
  const padR = 72;
  const padT = 88;
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
