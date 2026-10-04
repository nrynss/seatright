/**
 * Focused login-copy probe for S1-H2-B1. Talks only to the built image.
 * Prints statuses and copy. It does not print tokens.
 */
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { chromium } from 'playwright';

const base = process.env.BASE ?? 'http://127.0.0.1:9010';
const evidence = process.env.EVIDENCE ?? '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S1-H2-B1';
const chrome = process.env.CHROME ?? '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const credential = 'Email or password is incorrect.';
const password = 'correct horse';

const fixture = {
  users: [{ id: 'u_ada', email: 'ada@example.com', password, display_name: 'Ada' }],
  restaurants: [
    {
      id: 'r_anker',
      name: 'Zum Anker',
      timezone: 'Europe/Berlin',
      slot_minutes: 30,
      reservation_duration_minutes: 90,
      cancellation_cutoff_minutes: 120,
      opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
      tables: [
        { id: 't_1', label: '1', capacity: 2 },
        { id: 't_2', label: '2', capacity: 4 },
      ],
    },
  ],
  reservations: [],
};

const report = {
  base,
  offOrigin: [],
  pageErrors: [],
  checks: {},
};

function fail(message) {
  throw new Error(message);
}

function pathname(url) {
  return new URL(url).pathname;
}

async function reset() {
  const response = await fetch(`${base}/_test/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify(fixture),
  });
  if (response.status !== 204) fail(`reset status ${response.status}`);
}

async function waitHealth() {
  const started = Date.now();
  let last = 'none';
  while (Date.now() - started < 20000) {
    try {
      const response = await fetch(`${base}/health`);
      last = String(response.status);
      if (response.status === 200) {
        const body = await response.json();
        if (body.status !== 'ok') fail(`health body ${JSON.stringify(body)}`);
        return;
      }
    } catch (error) {
      last = error instanceof Error ? error.name : 'error';
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  fail(`health not ready (${last})`);
}

function watch(page) {
  page.on('request', (request) => {
    const url = request.url();
    let origin = '';
    try {
      origin = new URL(url).origin;
    } catch {
      origin = 'bad-url';
    }
    if (origin !== new URL(base).origin && !url.startsWith('data:')) report.offOrigin.push(url);
  });
  page.on('pageerror', (error) => report.pageErrors.push(String(error)));
}

async function shot(page, name) {
  const dir = join(evidence, 'screenshots');
  await mkdir(dir, { recursive: true });
  await page.screenshot({ path: join(dir, `${name}.png`), fullPage: true });
  console.log(`screenshot ${name}`);
}

async function noHorizontalScroll(page, label) {
  const box = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    client: document.documentElement.clientWidth,
  }));
  if (box.scroll > box.client + 1) fail(`${label} horizontal scroll ${box.scroll} > ${box.client}`);
}

async function openLogin(browser, width, theme) {
  const context = await browser.newContext({
    viewport: { width, height: width < 700 ? 900 : 1000 },
    deviceScaleFactor: 1,
  });
  await context.addInitScript((mode) => {
    localStorage.setItem('chaaya-theme', mode);
  }, theme);
  const page = await context.newPage();
  watch(page);
  await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' });
  await page.locator('[data-testid="login-email"]').waitFor();
  return { context, page };
}

async function readLogin(page, email, secret) {
  const pending = page.waitForResponse((response) => pathname(response.url()) === '/auth/login', { timeout: 10000 });
  await page.locator('[data-testid="login-email"]').fill(email);
  await page.locator('[data-testid="login-password"]').fill(secret);
  await page.locator('[data-testid="login-submit"]').click();
  const response = await pending;
  const body = await response.json();
  await page.locator('[data-testid="auth-error"]').waitFor();
  const text = (await page.locator('[data-testid="auth-error"]').innerText()).replace(/\s+/g, ' ').trim();
  return {
    status: response.status(),
    code: body.error?.code ?? null,
    apiMessage: body.error?.message ?? null,
    ui: text,
  };
}

function expectCredential(result, label) {
  if (result.status !== 401 || result.code !== 'unauthenticated') {
    fail(`${label} api ${result.status} ${result.code}`);
  }
  if (result.apiMessage !== 'missing or invalid bearer token') {
    fail(`${label} api message changed`);
  }
  if (result.ui !== credential) fail(`${label} ui ${result.ui}`);
  const lower = result.ui.toLowerCase();
  if (lower.includes('bearer') || lower.includes('token')) fail(`${label} jargon ${result.ui}`);
}

async function main() {
  await mkdir(evidence, { recursive: true });
  await waitHealth();
  await reset();
  const browser = await chromium.launch({
    executablePath: chrome,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });
  try {
    const shots = [];
    for (const width of [375, 1280]) {
      for (const theme of ['light', 'dark']) {
        const { context, page } = await openLogin(browser, width, theme);
        const wrong = await readLogin(page, 'ada@example.com', 'wrong password');
        expectCredential(wrong, `${width}-${theme} wrong password`);
        await shot(page, `${width}-${theme}-wrong-password`);
        await noHorizontalScroll(page, `${width}-${theme} wrong password`);
        const unknown = await readLogin(page, 'nobody@example.com', password);
        expectCredential(unknown, `${width}-${theme} unknown email`);
        if (unknown.ui !== wrong.ui) fail(`${width}-${theme} account leak`);
        await shot(page, `${width}-${theme}-unknown-email`);
        await noHorizontalScroll(page, `${width}-${theme} unknown email`);
        shots.push(`${width}-${theme}-wrong-password`, `${width}-${theme}-unknown-email`);
        report.checks[`${width}-${theme}`] = {
          wrong: { status: wrong.status, code: wrong.code, apiMessage: wrong.apiMessage, ui: wrong.ui },
          unknown: { status: unknown.status, code: unknown.code, apiMessage: unknown.apiMessage, ui: unknown.ui },
        };
        await context.close();
      }
    }
    report.checks.credentialShots = shots;

    const successContext = await openLogin(browser, 1280, 'light');
    const pending = successContext.page.waitForResponse(
      (response) => pathname(response.url()) === '/auth/login',
      { timeout: 10000 },
    );
    await successContext.page.locator('[data-testid="login-email"]').fill('ada@example.com');
    await successContext.page.locator('[data-testid="login-password"]').fill(password);
    await successContext.page.locator('[data-testid="login-submit"]').click();
    const signed = await pending;
    const signedBody = await signed.json();
    if (signed.status() !== 200 || signedBody.display_name !== 'Ada' || typeof signedBody.token !== 'string') {
      fail(`valid login status ${signed.status()}`);
    }
    if (signedBody.token.length < 8) fail('valid login token missing');
    await successContext.page.locator('[data-testid="current-user"]').waitFor();
    const who = await successContext.page.locator('[data-testid="current-user"]').innerText();
    if (!who.includes('Ada')) fail(`current user ${who}`);
    if ((await successContext.page.locator('[data-testid="auth-error"]').count()) !== 0) {
      fail('valid login showed auth-error');
    }
    await shot(successContext.page, '1280-light-signed-in');
    await noHorizontalScroll(successContext.page, '1280 signed in');
    await successContext.context.close();

    const phone = await openLogin(browser, 375, 'dark');
    const phonePending = phone.page.waitForResponse(
      (response) => pathname(response.url()) === '/auth/login',
      { timeout: 10000 },
    );
    await phone.page.locator('[data-testid="login-email"]').fill('ada@example.com');
    await phone.page.locator('[data-testid="login-password"]').fill(password);
    await phone.page.locator('[data-testid="login-submit"]').click();
    const phoneSigned = await phonePending;
    if (phoneSigned.status() !== 200) fail(`phone login ${phoneSigned.status()}`);
    await phone.page.locator('[data-testid="current-user"]').waitFor();
    const phoneWho = await phone.page.locator('[data-testid="current-user"]').innerText();
    if (!phoneWho.includes('Ada')) fail(`phone current user ${phoneWho}`);
    if ((await phone.page.locator('[data-testid="auth-error"]').count()) !== 0) fail('phone login showed auth-error');
    await shot(phone.page, '375-dark-signed-in');
    await noHorizontalScroll(phone.page, '375 signed in');
    await phone.context.close();

    const other = await openLogin(browser, 1280, 'dark');
    const shortPending = other.page.waitForResponse((response) => pathname(response.url()) === '/auth/signup', {
      timeout: 10000,
    });
    await other.page.goto(`${base}/signup`, { waitUntil: 'domcontentloaded' });
    await other.page.locator('[data-testid="signup-email"]').fill('ada@example.com');
    await other.page.locator('[data-testid="signup-password"]').fill('short');
    await other.page.locator('[data-testid="signup-display-name"]').fill('Ada');
    await other.page.locator('[data-testid="signup-submit"]').click();
    const short = await shortPending;
    const shortBody = await short.json();
    const shortText = (await other.page.locator('[data-testid="auth-error"]').innerText()).replace(/\s+/g, ' ').trim();
    if (short.status() !== 422 || shortBody.error?.code !== 'validation_failed') {
      fail(`short password api ${short.status()} ${shortBody.error?.code ?? ''}`);
    }
    if (shortText === credential || shortText.toLowerCase().includes('bearer') || shortText.toLowerCase().includes('token')) {
      fail(`short password ui ${shortText}`);
    }
    if (!shortText.toLowerCase().includes('password')) fail(`short password copy ${shortText}`);
    await shot(other.page, '1280-dark-signup-refusal');
    await noHorizontalScroll(other.page, 'signup refusal');
    report.checks.otherRefusal = { status: short.status(), code: shortBody.error.code, ui: shortText };
    await other.context.close();

    const lost = await openLogin(browser, 375, 'light');
    await lost.page.route('**/auth/login', (route) => route.abort('failed'));
    await lost.page.locator('[data-testid="login-email"]').fill('ada@example.com');
    await lost.page.locator('[data-testid="login-password"]').fill(password);
    await lost.page.locator('[data-testid="login-submit"]').click();
    await lost.page.locator('[data-testid="auth-error"]').waitFor();
    const lostText = (await lost.page.locator('[data-testid="auth-error"]').innerText()).replace(/\s+/g, ' ').trim();
    if (lostText === credential || lostText.toLowerCase().includes('password') || lostText.toLowerCase().includes('bearer')) {
      fail(`transport ui ${lostText}`);
    }
    if (!lostText.toLowerCase().includes('could not reach')) fail(`transport copy ${lostText}`);
    await shot(lost.page, '375-light-transport');
    await noHorizontalScroll(lost.page, 'transport');
    report.checks.transport = { ui: lostText };
    await lost.context.close();

    report.checks.credential = credential;
    report.checks.offOrigin = report.offOrigin.length;
    report.checks.pageErrors = report.pageErrors.length;
    if (report.offOrigin.length > 0) fail(`off origin ${report.offOrigin.slice(0, 4).join(' | ')}`);
    if (report.pageErrors.length > 0) fail(`page errors ${report.pageErrors.join(' | ')}`);
    await writeFile(join(evidence, 'live-probe.json'), `${JSON.stringify(report, null, 2)}\n`);
    console.log('probe ok');
  } finally {
    await browser.close();
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error instanceof Error ? error.stack ?? error.message : String(error));
    process.exit(1);
  });
