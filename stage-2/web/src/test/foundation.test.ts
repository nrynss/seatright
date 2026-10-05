import { readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { themeScript } from '@nrynss/chaaya/theme';
import { checkTokens } from '@nrynss/chaaya/tokens';
import { contrastGate } from '@nrynss/chaaya/testing';
import { ApiError, api as keelApi, keelErrorParser } from '@nrynss/chaaya/keel';
import { describe, expect, it } from 'vitest';
import { api } from '../lib/client';
import { initialSelection, parsePresentation } from '../lib/demo';
import { drawTable, layoutRoom, roomCollision } from '../lib/floor';
import { bookingSummary, formatClock, formatLongDate, timezoneLabel, zonedInstant } from '../lib/format';
import { motionDuration, prefersReducedMotion } from '../lib/motion';
import { contrastPairs } from '../lib/pairs';
import { previewFixture, slotTestId, tableIsAvailable } from '../lib/preview';
import { chooseCell } from '../lib/selection';

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(here, '../..');

function walk(dir: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === 'node_modules' || entry === 'dist') continue;
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) files.push(...walk(path));
    else files.push(path);
  }
  return files;
}

describe('preview fixture', () => {
  it('matches the assigned sample seating exactly', () => {
    expect(previewFixture.restaurant).toEqual({
      id: 'r_anker',
      name: 'Zum Anker',
      timezone: 'Europe/Berlin',
    });
    expect(previewFixture.date).toBe('2026-09-24');
    expect(previewFixture.partySize).toBe(2);
    expect(previewFixture.tables).toEqual([
      { id: 't_1', label: '1', capacity: 2 },
      { id: 't_2', label: '2', capacity: 4 },
    ]);
    expect(previewFixture.slots).toEqual([
      { time: '18:00', availableTableIds: ['t_1', 't_2'] },
      { time: '18:30', availableTableIds: ['t_1', 't_2'] },
      { time: '19:00', availableTableIds: ['t_1'] },
    ]);
    expect(tableIsAvailable(previewFixture.slots[2], 't_2')).toBe(false);
    expect(slotTestId('t_2', '19:00')).toBe('slot-t_2-19:00');
  });
});

describe('human dates and times', () => {
  it('writes the preview evening in words and resolves Berlin daylight time', () => {
    expect(formatLongDate(previewFixture.date)).toBe('Thursday 24 September 2026');
    expect(formatClock('18:00')).toBe('6:00 PM');
    expect(formatClock('18:30')).toBe('6:30 PM');
    expect(formatClock('19:00')).toBe('7:00 PM');
    expect(formatClock('00:00')).toBe('12:00 AM');
    expect(formatClock('12:00')).toBe('12:00 PM');
    expect(timezoneLabel('2026-09-24', '18:00', 'Europe/Berlin')).toBe('Central European Summer Time');
    const instant = zonedInstant('2026-09-24', '18:00', 'Europe/Berlin');
    expect(instant.toISOString()).toBe('2026-09-24T16:00:00.000Z');
    const summary = bookingSummary('Zum Anker', '1', '2026-09-24', '18:00');
    expect(summary).toContain('Zum Anker');
    expect(summary).toContain('Table 1');
    expect(summary).toContain('18:00');
    expect(summary).toContain('6:00 PM');
  });

  it('rejects dates and times that are not civil values', () => {
    expect(() => formatLongDate('2026-02-31')).toThrow(/Invalid calendar date/);
    expect(() => formatClock('24:00')).toThrow(/Invalid local time/);
  });
});

