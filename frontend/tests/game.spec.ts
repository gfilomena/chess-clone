/**
 * E2E: Multi-user chess game flow
 *
 * Simulates two players (Alice and Bob) in separate browser contexts:
 *  1. Register / login via API
 *  2. Matchmaking  — join queue, get paired
 *  3. Navigate to game page (WebSocket connects, board hydrates)
 *  4. Play moves via click-to-move (wait for status badge to confirm turn)
 *  5. Resign → game-over overlay visible on both sides
 *  6. Analysis page — board + action buttons + review engine
 *
 * Prerequisites:
 *   - Backend running at http://localhost:8080  (or set BACKEND_URL env)
 *   - Frontend dev server at http://localhost:5174 (auto-started by Playwright)
 *
 * Run:
 *   npx playwright test game.spec.ts --project chromium
 */

import {
  test, expect,
  type Browser, type BrowserContext, type Page,
} from '@playwright/test';

// ── Constants ──────────────────────────────────────────────────────────────────

const BACKEND = process.env.BACKEND_URL ?? 'http://localhost:8080';

/**
 * Unique suffix per test run — used in usernames/emails to avoid DB conflicts.
 * Derived once so it stays stable for the entire run.
 */
const RUN_MS  = Date.now();
const RUN     = RUN_MS.toString(36);

/**
 * Time control unique to this run (7001–7999 s).
 * Prevents Alice/Bob from being matched with stale queue entries left by
 * previous failed test runs (the matchmaker uses in-memory state that
 * persists until the backend restarts).
 */
const TEST_TC = 7001 + (RUN_MS % 999);

// ── Types ──────────────────────────────────────────────────────────────────────

interface Player {
  ctx:      BrowserContext;
  page:     Page;
  username: string;
  email:    string;
}

/**
 * All backend responses are wrapped: { success: boolean, data: T }
 * Helper to unwrap the data field after a ctx.request call.
 */
async function apiData<T>(
  res: Awaited<ReturnType<BrowserContext['request']['get']>>,
): Promise<T> {
  const body = (await res.json()) as { success: boolean; data: T };
  return body.data;
}

// ── Player factory ─────────────────────────────────────────────────────────────

/**
 * Create a browser context, register a new user, and return a Player.
 * The auth_token cookie is stored in the context cookie jar so all page
 * navigations and ctx.request calls are authenticated.
 */
async function createPlayer(
  browser:  Browser,
  username: string,
  email:    string,
): Promise<Player> {
  const ctx  = await browser.newContext();
  const page = await ctx.newPage();

  // Suppress cookie / language banners before any navigation
  await page.addInitScript(() => {
    localStorage.setItem('cookie_consent', 'essential');
    localStorage.setItem('lang', 'en');
  });

  // Register → server sets auth_token cookie for localhost domain
  const res = await ctx.request.post(`${BACKEND}/api/auth/register`, {
    data: { username, email, password: 'Pw!23456' },
  });

  // 409 Conflict = user already exists from a previous run → login instead
  if (res.status() === 409) {
    await ctx.request.post(`${BACKEND}/api/auth/login`, {
      data: { email, password: 'Pw!23456' },
    });
  }

  return { ctx, page, username, email };
}

// ── API helpers ────────────────────────────────────────────────────────────────

/**
 * POST /api/matchmaking/join.
 * Uses TEST_TC (unique per run) so these players only match with each other,
 * not with stale entries from previous test runs.
 */
async function joinQueue(
  player: Player,
  tc   = TEST_TC,
  inc  = 0,
  type = 'rapid',
): Promise<void> {
  const res = await player.ctx.request.post(`${BACKEND}/api/matchmaking/join`, {
    data: { time_control: tc, increment: inc, game_type: type },
  });
  expect(res.ok(), `${player.username}: join queue`).toBe(true);
}

/**
 * Poll GET /api/games/active until a game_id appears (max 15 s).
 * Response envelope: { success, data: { game_id: string } }
 */
async function waitForActiveGame(
  player: Player,
  maxMs = 15_000,
): Promise<string> {
  const deadline = Date.now() + maxMs;
  while (Date.now() < deadline) {
    const res = await player.ctx.request.get(`${BACKEND}/api/games/active`);
    if (res.ok()) {
      const data = await apiData<{ game_id?: string } | null>(res);
      if (data?.game_id) return data.game_id;
    }
    await player.page.waitForTimeout(400);
  }
  throw new Error(`${player.username}: timeout waiting for active game`);
}

/**
 * Poll GET /api/games/{id} until pgn contains the expected SAN token.
 * Response envelope: { success, data: { pgn, white_id, black_id, … } }
 */
async function waitForPGN(
  player:   Player,
  gameId:   string,
  contains: string,
  maxMs = 8_000,
): Promise<void> {
  const deadline = Date.now() + maxMs;
  while (Date.now() < deadline) {
    const res = await player.ctx.request.get(`${BACKEND}/api/games/${gameId}`);
    if (res.ok()) {
      const data = await apiData<{ pgn: string }>(res);
      if ((data.pgn ?? '').includes(contains)) return;
    }
    await player.page.waitForTimeout(300);
  }
  throw new Error(`Timeout: game ${gameId} PGN never contained "${contains}"`);
}

