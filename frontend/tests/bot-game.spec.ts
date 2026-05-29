/**
 * bot-game.spec.ts — Guarda una partita completa nel browser
 *
 * Riproduce la "Opera Game" di Paul Morphy (Parigi, 1858).
 * Si aprono due finestre Chromium: una per il Bianco (Morphy), una per il Nero
 * (Duca di Brunswick). Le mosse si alternano automaticamente con una pausa
 * visibile tra una e l'altra.
 *
 * Avvia con:
 *   npx playwright test bot-game.spec.ts --project chromium --headed --workers=1
 *
 * Modifica MOVE_DELAY_MS per accelerare o rallentare.
 */

import {
  test, expect,
  type Browser, type BrowserContext, type Page,
} from '@playwright/test';

// ── Configurazione ─────────────────────────────────────────────────────────────

const BACKEND       = process.env.BACKEND_URL ?? 'http://localhost:8080';
const MOVE_DELAY_MS = 1_400; // pausa (ms) prima di ogni mossa — abbassala per andare più veloce

/**
 * L'Opera Game — Morphy vs Duca di Brunswick & Conte Isouard, Parigi 1858.
 * Il Bianco vince per scaccomatto alla mossa 17 con Td8#.
 *
 * Ogni coppia [from, to] è una semiossa:
 *   indice pari   → mossa del Bianco
 *   indice dispari → mossa del Nero
 */
const OPERA_GAME: ReadonlyArray<readonly [string, string]> = [
  ['e2','e4'], ['e7','e5'],   // 1.  e4      e5
  ['g1','f3'], ['d7','d6'],   // 2.  Cf3     d6
  ['d2','d4'], ['c8','g4'],   // 3.  d4      Ag4
  ['d4','e5'], ['g4','f3'],   // 4.  dxe5    Axf3
  ['d1','f3'], ['d6','e5'],   // 5.  Dxf3    dxe5
  ['f1','c4'], ['g8','f6'],   // 6.  Ac4     Cf6
  ['f3','b3'], ['d8','e7'],   // 7.  Db3     De7
  ['b1','c3'], ['c7','c6'],   // 8.  Cc3     c6
  ['c1','g5'], ['b7','b5'],   // 9.  Ag5     b5
  ['c3','b5'], ['c6','b5'],   // 10. Cxb5    cxb5
  ['c4','b5'], ['b8','d7'],   // 11. Axb5+   Cbd7
  ['e1','c1'], ['a8','d8'],   // 12. O-O-O   Td8
  ['d1','d7'], ['d8','d7'],   // 13. Txd7    Txd7
  ['h1','d1'], ['e7','e6'],   // 14. Td1     De6
  ['b5','d7'], ['f6','d7'],   // 15. Axd7+   Cxd7
  ['b3','b8'], ['d7','b8'],   // 16. Db8+!   Cxb8
  ['d1','d8'],                 // 17. Td8#   (scaccomatto)
] as const;

// ── Helpers ────────────────────────────────────────────────────────────────────

interface Player {
  ctx:      BrowserContext;
  page:     Page;
  username: string;
}

async function apiData<T>(res: Awaited<ReturnType<BrowserContext['request']['get']>>): Promise<T> {
  const body = (await res.json()) as { success: boolean; data: T };
  return body.data;
}

async function createPlayer(browser: Browser, username: string, email: string): Promise<Player> {
  const ctx  = await browser.newContext();
  const page = await ctx.newPage();

  await page.addInitScript(() => {
    localStorage.setItem('cookie_consent', 'essential');
    localStorage.setItem('lang', 'en');
  });

  const res = await ctx.request.post(`${BACKEND}/api/auth/register`, {
    data: { username, email, password: 'Pw!23456' },
  });
  if (res.status() === 409) {
    await ctx.request.post(`${BACKEND}/api/auth/login`, {
      data: { email, password: 'Pw!23456' },
    });
  }

  return { ctx, page, username };
}

async function joinQueue(player: Player, tc: number): Promise<void> {
  const res = await player.ctx.request.post(`${BACKEND}/api/matchmaking/join`, {
    data: { time_control: tc, increment: 0, game_type: 'rapid' },
  });
  expect(res.ok(), `${player.username}: join queue`).toBe(true);
}

async function waitForGame(player: Player, maxMs = 15_000): Promise<string> {
  const deadline = Date.now() + maxMs;
  while (Date.now() < deadline) {
    const res = await player.ctx.request.get(`${BACKEND}/api/games/active`);
    if (res.ok()) {
      const data = await apiData<{ game_id?: string } | null>(res);
      if (data?.game_id) return data.game_id;
    }
    await player.page.waitForTimeout(400);
  }
  throw new Error(`${player.username}: timeout in attesa della partita`);
}

