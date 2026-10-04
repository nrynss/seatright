import type { PreviewFixture } from './preview';
import { firstAvailable, type Selection } from './selection';

export const presentations = ['results', 'selected', 'loading', 'empty', 'refused', 'uncertain', 'confirmed'] as const;

export type Presentation = (typeof presentations)[number];

const withSelection = new Set<Presentation>(['selected', 'refused', 'uncertain', 'confirmed']);

export function parsePresentation(value: string | null): Presentation {
  if (value && (presentations as readonly string[]).includes(value)) return value as Presentation;
  return 'results';
}

export function readDemo(search: string): { presentation: Presentation; authError: boolean } {
  const demo = new URLSearchParams(search).get('demo');
  return {
    presentation: parsePresentation(demo),
    authError: demo === 'auth',
  };
}

export function initialSelection(presentation: Presentation, fixture: PreviewFixture): Selection | null {
  if (!withSelection.has(presentation)) return null;
  return firstAvailable(fixture);
}
