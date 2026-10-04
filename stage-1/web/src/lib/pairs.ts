import type { ContrastPair } from '@nrynss/chaaya/testing';

/** Foreground and background token roles the screens actually paint. */
export const contrastPairs: readonly ContrastPair[] = [
  ['text', 'ground'],
  ['text', 'surface'],
  ['text', 'raised'],
  ['text', 'sunken'],
  ['text', 'ok-soft'],
  ['text', 'warn-soft'],
  ['text', 'stop-soft'],
  ['dim', 'ground'],
  ['dim', 'surface'],
  ['dim', 'raised'],
  ['dim', 'sunken'],
  ['faint', 'ground'],
  ['faint', 'surface'],
  ['faint', 'raised'],
  ['accent', 'ground'],
  ['accent', 'surface'],
  ['on-accent', 'accent'],
  ['on-accent', 'ok'],
  ['ok', 'surface'],
  ['ok', 'ok-soft'],
  ['warn', 'warn-soft'],
  ['stop', 'stop-soft'],
  ['stop', 'sunken'],
];
