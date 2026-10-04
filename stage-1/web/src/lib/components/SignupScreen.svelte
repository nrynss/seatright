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
  <p class="kicker">Join the book</p>
  <h1 class="display">Create your account</h1>
  <p class="lede">A name, an email and a password. The table can then be held for you.</p>
  <form class="stack-form panel" onsubmit={submitAccount}>
    {#if shown}
      <p id="auth-error" data-testid="auth-error" class="state state-refused" role="alert">{shown}</p>
    {/if}
    <div class="field">
      <label for="signup-email">Email</label>
      <input id="signup-email" data-testid="signup-email" name="email" type="email" autocomplete="email" placeholder="name@example.com" aria-describedby={shown ? 'auth-error' : undefined} />
    </div>
    <div class="field">
      <label for="signup-password">Password</label>
      <input id="signup-password" data-testid="signup-password" name="password" type="password" autocomplete="new-password" aria-describedby={shown ? 'auth-error' : undefined} />
    </div>
    <div class="field">
      <label for="signup-display-name">Name</label>
      <input id="signup-display-name" data-testid="signup-display-name" name="display-name" type="text" autocomplete="name" placeholder="Your name" />
    </div>
    <button class="btn btn-primary" type="submit" data-testid="signup-submit">Create account</button>
  </form>
</section>
