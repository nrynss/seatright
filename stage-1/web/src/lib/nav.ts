/** Move to a screen route without reloading, then tell the shell. */
export function go(href: string): void {
  history.pushState({}, '', href);
  window.dispatchEvent(new PopStateEvent('popstate'));
}