/** GET /api/auth/me → { id, username, … } */
async function getMe(
  player: Player,
): Promise<{ id: string; username: string }> {
  const res = await player.ctx.request.get(`${BACKEND}/api/auth/me`);
  return apiData<{ id: string; username: string }>(res);
}

/** GET /api/games/{id} → { white_id, black_id, pgn, … } */
async function getGame(
  player: Player,
  gameId: string,
): Promise<{ white_id: string; black_id: string; pgn: string }> {
  const res = await player.ctx.request.get(`${BACKEND}/api/games/${gameId}`);
  return apiData<{ white_id: string; black_id: string; pgn: string }>(res);
}

// ── Board / UI helpers ─────────────────────────────────────────────────────────

/**
 * Wait until the Board component has rendered squares.
 * `[data-sq]` elements appear only after the WS game_start message is received.
 */
async function waitForBoard(page: Page): Promise<void> {
  await expect(page.locator('[data-sq="e2"]')).toBeVisible({ timeout: 15_000 });
}

/**
 * Wait until the .status-badge shows "Your turn" / "Tocca a te".
 * Driven by $gameState via WS — confirms move_made was processed.
 */
async function waitForMyTurn(player: Player): Promise<void> {
  await expect(
    player.page.locator('.status-badge'),
  ).toContainText(/your turn|tocca a te|tu turno/i, { timeout: 10_000 });
}

/** Click-to-move: select piece square, then click destination. */
async function clickMove(page: Page, from: string, to: string): Promise<void> {
  await page.locator(`[data-sq="${from}"]`).click();
  await page.locator(`[data-sq="${to}"]`).click();
}