// ── Suite ──────────────────────────────────────────────────────────────────────

test.describe('Bot game — Opera Game (Morphy 1858)', () => {
  test.describe.configure({ mode: 'serial' });
  test.skip(
    ({ browserName }) => !['chromium', 'webkit'].includes(browserName),
    'Bot game: chromium + webkit (mobile)',
  );

  const RUN    = Date.now().toString(36);
  const BOT_TC = 4001 + (Date.now() % 499); // TC unico per evitare match con code stantie

  let p1: Player;   // uno dei due — il server decide il colore
  let p2: Player;
  let gameId: string;
  let whitePage: Page;
  let blackPage: Page;

  test.beforeAll(async ({ browser }) => {
    p1 = await createPlayer(browser, `morphy_${RUN}`, `morphy_${RUN}@bot.test`);
    p2 = await createPlayer(browser, `duke_${RUN}`,   `duke_${RUN}@bot.test`);
    await Promise.all([
      p1.ctx.request.post(`${BACKEND}/api/users/heartbeat`),
      p2.ctx.request.post(`${BACKEND}/api/users/heartbeat`),
    ]);
  });

  test.afterAll(async () => {
    await p1?.ctx.close().catch(() => {});
    await p2?.ctx.close().catch(() => {});
  });

  // ── 1. Matchmaking ────────────────────────────────────────────────────────

  test('1 — matchmaking: i due bot si trovano', async () => {
    await joinQueue(p1, BOT_TC);
    await joinQueue(p2, BOT_TC);

    gameId              = await waitForGame(p1);
    const p2GameId      = await waitForGame(p2);

    expect(gameId, 'stessa partita').toBeTruthy();
    expect(p2GameId,  'stessa partita').toBe(gameId);
  });

  // ── 2. Apertura scacchiera ────────────────────────────────────────────────

  test('2 — apri la scacchiera e determina i colori', async () => {
    await Promise.all([
      p1.page.goto(`/game/${gameId}`),
      p2.page.goto(`/game/${gameId}`),
    ]);

    await Promise.all([
      expect(p1.page.locator('[data-sq="e2"]')).toBeVisible({ timeout: 15_000 }),
      expect(p2.page.locator('[data-sq="e2"]')).toBeVisible({ timeout: 15_000 }),
    ]);

    // Scopri quale player ha il Bianco
    const [gameData, p1Me] = await Promise.all([
      p1.ctx.request.get(`${BACKEND}/api/games/${gameId}`)
        .then(r => apiData<{ white_id: string }>(r)),
      p1.ctx.request.get(`${BACKEND}/api/auth/me`)
        .then(r => apiData<{ id: string }>(r)),
    ]);

    whitePage = p1Me.id === gameData.white_id ? p1.page : p2.page;
    blackPage = p1Me.id === gameData.white_id ? p2.page : p1.page;

    console.log(`\nBianco: ${p1Me.id === gameData.white_id ? p1.username : p2.username}`);
    console.log(`Nero:   ${p1Me.id === gameData.white_id ? p2.username : p1.username}`);
  });

  // ── 3. La partita ─────────────────────────────────────────────────────────

  test('3 — Opera Game: 17 mosse, finale Td8#', async () => {
    test.setTimeout(5 * 60_000); // budget di 5 minuti

    for (let i = 0; i < OPERA_GAME.length; i++) {
      const [from, to] = OPERA_GAME[i];
      const isWhite    = i % 2 === 0;
      const activePage = isWhite ? whitePage : blackPage;
      const moveNum    = Math.floor(i / 2) + 1;
      const moveName   = `${moveNum}${isWhite ? '.' : '…'} ${from}-${to}`;

      // Aspetta il proprio turno
      await expect(activePage.locator('.status-badge'))
        .toContainText(/your turn|tocca a te|tu turno/i, { timeout: 8_000 });

      // Pausa visiva prima di muovere
      await activePage.waitForTimeout(MOVE_DELAY_MS);

      // Esegui la mossa: click su from, poi click su to
      await activePage.locator(`[data-sq="${from}"]`).click();
      await activePage.locator(`[data-sq="${to}"]`).click();

      console.log(`  ♟  ${moveName}`);
    }

    console.log('\n  ✓  Td8# — Scaccomatto! Morphy vince.\n');

    // Aspetta che appaiano gli overlay di fine partita
    await Promise.all([
      expect(whitePage.locator('.overlay.finished')).toBeVisible({ timeout: 10_000 }),
      expect(blackPage.locator('.overlay.finished')).toBeVisible({ timeout: 10_000 }),
    ]);

    // Lascia un momento per ammirare la posizione finale
    await whitePage.waitForTimeout(4_000);
  });
});
