import { readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { contrastGate } from '@nrynss/chaaya/testing';
import { describe, expect, it } from 'vitest';
import { contrastPairs } from '../lib/pairs';

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..');

type Element = {
  tag: string;
  classes: string[];
  attrs: Record<string, string>;
};

type Rule = {
  selector: string;
  declarations: Record<string, string>;
  order: number;
};

const cell = (together: boolean, available: boolean, selected: boolean): Element => ({
  tag: 'button',
  classes: together ? ['cell', 'together'] : ['cell'],
  attrs: {
    'data-available': available ? 'true' : 'false',
    'data-selected': selected ? 'true' : 'false',
  },
});

const stateWord: Element = { tag: 'span', classes: ['state-word'], attrs: {} };

function specificity(selector: string): [number, number, number] {
  const stripped = selector.replace(/::[a-z-]+/gi, ' ');
  const ids = stripped.match(/#[\w-]+/g)?.length ?? 0;
  const classes = stripped.match(/\.[\w-]+/g)?.length ?? 0;
  const attributes = stripped.match(/\[[^\]]+\]/g)?.length ?? 0;
  const pseudos = stripped.match(/:(?!:)[\w-]+(\([^)]*\))?/g)?.length ?? 0;
  const without = stripped
    .replace(/#[\w-]+/g, ' ')
    .replace(/\.[\w-]+/g, ' ')
    .replace(/\[[^\]]+\]/g, ' ')
    .replace(/:(?!:)[\w-]+(\([^)]*\))?/g, ' ')
    .replace(/[+>~]/g, ' ');
  const elements = without.split(/\s+/).filter((part) => part && part !== '*').length;
  return [ids, classes + attributes + pseudos, elements];
}

function better(left: [number, number, number], right: [number, number, number]): number {
  for (let index = 0; index < 3; index += 1) {
    if (left[index] !== right[index]) return left[index] - right[index];
  }
  return 0;
}

function compoundMatches(compound: string, element: Element): boolean {
  if (/:(?:hover|focus|focus-visible|active|disabled|visited)/.test(compound)) return false;
  if (/::/.test(compound)) return false;
  const tag = compound.match(/^[a-zA-Z][\w-]*/);
  if (tag && tag[0].toLowerCase() !== element.tag) return false;
  const classes = [...compound.matchAll(/\.([\w-]+)/g)].map((match) => match[1]);
  if (!classes.every((name) => element.classes.includes(name))) return false;
  for (const match of compound.matchAll(/\[([\w-]+)(?:([~|^$*]?=)"([^"]*)")?\]/g)) {
    const [, name, operator, value] = match;
    if (!(name in element.attrs)) return false;
    if (operator === '=' && element.attrs[name] !== value) return false;
  }
  return true;
}

function selectorMatches(selector: string, element: Element, parent: Element | null): boolean {
  const parts = selector.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 1) return compoundMatches(parts[0], element);
  if (parts.length === 2 && parent) return compoundMatches(parts[0], parent) && compoundMatches(parts[1], element);
  return false;
}

function parseRules(css: string): Rule[] {
  const text = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const rules: Rule[] = [];
  let order = 0;

  function walk(source: string) {
    let index = 0;
    while (index < source.length) {
      while (/\s/.test(source[index] ?? '')) index += 1;
      if (index >= source.length) return;
      if (source[index] === '@') {
        const open = source.indexOf('{', index);
        const header = source.slice(index, open);
        const end = blockEnd(source, open + 1);
        if (!/keyframes/i.test(header)) walk(source.slice(open + 1, end));
        index = end + 1;
        continue;
      }
      const open = source.indexOf('{', index);
      const selector = source.slice(index, open).trim();
      const end = blockEnd(source, open + 1);
      const body = source.slice(open + 1, end);
      const declarations: Record<string, string> = {};
      for (const piece of body.split(';')) {
        const colon = piece.indexOf(':');
        if (colon === -1) continue;
        const property = piece.slice(0, colon).trim();
        const value = piece.slice(colon + 1).trim();
        if (property && !property.startsWith('@')) declarations[property] = value;
      }
      for (const part of selector.split(',')) {
        const trimmed = part.trim();
        if (trimmed) rules.push({ selector: trimmed, declarations, order });
      }
      order += 1;
      index = end + 1;
    }
  }

  function blockEnd(source: string, start: number): number {
    let depth = 1;
    for (let cursor = start; cursor < source.length; cursor += 1) {
      if (source[cursor] === '{') depth += 1;
      else if (source[cursor] === '}') {
        depth -= 1;
        if (depth === 0) return cursor;
      }
    }
    throw new Error('unclosed css block');
  }

  walk(text);
  return rules;
}

function winner(rules: Rule[], element: Element, parent: Element | null, property: string): string {
  let best: { value: string; rank: [number, number, number]; order: number } | null = null;
  for (const rule of rules) {
    if (!(property in rule.declarations)) continue;
    if (!selectorMatches(rule.selector, element, parent)) continue;
    const rank = specificity(rule.selector);
    if (!best || better(rank, best.rank) > 0 || (better(rank, best.rank) === 0 && rule.order >= best.order)) {
      best = { value: rule.declarations[property], rank, order: rule.order };
    }
  }
  return best?.value ?? '';
}

describe('selected combination cell cascade', () => {
  const css = readFileSync(join(webRoot, 'src/app.css'), 'utf8');
  const rules = parseRules(css);
  const theme = readFileSync(join(webRoot, 'src/theme.css'), 'utf8');

  it('keeps the attested token pairs and does not treat the pale pair wash as proof', () => {
    expect(contrastPairs).toHaveLength(25);
    expect(contrastPairs).toContainEqual(['on-accent', 'accent']);
    expect(contrastPairs).not.toContainEqual(['on-accent', 'accent-soft']);
    expect(() => contrastGate(theme, contrastPairs)).not.toThrow();
  });

  it('paints a held available pair with accent and a readable word', () => {
    const heldPair = cell(true, true, true);
    const openPair = cell(true, true, false);
    const heldSingle = cell(false, true, true);
    const openSingle = cell(false, true, false);
    const takenPair = cell(true, false, false);
    const takenSingle = cell(false, false, false);

    expect(winner(rules, heldPair, null, 'background')).toBe('var(--accent)');
    expect(winner(rules, heldPair, null, 'color')).toBe('var(--on-accent)');
    expect(winner(rules, stateWord, heldPair, 'color')).toBe('var(--on-accent)');
    expect(winner(rules, stateWord, heldPair, 'display')).not.toBe('none');

    expect(winner(rules, openPair, null, 'background')).toBe('var(--accent-soft)');
    expect(winner(rules, stateWord, openPair, 'color')).toBe('var(--ok)');

    expect(winner(rules, heldSingle, null, 'background')).toBe('var(--accent)');
    expect(winner(rules, stateWord, heldSingle, 'color')).toBe('var(--on-accent)');
    expect(winner(rules, stateWord, openSingle, 'color')).toBe('var(--ok)');

    expect(winner(rules, takenPair, null, 'background')).toBe('var(--sunken)');
    expect(winner(rules, takenSingle, null, 'background')).toBe('var(--sunken)');
    expect(winner(rules, stateWord, takenPair, 'color')).toBe('var(--stop)');
    expect(winner(rules, stateWord, takenSingle, 'color')).toBe('var(--stop)');
  });
});