// ═══════════════════════════════════════════════════════════════════════════════
// Suite A — Matchmaking + gameplay + resign + analysis
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Multi-user: matchmaking game flow', () => {
  test.describe.configure({ mode: 'serial' });

  // Only run on chromium to avoid duplicate DB records per browser project.
  // Usage: npx playwright test game.spec.ts --project chromium
  test.skip(
    ({ browserName }) => browserName !== 'chromium',
    'Multi-user suite: chromium only',
  );

  let alice:       Player;
  let bob:         Player;
  let gameId:      string;
  let whitePlayer: Player;
  let blackPlayer: Player;

  test.beforeAll(async ({ browser }) => {
    alice = await createPlayer(browser, `alice_${RUN}`, `alice_${RUN}@pw-test.com`);
    bob   = await createPlayer(browser, `bob_${RUN}`,   `bob_${RUN}@pw-test.com`);

    await Promise.all([
      alice.ctx.request.post(`${BACKEND}/api/users/heartbeat`),
      bob.ctx.request.post(`${BACKEND}/api/users/heartbeat`),
    ]);
  });

  test.afterAll(async () => {
    // Leave queue to avoid polluting future runs
    await Promise.all([
      alice?.ctx.request.delete(`${BACKEND}/api/matchmaking/leave`).catch(() => {}),
      bob?.ctx.request.delete(`${BACKEND}/api/matchmaking/leave`).catch(() => {}),
    ]);
    await alice?.ctx.close().catch(() => {});
    await bob?.ctx.close().catch(() => {});
  });

  // ── 1. Matchmaking ───────────────────────────────────────────────────────

  test('1 — matchmaking: both join queue and are paired to the same game', async () => {
    await joinQueue(alice); // uses TEST_TC
    await joinQueue(bob);   // same TC → only they can match each other

    // Matchmaker runs every 2 s — wait up to 15 s
    gameId          = await waitForActiveGame(alice);
    const bobGameId = await waitForActiveGame(bob);

    expect(gameId,    'game ID non vuoto').toBeTruthy();
    expect(bobGameId, 'Alice e Bob nello stesso game').toBe(gameId);
  });

  // ── 2. Open game page ────────────────────────────────────────────────────

  test('2 — both players open /game/{id} and board hydrates via WS', async () => {
    await Promise.all([
      alice.page.goto(`/game/${gameId}`),
      bob.page.goto(`/game/${gameId}`),
    ]);

    await Promise.all([
      waitForBoard(alice.page),
      waitForBoard(bob.page),
    ]);

    const [gameData, aliceMe] = await Promise.all([
      getGame(alice, gameId),
      getMe(alice),
    ]);

    whitePlayer = gameData.white_id === aliceMe.id ? alice : bob;
    blackPlayer = gameData.white_id === aliceMe.id ? bob   : alice;
  });

  // ── 3. Play moves ────────────────────────────────────────────────────────

  test('3 — white plays e4 (e2→e4)', async () => {
    await waitForMyTurn(whitePlayer);
    await clickMove(whitePlayer.page, 'e2', 'e4');
    await waitForPGN(alice, gameId, 'e4');
  });

  test('4 — black plays e5 (e7→e5)', async () => {
    await waitForMyTurn(blackPlayer);
    await clickMove(blackPlayer.page, 'e7', 'e5');
    await waitForPGN(alice, gameId, 'e5');
  });

  test('5 — white plays Nf3 (g1→f3)', async () => {
    await waitForMyTurn(whitePlayer);
    await clickMove(whitePlayer.page, 'g1', 'f3');
    await waitForPGN(alice, gameId, 'Nf3');
  });

  // ── 4. Resign ────────────────────────────────────────────────────────────

  test('6 — black resigns; game-over overlay appears on both pages', async () => {
    // Intercept the native browser confirm() dialog and accept it
    blackPlayer.page.once('dialog', (dialog) => dialog.accept());

    const resignBtn = blackPlayer.page.getByRole('button', {
      name: /resign|abbandona|rendirse/i,
    });
    await expect(resignBtn).toBeVisible({ timeout: 5_000 });
    await resignBtn.click();

    await Promise.all([
      expect(whitePlayer.page.locator('.overlay.finished')).toBeVisible({
        timeout: 10_000,
      }),
      expect(blackPlayer.page.locator('.overlay.finished')).toBeVisible({
        timeout: 10_000,
      }),
    ]);
  });

  // ── 5. Analysis page ─────────────────────────────────────────────────────

  test('7 — navigate to /analysis/{id}; board renders', async () => {
    await alice.page.goto(`/analysis/${gameId}`);
    await expect(alice.page.locator('.board-wrap')).toBeVisible({
      timeout: 15_000,
    });
  });

  test('8 — analysis: primary + ghost action buttons are visible', async () => {
    await expect(alice.page.locator('.action-primary')).toBeVisible();
    await expect(alice.page.locator('.action-ghost--accent')).toBeVisible();
    await expect(
      alice.page.locator('.action-ghost').filter({ hasText: 'PGN' }),
    ).toBeVisible();
  });

  test('9 — analysis: clicking review triggers engine analysis', async () => {
    await alice.page.locator('.action-ghost--accent').click();
    await expect(
      alice.page.locator('.review-progress, .engine-info').first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
// Suite B — Invitation flow (Alice invites Bob; Bob sees toast and accepts)
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Multi-user: invitation flow', () => {
  test.describe.configure({ mode: 'serial' });
  test.skip(
    ({ browserName }) => browserName !== 'chromium',
    'Invitation suite: chromium only',
  );

  // Different suffix from Suite A to avoid user/game collisions
  const INV_MS  = Date.now() + 1; // +1 to differ from RUN_MS if run same instant
  const INV_RUN = INV_MS.toString(36) + 'i';

  let alice: Player;
  let bob:   Player;
  let gameId: string;

  test.beforeAll(async ({ browser }) => {
    alice = await createPlayer(
      browser,
      `alice_${INV_RUN}`,
      `alice_${INV_RUN}@pw-test.com`,
    );
    bob = await createPlayer(
      browser,
      `bob_${INV_RUN}`,
      `bob_${INV_RUN}@pw-test.com`,
    );

    await Promise.all([
      alice.ctx.request.post(`${BACKEND}/api/users/heartbeat`),
      bob.ctx.request.post(`${BACKEND}/api/users/heartbeat`),
    ]);
  });

  test.afterAll(async () => {
    await alice?.ctx.close().catch(() => {});
    await bob?.ctx.close().catch(() => {});
  });

  test('10 — Bob opens /play (SSE invitation stream subscribes via onMount)', async () => {
    await bob.page.goto('/play');
    // The SSE stream opens on mount; give it a moment to connect
    await bob.page.waitForTimeout(1_200);
    await expect(bob.page.locator('body')).toBeAttached();
  });

  test('11 — Alice sends a blitz-5min invite to Bob via API', async () => {
    const bobMe = await getMe(bob);

    const res = await alice.ctx.request.post(`${BACKEND}/api/invitations`, {
      data: {
        to_user_id:   bobMe.id,
        time_control: 300,   // 5 min blitz
        increment:    0,
      },
    });
    expect(res.ok(), 'invite POST should succeed').toBe(true);
  });

  test('12 — Bob sees the InviteToast and accepts', async () => {
    // InviteToast renders when $pendingInvite store is set by the SSE event
    const toast = bob.page.locator('.invite-toast');
    await expect(toast).toBeVisible({ timeout: 10_000 });

    // The accept button inside the toast is hardcoded "Accetta" (Italian)
    const acceptBtn = bob.page.getByRole('button', {
      name: /accetta|accept/i,
    });
    await expect(acceptBtn).toBeVisible();
    await acceptBtn.click();
  });

  test('13 — both players land on the game page after acceptance', async () => {
    // After accepting, goto is called → Bob's page navigates to /game/{id}
    await expect(bob.page).toHaveURL(/\/game\//, { timeout: 10_000 });
    const url = bob.page.url();
    gameId = url.split('/game/')[1];

    expect(gameId).toBeTruthy();

    // Alice may need to navigate manually (she sent the invite, no auto-redirect)
    if (!alice.page.url().includes('/game/')) {
      await alice.page.goto(`/game/${gameId}`);
    }

    await Promise.all([
      waitForBoard(alice.page),
      waitForBoard(bob.page),
    ]);
  });
});
