<script lang="ts">
  import { authMessage } from '../auth';

  let {
    error = null,
    onAccount = null,
  }: {
    error?: string | null;
    onAccount?: ((form: FormData) => Promise<void> | void) | null;
  } = $props();

  let localError = $state<string | null>(null);
  const shown = $derived(localError ?? error);

  async function submitAccount(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (!onAccount) return;
    localError = null;
    const form = new FormData(event.currentTarget as HTMLFormElement);
    try {
      await onAccount(form);
    } catch (failure) {
      localError = authMessage(failure);
    }
  }
</script>

<section class="route auth-card">
  <p class="kicker">Welcome back</p>
  <h1 class="display">Sign in</h1>
  <p class="lede">Use the email and password for your Tablekeeper account.</p>
  <form class="stack-form panel" onsubmit={submitAccount}>
    {#if shown}
      <p id="auth-error" data-testid="auth-error" class="state state-refused" role="alert">{shown}</p>
    {/if}
    <div class="field">
      <label for="login-email">Email</label>
      <input id="login-email" data-testid="login-email" name="email" type="email" autocomplete="email" placeholder="name@example.com" aria-describedby={shown ? 'auth-error' : undefined} />
    </div>
    <div class="field">
      <label for="login-password">Password</label>
      <input id="login-password" data-testid="login-password" name="password" type="password" autocomplete="current-password" aria-describedby={shown ? 'auth-error' : undefined} />
    </div>
    <button class="btn btn-primary" type="submit" data-testid="login-submit">Sign in</button>
  </form>
</section>
