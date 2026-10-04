/** Motion helpers. A reduced-motion preference shortens transitions to zero so the state is not held back. */

export function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false;
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

export function motionDuration(ms: number): number {
  return prefersReducedMotion() ? 0 : ms;
}

export function staggerDelay(index: number, step = 40): number {
  return prefersReducedMotion() ? 0 : index * step;
}

export function springOptions(): { stiffness: number; damping: number } {
  if (prefersReducedMotion()) return { stiffness: 1, damping: 1 };
  return { stiffness: 0.16, damping: 0.46 };
}
