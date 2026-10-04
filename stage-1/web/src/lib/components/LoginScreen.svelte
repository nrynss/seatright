<script lang="ts">
  let { error = null }: { error?: string | null } = $props();

  function onSubmit(event: SubmitEvent): void {
    event.preventDefault();
  }
</script>

<section class="route auth-card">
  <p class="kicker">Welcome back</p>
  <h1 class="display">Sign in</h1>
  <p class="lede">Use the email and password for your Tablekeeper account.</p>
  <form class="stack-form panel" onsubmit={onSubmit}>
    {#if error}
      <p id="auth-error" data-testid="auth-error" class="state state-refused" role="alert">{error}</p>
    {/if}
    <div class="field">
      <label for="login-email">Email</label>
      <input id="login-email" data-testid="login-email" name="email" type="email" autocomplete="email" placeholder="name@example.com" aria-describedby={error ? 'auth-error' : undefined} />
    </div>
    <div class="field">
      <label for="login-password">Password</label>
      <input id="login-password" data-testid="login-password" name="password" type="password" autocomplete="current-password" aria-describedby={error ? 'auth-error' : undefined} />
    </div>
    <button class="btn btn-primary" type="submit" data-testid="login-submit">Sign in</button>
  </form>
</section>