describe('floor geometry', () => {
  it('draws one seat per place and grows the top with capacity', () => {
    const two = drawTable(2);
    const four = drawTable(4);
    expect(two.kind).toBe('round');
    expect(four.kind).toBe('rect');
    expect(two.seats).toHaveLength(2);
    expect(four.seats).toHaveLength(4);
    expect(four.size).toBeGreaterThan(two.size);
    expect(drawTable(6).size).toBeGreaterThan(four.size);
    expect(drawTable(2).radius).toBeGreaterThan(drawTable(1).radius);
    for (const drawing of [two, four, drawTable(6)]) {
      for (const seat of drawing.seats) {
        expect(seat.x - seat.r).toBeGreaterThanOrEqual(-0.01);
        expect(seat.y - seat.r).toBeGreaterThanOrEqual(-0.01);
        expect(seat.x + seat.r).toBeLessThanOrEqual(drawing.size + 0.01);
        expect(seat.y + seat.r).toBeLessThanOrEqual(drawing.size + 0.01);
      }
    }
  });

  it('keeps every table inside one room for several fixture sizes', () => {
    const preview = layoutRoom([
      { id: 't_1', label: '1', capacity: 2 },
      { id: 't_2', label: '2', capacity: 4 },
    ]);
    expect(preview.tables).toHaveLength(2);
    expect(preview.windows.length).toBeGreaterThan(1);
    expect(roomCollision(preview)).toBeNull();
    expect(preview.tables[0].drawing.seats).toHaveLength(2);
    expect(preview.tables[1].drawing.seats).toHaveLength(4);
    expect(preview.tables[1].drawing.size).toBeGreaterThan(preview.tables[0].drawing.size);
    for (const count of [1, 3, 6, 8]) {
      const scene = layoutRoom(
        Array.from({ length: count }, (_, index) => ({
          id: `t_${index + 1}`,
          label: String(index + 1),
          capacity: [2, 4, 6][index % 3],
        })),
      );
      expect(scene.tables).toHaveLength(count);
      expect(roomCollision(scene), `${count} tables`).toBeNull();
    }
    const many = Array.from({ length: 6 }, (_, index) => ({
      id: `t_${index + 1}`,
      label: `Window ${index + 1}`,
      capacity: 4,
    }));
    const wide = layoutRoom(many);
    const stacked = layoutRoom(many, { maxColumns: 1 });
    expect(stacked.tables).toHaveLength(6);
    expect(roomCollision(stacked)).toBeNull();
    expect(stacked.width).toBeLessThan(wide.width);
    expect(stacked.height).toBeGreaterThan(wide.height);
  });
});

describe('selection', () => {
  it('keeps the current choice when the cell is unavailable', () => {
    const current = { tableId: 't_9', time: '12:15' };
    expect(chooseCell(current, 't_2', '19:00', false)).toBe(current);
    expect(chooseCell(null, 't_2', '19:00', false)).toBeNull();
  });

  it('replaces the choice immediately when the cell is available', () => {
    const next = chooseCell({ tableId: 't_1', time: '18:00' }, 't_4', '21:00', true);
    expect(next).toEqual({ tableId: 't_4', time: '21:00' });
  });

  it('seeds a presentation from the first free table in fixture order', () => {
    expect(initialSelection('results', previewFixture)).toBeNull();
    expect(initialSelection('selected', previewFixture)).toEqual({ tableId: 't_1', time: '18:00' });
    const other = {
      restaurant: { id: 'r_other', name: 'Other', timezone: 'Europe/Berlin' },
      date: '2026-01-05',
      partySize: 3,
      tables: [{ id: 't_window', label: 'Window', capacity: 3 }],
      slots: [{ time: '20:15', availableTableIds: ['t_window'] }],
    };
    expect(initialSelection('confirmed', other)).toEqual({ tableId: 't_window', time: '20:15' });
    expect(parsePresentation('auth')).toBe('results');
    expect(parsePresentation('empty')).toBe('empty');
  });
});

describe('motion', () => {
  it('drops duration when reduced motion is requested', () => {
    expect(prefersReducedMotion()).toBe(false);
    expect(motionDuration(240)).toBe(240);
    const original = window.matchMedia;
    window.matchMedia = ((query: string) => ({
      matches: query.includes('prefers-reduced-motion'),
      media: query,
      onchange: null,
      addListener() {},
      removeListener() {},
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() {
        return false;
      },
    })) as typeof window.matchMedia;
    expect(motionDuration(240)).toBe(0);
    expect(prefersReducedMotion()).toBe(true);
    window.matchMedia = original;
  });
});

