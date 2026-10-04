<script lang="ts">
  import '@nrynss/chaaya/tokens/reference.css';
  import './theme.css';
  import './app.css';
  import { fly } from 'svelte/transition';
  import LoginScreen from './lib/components/LoginScreen.svelte';
  import LookupScreen from './lib/components/LookupScreen.svelte';
  import SearchScreen from './lib/components/SearchScreen.svelte';
  import Shell from './lib/components/Shell.svelte';
  import SignupScreen from './lib/components/SignupScreen.svelte';
  import { readDemo } from './lib/demo';
  import { motionDuration } from './lib/motion';
  import { previewFixture } from './lib/preview';

  function normalize(pathname: string): string {
    if (pathname.length > 1 && pathname.endsWith('/')) return pathname.slice(0, -1);
    return pathname || '/';
  }

  let path = $state(normalize(window.location.pathname));
  let search = $state(window.location.search);

  $effect(() => {
    const sync = () => {
      path = normalize(window.location.pathname);
      search = window.location.search;
    };
    window.addEventListener('popstate', sync);
    return () => window.removeEventListener('popstate', sync);
  });

  const demo = $derived(readDemo(search));
  const loginError = $derived(
    path === '/login' && demo.authError ? 'Those sign-in details were not accepted.' : null,
  );
  const signupError = $derived(
    path === '/signup' && demo.authError ? 'An account with that email already exists.' : null,
  );
  const lookupError = $derived(
    path === '/lookup' && new URLSearchParams(search).get('demo') === 'missing'
      ? 'No reservation matches that reference.'
      : null,
  );

  $effect(() => {
    const name = previewFixture.restaurant.name;
    if (path === '/signup') document.title = 'Create account · Tablekeeper';
    else if (path === '/login') document.title = 'Sign in · Tablekeeper';
    else if (path === '/lookup') document.title = 'Find a reservation · Tablekeeper';
    else document.title = `${name} · Tablekeeper`;
  });
</script>

<Shell {path}>
  {#key `${path}${search}`}
    <div class="route" in:fly={{ y: 8, opacity: 1, duration: motionDuration(200) }}>
      {#if path === '/signup'}
        <SignupScreen error={signupError} />
      {:else if path === '/login'}
        <LoginScreen error={loginError} />
      {:else if path === '/lookup'}
        <LookupScreen error={lookupError} />
      {:else}
        <SearchScreen presentation={demo.presentation} />
      {/if}
    </div>
  {/key}
</Shell>
