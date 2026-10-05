import { spawn } from 'node:child_process';
import { mkdir, rename } from 'node:fs/promises';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from 'playwright';

const web = '/home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-grok/stage-1/web';
const evid = '/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S1-G';
const shotDir = `${evid}/screenshots-final`;
const videoDir = `${evid}/video-final`;
const chrome = '/home/agent/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome';
const base = 'http://127.0.0.1:4173';

const pages = [
  ['results', '/'],
  ['selected', '/?demo=selected'],
  ['loading', '/?demo=loading'],
  ['empty', '/?demo=empty'],
  ['refused', '/?demo=refused'],
  ['uncertain', '/?demo=uncertain'],
  ['confirmed', '/?demo=confirmed'],
  ['signup', '/signup'],
  ['signup-error', '/signup?demo=auth'],
  ['login', '/login'],
  ['login-error', '/login?demo=auth'],
  ['lookup', '/lookup'],
  ['lookup-missing', '/lookup?demo=missing'],
];

const ready = {
  results: 'availability-grid',
  selected: 'booking-form',
  loading: 'loading-state',
  empty: 'no-slots',
  refused: 'booking-error',
  uncertain: 'booking-uncertain',
  confirmed: 'confirmation',
  signup: 'signup-submit',
  'signup-error': 'auth-error',
  login: 'login-submit',
  'login-error': 'auth-error',
  lookup: 'lookup-submit',
  'lookup-missing': 'reservation-error',
};

function wire(page, problems) {
  page.on('pageerror', (error) => problems.push(`pageerror ${error}`));
  page.on('console', (message) => {
    if (message.type() === 'error') problems.push(`console ${message.text()}`);
  });
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (url.protocol === 'data:' || url.protocol === 'blob:') return;
    if (url.hostname !== '127.0.0.1') problems.push(`external ${request.url()}`);
  });
}

async function assertLayout(page, label) {
  const box = await page.evaluate(() => {
    const width = document.documentElement.clientWidth;
    const bad = [];
    for (const el of document.querySelectorAll('body *')) {
      const rect = el.getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) continue;
      if (rect.left < -1 || rect.right > width + 1) {
        bad.push({
          tag: el.tagName,
          id: el.id,
          testid: el.getAttribute('data-testid'),
          className: String(el.className).slice(0, 80),
          left: Math.round(rect.left),
          right: Math.round(rect.right),
          width: Math.round(rect.width),
        });
      }
    }
    return {
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: width,
      bad: bad.slice(0, 12),
    };
  });
  if (box.scrollWidth > box.clientWidth + 1 || box.bad.length > 0) {
    throw new Error(`${label} overflow ${JSON.stringify(box)}`);
  }
}

const floorPages = new Set(['results', 'selected', 'refused', 'uncertain', 'confirmed']);

async function assertFloor(page, label) {
  const report = await page.evaluate(() => {
    const scenes = [...document.querySelectorAll('[data-testid="floor-plan"] svg.room-scene')];
    const scene = scenes[0];
    const plan1 = document.querySelector('[data-testid="plan-t_1"]');
    const plan2 = document.querySelector('[data-testid="plan-t_2"]');
    return {
      scenes: scenes.length,
      cards: document.querySelectorAll('.table-card').length,
      wall: Boolean(scene?.querySelector('.room-wall')),
      aisle: Boolean(scene?.querySelector('.room-aisle')),
      owns: Boolean(scene && plan1 && plan2 && scene.contains(plan1) && scene.contains(plan2)),
      seats1: plan1 ? plan1.querySelectorAll('.seat').length : -1,
      seats2: plan2 ? plan2.querySelectorAll('.seat').length : -1,
      svg: scene?.namespaceURI ?? '',
    };
  });
  if (
    report.scenes !== 1 ||
    report.cards !== 0 ||
    !report.wall ||
    !report.aisle ||
    !report.owns ||
    report.seats1 !== 2 ||
    report.seats2 !== 4 ||
    report.svg !== 'http://www.w3.org/2000/svg'
  ) {
    throw new Error(`${label} floor ${JSON.stringify(report)}`);
  }
}

async function assertPreview(page, path) {
  const route = new URL(path, base);
  if (route.pathname !== '/') return;
  const root = page.locator('[data-preview="sample"]');
  const restaurant = await root.getAttribute('data-preview-restaurant');
  const date = await root.getAttribute('data-preview-date');
  const party = await root.getAttribute('data-preview-party');
  if (restaurant !== 'r_anker' || date !== '2026-09-24' || party !== '2') {
    throw new Error(`preview fixture ${restaurant} ${date} ${party} on ${path}`);
  }
  const text = await page.locator('body').innerText();
  if (!text.includes('Zum Anker') || !text.includes('Thursday 24 September 2026')) {
    throw new Error(`human date missing on ${path}`);
  }
}

