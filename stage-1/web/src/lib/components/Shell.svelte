<script lang="ts">
  import type { Snippet } from 'svelte';
  import Mark from './Mark.svelte';
  import ThemeControl from './ThemeControl.svelte';

  let {
    path = '/',
    user = null,
    onLogout = null,
    children,
  }: {
    path?: string;
    user?: string | null;
    onLogout?: (() => void) | null;
    children?: Snippet;
  } = $props();

  const links = [
    { href: '/', label: 'Search' },
    { href: '/lookup', label: 'Look up' },
    { href: '/login', label: 'Sign in' },
    { href: '/signup', label: 'Create account' },
  ];

  function follow(event: MouseEvent, href: string): void {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    history.pushState({}, '', href);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }
</script>

<div class="shell">
  <a class="skip" href="#content">Skip to content</a>
  <header class="topbar">
    <a class="brand" href="/" onclick={(event) => follow(event, '/')}>
      <Mark />
      <span>Tablekeeper</span>
    </a>
    <nav class="nav" aria-label="Primary">
      {#each links as link (link.href)}
        <a
          href={link.href}
          aria-current={path === link.href ? 'page' : undefined}
          onclick={(event) => follow(event, link.href)}
        >
          {link.label}
        </a>
      {/each}
    </nav>
    <div class="top-tools">
      {#if user !== null}
        <p class="current-user" data-testid="current-user">{user}</p>
        <button type="button" class="btn" data-testid="logout-button" onclick={() => onLogout?.()}>Log out</button>
      {/if}
      <ThemeControl />
    </div>
  </header>
  <main id="content" class="stage">
    {@render children?.()}
  </main>
</div>