describe('Chaaya contracts', () => {
  it('runs on Node 26 with Chaaya 0.3.0', () => {
    expect(process.versions.node.split('.')[0]).toBe('26');
    const pkg = JSON.parse(readFileSync(join(webRoot, 'node_modules/@nrynss/chaaya/package.json'), 'utf8')) as {
      version: string;
      exports: Record<string, unknown>;
    };
    expect(pkg.version).toBe('0.3.0');
    expect(pkg.exports['./wire']).toBeUndefined();
    expect(pkg.exports['./keel']).toBeTruthy();
    expect(pkg.exports['./tokens/reference.css']).toBeTruthy();
  });

  it('parses the reservation error envelope through the Keel adapter', () => {
    expect(api).toBe(keelApi);
    expect(typeof api).toBe('function');
    const parsed = keelErrorParser(
      JSON.stringify({ error: { code: 'table_unavailable', message: 'That table is taken.' } }),
      new Response('', { status: 409 }),
    );
    expect(parsed).toEqual({ code: 'table_unavailable', message: 'That table is taken.' });
    expect(keelErrorParser('not-json', new Response())).toBeUndefined();
    expect(keelErrorParser(JSON.stringify({ error: { code: 12, message: 'no' } }), new Response())).toBeUndefined();
    const failure = new ApiError('Sign in again.', 'unauthenticated', 401);
    expect(failure.code).toBe('unauthenticated');
    expect(failure.status).toBe(401);
  });

  it('fills every token role and passes the contrast gate on the product theme', () => {
    const theme = readFileSync(join(webRoot, 'src/theme.css'), 'utf8');
    const reference = readFileSync(join(webRoot, 'node_modules/@nrynss/chaaya/dist/tokens/reference.css'), 'utf8');
    expect(checkTokens(theme)).toEqual([]);
    expect(checkTokens(reference)).toEqual([]);
    expect(theme.match(/(?<!-)color-scheme\s*:/g)).toHaveLength(4);
    expect(theme).toContain('prefers-color-scheme: dark');
    expect(theme).toContain('data-theme="dark"');
    expect(theme).toContain('data-theme="light"');
    expect(theme).toContain('"Source Serif 4"');
    expect(theme).toContain('"Source Code Pro"');
    expect(() => contrastGate(theme, contrastPairs)).not.toThrow();
  });

  it('ships local serif and numeric faces', () => {
    const css = readFileSync(join(webRoot, 'src/fonts.css'), 'utf8');
    expect(css).toContain('@font-face');
    expect(css).toContain('/fonts/source-serif-4-latin-400-normal.woff2');
    expect(css).toContain('/fonts/source-serif-4-latin-600-normal.woff2');
    expect(css).toContain('/fonts/source-serif-4-latin-700-normal.woff2');
    expect(css).toContain('/fonts/source-code-pro-latin-400-normal.woff2');
    expect(css).toContain('/fonts/source-code-pro-latin-500-normal.woff2');
    expect(css).not.toMatch(/https?:\/\//);
    const app = readFileSync(join(webRoot, 'src/App.svelte'), 'utf8');
    expect(app).toContain("import './fonts.css'");
    expect(app).not.toContain('previewFixture');
    expect(app).not.toContain('readDemo');
    expect(app).not.toContain('SearchScreen');
    for (const file of [
      'source-serif-4-latin-400-normal.woff2',
      'source-serif-4-latin-600-normal.woff2',
      'source-serif-4-latin-700-normal.woff2',
      'source-code-pro-latin-400-normal.woff2',
      'source-code-pro-latin-500-normal.woff2',
      'source-serif-4-OFL.txt',
      'source-code-pro-OFL.txt',
    ]) {
      expect(statSync(join(webRoot, 'public/fonts', file)).size).toBeGreaterThan(100);
    }
  });

  it('paints the stored theme before first paint and ships no remote assets in source', () => {
    const html = readFileSync(join(webRoot, 'index.html'), 'utf8');
    const app = readFileSync(join(webRoot, 'src/App.svelte'), 'utf8');
    const themeControl = readFileSync(join(webRoot, 'src/lib/components/ThemeControl.svelte'), 'utf8');
    expect(html).toContain(themeScript);
    expect(app).toContain("@nrynss/chaaya/tokens/reference.css");
    expect(themeControl).toContain('@nrynss/chaaya/theme');
    // Stage 2 names declared pairs. Later stages stay out of this folder.
    const banned = ['policy_version', 'series_id', 'replan'];
    const sources = walk(join(webRoot, 'src')).filter((path) => /\.(svelte|ts|css)$/.test(path));
    sources.push(join(webRoot, 'index.html'));
    for (const path of sources) {
      if (path.includes(`${join('src', 'test')}`)) continue;
      const text = readFileSync(path, 'utf8').replaceAll('http://www.w3.org/2000/svg', '');
      expect(text, path).not.toMatch(/https?:\/\//);
      expect(text, path).not.toMatch(/fonts\.googleapis|cdn\.|unpkg|jsdelivr|fontshare|typekit/i);
      expect(text, path).not.toMatch(/fetch\s*\(/);
      for (const word of banned) expect(text, path).not.toContain(word);
    }
  });
});