async function openPage(page, path) {
  await page.goto(base + path, { waitUntil: 'networkidle' });
  await page.waitForTimeout(650);
}

const server = spawn('npm', ['run', 'preview'], {
  cwd: web,
  env: { ...process.env, FORCE_COLOR: '0', NO_COLOR: '1' },
  stdio: ['ignore', 'pipe', 'pipe'],
  detached: true,
});
let serverLog = '';
server.stdout.on('data', (chunk) => {
  serverLog += chunk.toString();
});
server.stderr.on('data', (chunk) => {
  serverLog += chunk.toString();
});

async function stopServer() {
  if (server.exitCode !== null) return;
  const pid = server.pid;
  try {
    if (pid) process.kill(-pid, 'SIGTERM');
  } catch {
    server.kill('SIGTERM');
  }
  await Promise.race([
    new Promise((resolve) => server.once('exit', resolve)),
    delay(3000),
  ]);
  if (server.exitCode === null && pid) {
    try {
      process.kill(-pid, 'SIGKILL');
    } catch {
      server.kill('SIGKILL');
    }
  }
}

let exitCode = 0;
try {
  const deadline = Date.now() + 30000;
  let readyServer = false;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(base + '/');
      if (response.ok) {
        readyServer = true;
        break;
      }
    } catch {
      // preview is still binding the port
    }
    await delay(200);
  }
  if (!readyServer) throw new Error(`preview did not start\n${serverLog}`);

  await mkdir(shotDir, { recursive: true });
  await mkdir(videoDir, { recursive: true });

  const browser = await chromium.launch({
    executablePath: chrome,
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });

  try {
    for (const width of [375, 1280]) {
      for (const theme of ['light', 'dark']) {
        const problems = [];
        const context = await browser.newContext({
          viewport: { width, height: width === 375 ? 812 : 900 },
          colorScheme: theme,
        });
        await context.addInitScript((mode) => {
          localStorage.setItem('chaaya-theme', mode);
        }, theme);
        const page = await context.newPage();
        wire(page, problems);
        for (const [name, path] of pages) {
          const label = `${name}-${theme}-${width}`;
          await openPage(page, path);
          await page.locator(`[data-testid="${ready[name]}"]`).waitFor();
          await assertPreview(page, path);
          if (name === 'confirmed') {
            if (await page.locator('[data-testid="confirmation-reference"]').count()) {
              throw new Error(`${label} invented a confirmation reference`);
            }
          }
          if (name === 'uncertain' && (await page.locator('[data-testid="booking-error"]').count())) {
            throw new Error(`${label} showed booking-error beside uncertain`);
          }
          if (name === 'refused' && (await page.locator('[data-testid="confirmation"]').count())) {
            throw new Error(`${label} confirmed a refusal`);
          }
          if (floorPages.has(name)) await assertFloor(page, label);
          await assertLayout(page, label);
          await page.screenshot({ path: `${shotDir}/${label}.png`, fullPage: true });
          console.log(`shot ${label}`);
          if (problems.length > 0) throw new Error(`${label}\n${problems.join('\n')}`);
        }
        await context.close();
      }
    }

    for (const width of [375, 1280]) {
      for (const scheme of ['light', 'dark']) {
        const problems = [];
        const context = await browser.newContext({
          viewport: { width, height: width === 375 ? 812 : 900 },
          colorScheme: scheme,
        });
        await context.addInitScript(() => {
          localStorage.removeItem('chaaya-theme');
        });
        const page = await context.newPage();
        wire(page, problems);
        const label = `results-system-${scheme}-${width}`;
        await openPage(page, '/');
        await page.locator('[data-testid="theme-system"]').click();
        await page.waitForTimeout(200);
        const mode = await page.locator('[data-testid="theme-system"]').getAttribute('aria-pressed');
        if (mode !== 'true') throw new Error(`${label} system control not pressed`);
        const themeAttr = await page.evaluate(() => document.documentElement.dataset.theme ?? '');
        if (themeAttr !== '') throw new Error(`${label} data-theme=${themeAttr}`);
        await assertPreview(page, '/');
        await assertFloor(page, label);
        await assertLayout(page, label);
        await page.screenshot({ path: `${shotDir}/${label}.png`, fullPage: true });
        console.log(`shot ${label}`);
        if (problems.length > 0) throw new Error(`${label}\n${problems.join('\n')}`);
        await context.close();
      }
    }

    async function record(name, width, options, run) {
      const problems = [];
      const height = width === 375 ? 812 : 900;
      const context = await browser.newContext({
        viewport: { width, height },
        recordVideo: { dir: videoDir, size: { width, height } },
        colorScheme: 'light',
        ...options,
      });
      await context.addInitScript(() => {
        localStorage.setItem('chaaya-theme', 'light');
      });
      const page = await context.newPage();
      wire(page, problems);
      let failed;
      try {
        await run(page);
      } catch (error) {
        failed = error;
      }
      const video = page.video();
      await context.close();
      if (video) await rename(await video.path(), `${videoDir}/${name}.webm`);
      if (failed) throw failed;
      if (problems.length > 0) throw new Error(`${name}\n${problems.join('\n')}`);
      console.log(`video ${name}`);
    }

    for (const width of [375, 1280]) {
      await record(`selection-${width}`, width, {}, async (page) => {
        await openPage(page, '/');
        const first = page.locator('[data-testid="slot-t_1-18:00"]');
        await first.scrollIntoViewIfNeeded();
        await first.click();
        await page.waitForTimeout(700);
        const second = page.locator('[data-testid="slot-t_2-18:30"]');
        await second.scrollIntoViewIfNeeded();
        await second.click();
        await page.waitForTimeout(700);
        const taken = page.locator('[data-testid="slot-t_2-19:00"]');
        await taken.scrollIntoViewIfNeeded();
        await taken.click();
        await page.waitForTimeout(450);
        const selected = await page.locator('[data-testid="slot-t_2-18:30"]').getAttribute('data-selected');
        if (selected !== 'true') throw new Error(`selection-${width} lost the 18:30 choice`);
        const summary = await page.locator('[data-testid="booking-summary"]').innerText();
        if (!summary.includes('Table 2') || !summary.includes('18:30') || !summary.includes('Zum Anker')) {
          throw new Error(`selection-${width} summary ${summary}`);
        }
      });

      await record(`routes-${width}`, width, {}, async (page) => {
        await openPage(page, '/');
        await page.locator('nav a[href="/signup"]').click();
        await page.locator('[data-testid="signup-email"]').waitFor();
        await page.waitForTimeout(500);
        await page.locator('nav a[href="/login"]').click();
        await page.locator('[data-testid="login-email"]').waitFor();
        await page.waitForTimeout(500);
        await page.locator('nav a[href="/lookup"]').click();
        await page.locator('[data-testid="lookup-reference-input"]').waitFor();
        await page.waitForTimeout(500);
        await page.locator('a.brand').click();
        await page.locator('[data-testid="availability-grid"]').waitFor();
        await page.waitForTimeout(400);
      });

      await record(`theme-${width}`, width, {}, async (page) => {
        await openPage(page, '/?demo=selected');
        await page.locator('[data-testid="theme-dark"]').click();
        await page.waitForTimeout(450);
        await page.locator('[data-testid="theme-light"]').click();
        await page.waitForTimeout(450);
        await page.locator('[data-testid="theme-system"]').click();
        await page.waitForTimeout(450);
        const pressed = await page.locator('[data-testid="theme-system"]').getAttribute('aria-pressed');
        if (pressed !== 'true') throw new Error(`theme-${width} system not pressed`);
      });

      await record(`keyboard-plan-${width}`, width, {}, async (page) => {
        await openPage(page, '/');
        const first = page.locator('[data-testid="plan-t_1"]');
        await first.scrollIntoViewIfNeeded();
        await first.focus();
        await page.keyboard.press('Enter');
        const selected = await first.getAttribute('data-selected');
        if (selected !== 'true') throw new Error(`keyboard-plan-${width} did not select table 1`);
        const summary = await page.locator('[data-testid="booking-summary"]').innerText();
        if (!summary.includes('Table 1') || !summary.includes('18:00')) {
          throw new Error(`keyboard-plan-${width} summary ${summary}`);
        }
        const second = page.locator('[data-testid="plan-t_2"]');
        await second.focus();
        await page.keyboard.press('Enter');
        const moved = await second.getAttribute('data-selected');
        if (moved !== 'true') throw new Error(`keyboard-plan-${width} did not select table 2`);
        await assertLayout(page, `keyboard-plan-${width}`);
        await page.waitForTimeout(350);
      });

      await record(`reduced-motion-${width}`, width, { reducedMotion: 'reduce' }, async (page) => {
        await openPage(page, '/');
        const plan = page.locator('[data-testid="plan-t_1"]');
        await plan.scrollIntoViewIfNeeded();
        await plan.focus();
        await page.keyboard.press('Enter');
        const planSelected = await plan.getAttribute('data-selected');
        const cellSelected = await page.locator('[data-testid="slot-t_1-18:00"]').getAttribute('data-selected');
        if (planSelected !== 'true' || cellSelected !== 'true') {
          throw new Error(`reduced-motion-${width} plan=${planSelected} cell=${cellSelected}`);
        }
        const summary = await page.locator('[data-testid="booking-summary"]').innerText();
        if (!summary.includes('Table 1')) throw new Error(`reduced-motion-${width} summary ${summary}`);
        await assertLayout(page, `reduced-motion-${width}`);
      });
    }
  } finally {
    await Promise.race([browser.close(), delay(5000)]);
  }
} catch (error) {
  exitCode = 1;
  console.error(error);
} finally {
  await stopServer();
}
process.exit(exitCode);
