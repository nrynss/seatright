<script lang="ts">
  let { error = null }: { error?: string | null } = $props();

  function onSubmit(event: SubmitEvent): void {
    event.preventDefault();
  }
</script>

<section class="route auth-card">
  <p class="kicker">Join the book</p>
  <h1 class="display">Create your account</h1>
  <p class="lede">A name, an email and a password. The table can then be held for you.</p>
  <form class="stack-form panel" onsubmit={onSubmit}>
    {#if error}
      <p id="auth-error" data-testid="auth-error" class="state state-refused" role="alert">{error}</p>
    {/if}
    <div class="field">
      <label for="signup-email">Email</label>
      <input id="signup-email" data-testid="signup-email" name="email" type="email" autocomplete="email" placeholder="name@example.com" aria-describedby={error ? 'auth-error' : undefined} />
    </div>
    <div class="field">
      <label for="signup-password">Password</label>
      <input id="signup-password" data-testid="signup-password" name="password" type="password" autocomplete="new-password" aria-describedby={error ? 'auth-error' : undefined} />
    </div>
    <div class="field">
      <label for="signup-display-name">Name</label>
      <input id="signup-display-name" data-testid="signup-display-name" name="display-name" type="text" autocomplete="name" placeholder="Your name" />
    </div>
    <button class="btn btn-primary" type="submit" data-testid="signup-submit">Create account</button>
  </form>
</section>
