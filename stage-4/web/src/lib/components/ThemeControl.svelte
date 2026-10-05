<script lang="ts">
  import { theme } from '@nrynss/chaaya/theme';
  import type { ThemeMode } from '@nrynss/chaaya/theme';

  const modes: { id: ThemeMode; label: string }[] = [
    { id: 'light', label: 'Light' },
    { id: 'dark', label: 'Dark' },
    { id: 'system', label: 'System' },
  ];

  // Chaaya's theme object paints the document, but reading `theme.mode`
  // through its exported getter does not invalidate this component.
  // Keep a local copy so the pressed state updates in the same turn.
  let active = $state<ThemeMode>(theme.mode);

  function choose(mode: ThemeMode): void {
    theme.set(mode);
    active = mode;
  }
</script>

<div class="theme-control" role="group" aria-label="Colour theme">
  {#each modes as mode (mode.id)}
    <button
      type="button"
      data-testid={`theme-${mode.id}`}
      aria-pressed={active === mode.id}
      onclick={() => choose(mode.id)}
    >
      {mode.label}
    </button>
  {/each}
</div>
