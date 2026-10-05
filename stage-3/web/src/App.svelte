<script lang="ts">
  import '@nrynss/chaaya/tokens/reference.css';
  import './fonts.css';
  import './theme.css';
  import './app.css';
  import { fly } from 'svelte/transition';
  import { loginAccount, registerAccount } from './lib/auth';
  import LiveSearch from './lib/components/LiveSearch.svelte';
  import LoginScreen from './lib/components/LoginScreen.svelte';
  import LookupScreen from './lib/components/LookupScreen.svelte';
  import Shell from './lib/components/Shell.svelte';
  import SignupScreen from './lib/components/SignupScreen.svelte';
  import { motionDuration } from './lib/motion';
  import { go } from './lib/nav';
  import { clearSession, loadSession, saveSession, type Session } from './lib/session';
  import { liveTransport } from './lib/transport';

  function normalize(pathname: string): string {
    if (pathname.length > 1 && pathname.endsWith('/')) return pathname.slice(0, -1);
    return pathname || '/';
  }

  let path = $state(normalize(window.location.pathname));
  let search = $state(window.location.search);
  let session = $state<Session | null>(loadSession());

  $effect(() => {
    const sync = () => {
      path = normalize(window.location.pathname);
      search = window.location.search;
    };
    window.addEventListener('popstate', sync);
    return () => window.removeEventListener('popstate', sync);
  });

  function accept(next: Session): void {
    saveSession(next);
    session = next;
    go('/');
  }

  function logout(): void {
    clearSession();
    session = null;
  }

  async function createAccount(form: FormData): Promise<void> {
    const next = await registerAccount(liveTransport, {
      email: String(form.get('email') ?? ''),
      password: String(form.get('password') ?? ''),
      displayName: String(form.get('display-name') ?? ''),
    });
    accept(next);
  }

  async function signIn(form: FormData): Promise<void> {
    const next = await loginAccount(
      liveTransport,
      String(form.get('email') ?? ''),
      String(form.get('password') ?? ''),
    );
    accept(next);
  }

  $effect(() => {
    if (path === '/signup') document.title = 'Create account · Tablekeeper';
    else if (path === '/login') document.title = 'Sign in · Tablekeeper';
    else if (path === '/lookup') document.title = 'Find a reservation · Tablekeeper';
    else document.title = 'Find a table · Tablekeeper';
  });
</script>

<Shell {path} user={session ? session.displayName : null} onLogout={logout}>
  {#key `${path}${search}`}
    <div class="route" in:fly={{ y: 8, opacity: 1, duration: motionDuration(200) }}>
      {#if path === '/signup'}
        <SignupScreen onAccount={createAccount} />
      {:else if path === '/login'}
        <LoginScreen onAccount={signIn} />
      {:else if path === '/lookup'}
        <LookupScreen transport={liveTransport} token={session ? session.token : null} />
      {:else}
        <LiveSearch signedIn={session !== null} token={session ? session.token : null} />
      {/if}
    </div>
  {/key}
</Shell>
