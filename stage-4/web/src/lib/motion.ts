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

/**
 * Move a committed outcome into view. The caller publishes the state first.
 * `portionPx` reveals only the start of a taller region, so a long availability
 * grid is not aligned to its last row and the floor plan above that start stays
 * within reach. A short target is fitted whole. Reduced motion scrolls at once.
 */
export function revealOutcome(node: Element | null, portionPx = 0): void {
  if (!node || !node.isConnected || typeof window === 'undefined' || typeof window.scrollBy !== 'function') return;
  const rect = node.getBoundingClientRect();
  const viewH = window.innerHeight || document.documentElement?.clientHeight || 0;
  if (viewH <= 0) return;
  const margin = 16;
  const span = portionPx > 0 ? Math.min(portionPx, Math.max(1, viewH - margin * 2)) : rect.height;
  const top = rect.top;
  const bottom = top + span;
  if (top >= margin && bottom <= viewH - margin) return;
  let delta = top < margin ? top - margin : bottom - (viewH - margin);
  if (top - delta < margin) delta = top - margin;
  if (Math.abs(delta) < 1) return;
  window.scrollBy({ top: delta, left: 0, behavior: prefersReducedMotion() ? 'auto' : 'smooth' });
}
