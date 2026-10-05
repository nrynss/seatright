<script lang="ts">
  import LoginScreen from '../lib/components/LoginScreen.svelte';
  import LookupScreen from '../lib/components/LookupScreen.svelte';
  import SearchScreen from '../lib/components/SearchScreen.svelte';
  import Shell from '../lib/components/Shell.svelte';
  import SignupScreen from '../lib/components/SignupScreen.svelte';
  import type { Presentation } from '../lib/demo';

  let {
    screen = 'search',
    presentation = 'results' as Presentation,
    user = null as string | null,
    authError = null as string | null,
    lookupError = null as string | null,
    reservation = null as {
      reference: string;
      status: 'confirmed' | 'cancelled';
      summary: string;
      tables: string;
    } | null,
  } = $props();

  const path = $derived(screen === 'search' ? '/' : `/${screen}`);
</script>

<Shell {path} {user}>
  {#if screen === 'search'}
    <SearchScreen {presentation} />
  {:else if screen === 'signup'}
    <SignupScreen error={authError} />
  {:else if screen === 'login'}
    <LoginScreen error={authError} />
  {:else}
    <LookupScreen error={lookupError} {reservation} />
  {/if}
</Shell>
