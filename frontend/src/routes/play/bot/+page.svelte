<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { Chess } from 'chess.js';
	import Board from '$lib/chess/Board.svelte';
	import { StockfishEngine } from '$lib/chess/stockfish';
	import { user, authLoading } from '$lib/stores/auth';
	import { initSounds, playSound } from '$lib/chess/sounds';
	import { computeCaptured } from '$lib/chess/captured';
	import { t } from '$lib/i18n';
	import { browser } from '$app/environment';
	import { API_URL as API } from '$lib/config';

	// ── Auth guard ────────────────────────────────────────────────────────────
	$effect(() => {
		if (!$authLoading && !$user) goto('/login');
	});

	// ── Bot roster ────────────────────────────────────────────────────────────
	const BOTS = [
		{ id: 'matteo',   name: 'Matteo',   elo:  400, stars: 1, piece: '♟', badge: 'Principiante', color: '#4a9e5c', quote: 'Ama il cavallo, non sa perché.'            },
		{ id: 'sofia',    name: 'Sofia',    elo:  700, stars: 2, piece: '♞', badge: 'Novizio',       color: '#7aaa3e', quote: 'Ha letto mezza pagina di teoria.'           },
		{ id: 'luca',     name: 'Luca',     elo: 1000, stars: 2, piece: '♝', badge: 'Intermedio',   color: '#c9a227', quote: 'Gioca e4 perché lo fanno tutti.'             },
		{ id: 'giulia',   name: 'Giulia',   elo: 1300, stars: 3, piece: '♜', badge: 'Club',         color: '#d4811e', quote: 'Conosce la Siciliana a memoria.'             },
		{ id: 'marco',    name: 'Marco',    elo: 1500, stars: 3, piece: '♛', badge: 'Avanzato',     color: '#c95f2f', quote: 'Analizza le partite la sera.'                },
		{ id: 'elena',    name: 'Elena',    elo: 1700, stars: 4, piece: '♚', badge: 'Esperto',      color: '#b84040', quote: 'Punisce ogni errore senza pietà.'            },
		{ id: 'riccardo', name: 'Riccardo', elo: 2000, stars: 4, piece: '♕', badge: 'Maestro',      color: '#8040a0', quote: 'Ha vinto tornei regionali.'                  },
		{ id: 'magnus',   name: 'Magnus',   elo: 2500, stars: 5, piece: '♔', badge: 'Gran Maestro', color: '#2040a0', quote: 'Pressoché imbattibile. Buona fortuna.'       },
	] as const;

	// ── Setup state ───────────────────────────────────────────────────────────
	let phase = $state<'setup' | 'playing'>('setup');
	let selectedBotId = $state('marco');
	let selectedColor: 'white' | 'black' | 'random' = $state('white');
	const selectedBot = $derived(BOTS.find(b => b.id === selectedBotId) ?? BOTS[4]);

	// ── Game state ────────────────────────────────────────────────────────────
	let playerColor = $state<'white' | 'black'>('white');
	let chessGame = new Chess();
	let fen = $state(chessGame.fen());
	let lastMove: { from: string; to: string } | null = $state(null);
	let isThinking = $state(false);
	type GameResult = { outcome: 'win' | 'loss' | 'draw'; reason: string } | null;
	let result: GameResult = $state(null);
	let moveHistory: string[] = $state([]);
	let savedBotGameId = $state<string | null>(null);

	// ── Move navigation ────────────────────────────────────────────────────────
	interface HistoryEntry {
		fen: string;
		move: { from: string; to: string } | null;
		sound: import('$lib/chess/sounds').SoundName | null;
		san:   string | null;
	}

	function buildBotHistory(moves: string[]): HistoryEntry[] {
		const entries: HistoryEntry[] = [{ fen: new Chess().fen(), move: null, sound: null, san: null }];
		const replay = new Chess();
		for (const san of moves) {
			try {
				const mv = replay.move(san) as any;
				if (mv) {
					const inCheck   = replay.inCheck();
					const isCapture = mv.flags.includes('c') || mv.flags.includes('e');
					const sound = inCheck ? 'check' : isCapture ? 'capture' : 'move';
					entries.push({ fen: replay.fen(), move: { from: mv.from, to: mv.to }, sound, san: mv.san });
				}
			} catch {}
		}
		return entries;
	}

	let viewIndex = $state<number | null>(null); // null = live
	let stripEl   = $state<HTMLElement | null>(null);

	const botHistory      = $derived(buildBotHistory(moveHistory));
	const isReviewing     = $derived(viewIndex !== null);
	const atStart         = $derived(viewIndex === 0);
	const atEnd           = $derived(viewIndex === null);
	const timelinePercent = $derived(
		botHistory.length <= 1 || viewIndex === null
			? 100
			: Math.round((viewIndex / (botHistory.length - 1)) * 100)
	);

	const displayFen = $derived(
		viewIndex === null ? fen : (botHistory[viewIndex]?.fen ?? fen)
	);
	const displayLastMove = $derived(
		viewIndex === null ? lastMove : (botHistory[viewIndex]?.move ?? null)
	);
	const navLabel = $derived(
		viewIndex === null ? 'Live' : `${viewIndex} / ${botHistory.length - 1}`
	);

	function playNavSound(idx: number) {
		const s = botHistory[idx]?.sound;
		if (s) playSound(s);
	}

	function navTo(idx: number) {
		if (idx <= 0)                      { viewIndex = 0; }
		else if (idx >= botHistory.length - 1) { viewIndex = null; }
		else { viewIndex = idx; playNavSound(idx); }
	}
	function navFirst() { viewIndex = 0; }
	function navPrev() {
		const idx = viewIndex ?? botHistory.length - 1;
		if (idx > 0) { viewIndex = idx - 1; playNavSound(idx - 1); }
	}
	function navNext() {
		if (viewIndex === null) return;
		if (viewIndex < botHistory.length - 1) { viewIndex++; playNavSound(viewIndex); }
		else viewIndex = null;
	}
	function navLast() { viewIndex = null; }

	// Auto-scroll strip al chip attivo
	$effect(() => {
		const idx = viewIndex ?? botHistory.length - 1;
		if (!stripEl) return;
		(stripEl.children[idx] as HTMLElement | undefined)
			?.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
	});

	// Se il bot muove mentre si è in revisione → torna a live
	let prevHistLen = 0;
	$effect(() => {
		const len = botHistory.length;
		if (len > prevHistLen && prevHistLen > 0 && isReviewing) viewIndex = null;
		prevHistLen = len;
	});
	// ──────────────────────────────────────────────────────────────────────────

	// ── Engine ────────────────────────────────────────────────────────────────
	let engine: StockfishEngine | null = null;
	let engineReady = $state(false);

	let panelOpen = $state(false);

	onMount(async () => {
		initSounds();
		engine = new StockfishEngine();
		await engine.init();
		engineReady = true;
	});

	onDestroy(() => {
		engine?.destroy();
	});

	// ── Derived ───────────────────────────────────────────────────────────────
	const currentTurn = $derived(fen.split(' ')[1] as 'w' | 'b');

	const isPlayerTurn = $derived(
		phase === 'playing' &&
		!isThinking &&
		result === null &&
		((playerColor === 'white' && currentTurn === 'w') ||
		 (playerColor === 'black' && currentTurn === 'b'))
	);

	// In modalità revisione il board non accetta mosse
	const canMove = $derived(!isReviewing && isPlayerTurn);

	// ── Pezzi catturati ─────────────────────────────────────────────
	const captured    = $derived(computeCaptured(displayFen));
	const myCaptured  = $derived(playerColor === 'white' ? captured.byWhite : captured.byBlack);
	const botCaptured = $derived(playerColor === 'white' ? captured.byBlack : captured.byWhite);
	const myAdv       = $derived(playerColor === 'white' ? captured.advantage : -captured.advantage);
	const botAdv      = $derived(-myAdv);

	const movePairs = $derived(buildMovePairs(moveHistory));

	const resultDisplayText = $derived((() => {
		if (!result) return '';
		const r = result as NonNullable<GameResult>;
		const outcome = r.outcome === 'win' ? $t.bot.you_win :
		                r.outcome === 'loss' ? $t.bot.bot_wins :
		                $t.bot.draw;
		const reason = r.reason ? (($t.game.reasons as any)[r.reason] ?? '') : '';
		return reason ? `${outcome} — ${reason}` : outcome;
	})());

	function buildMovePairs(history: string[]) {
		const pairs: Array<{ n: number; w: string; b: string }> = [];
		for (let i = 0; i < history.length; i += 2) {
			pairs.push({ n: i / 2 + 1, w: history[i] ?? '', b: history[i + 1] ?? '' });
		}
		return pairs;
	}

	// ── Game functions ────────────────────────────────────────────────────────

	function startGame() {
		if (!engineReady) return;

		const resolved: 'white' | 'black' =
			selectedColor === 'random'
				? (Math.random() < 0.5 ? 'white' : 'black')
				: selectedColor;

		playerColor = resolved;
		chessGame = new Chess();
		fen = chessGame.fen();
		lastMove = null;
		result = null;
		moveHistory = [];
		isThinking = false;
		phase = 'playing';
		playSound('game_start');

		if (playerColor === 'black') {
			triggerBotMove();
		}
	}

	function handlePlayerMove(from: string, to: string, promotion?: string) {
		if (!isPlayerTurn) return;

		try {
			const moveObj: { from: string; to: string; promotion?: string } = { from, to };
			if (promotion) moveObj.promotion = promotion;
			const move = chessGame.move(moveObj);
			if (!move) return;

			fen = chessGame.fen();
			lastMove = { from, to };
			moveHistory = [...moveHistory, move.san];

			// Suono mossa: cattura, scacco o mossa normale
			if (chessGame.isCheckmate())        playSound('check');
			else if (chessGame.inCheck())       playSound('check');
			else if (move.flags.includes('c') || move.flags.includes('e')) playSound('capture');
			else                                playSound('move');

			if (checkGameOver()) return;
			triggerBotMove();
		} catch {
			// invalid move – ignore
		}
	}

	async function triggerBotMove() {
		if (!engine || !engineReady || result !== null) return;
		isThinking = true;

		try {
			const uciMove = await engine.getBestMove(chessGame.fen(), selectedBot.elo);
			if (!uciMove) { isThinking = false; return; }

			const from  = uciMove.slice(0, 2);
			const to    = uciMove.slice(2, 4);
			const promo = uciMove.length >= 5 ? uciMove[4] : undefined;

			await sleep(350);

			const moveObj: { from: string; to: string; promotion?: string } = { from, to };
			if (promo) moveObj.promotion = promo;
			const move = chessGame.move(moveObj);

			if (move) {
				fen = chessGame.fen();
				lastMove = { from, to };
				moveHistory = [...moveHistory, move.san];

				if (chessGame.isCheckmate())        playSound('check');
				else if (chessGame.inCheck())       playSound('check');
				else if (move.flags.includes('c') || move.flags.includes('e')) playSound('capture');
				else                                playSound('move');

				checkGameOver();
			}
		} catch {
			// engine error – ignore
		}

		isThinking = false;
	}

	async function saveBotGame(gameResult: NonNullable<GameResult>) {
		// Salva sempre il PGN in sessionStorage come fallback
		if (browser) sessionStorage.setItem('botGamePgn', chessGame.pgn());

		// Prova a salvare nel database per persistenza nel profilo
		if (!$user) return;
		try {
			const res = await fetch(`${API}/api/bot-games`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					pgn:          chessGame.pgn(),
					outcome:      gameResult.outcome,
					finish_reason: gameResult.reason,
					player_color: playerColor,
				}),
			});
			if (res.ok) {
				const json = await res.json();
				savedBotGameId = json.data?.game_id ?? null;
			}
		} catch {
			// silenzio — il fallback sessionStorage è già pronto
		}
	}

	function checkGameOver(): boolean {
		if (!chessGame.isGameOver()) return false;

		playSound('game_over');

		let gameResult: NonNullable<GameResult>;
		if (chessGame.isCheckmate()) {
			const loserColor = chessGame.turn() === 'w' ? 'white' : 'black';
			gameResult = { outcome: loserColor === playerColor ? 'loss' : 'win', reason: 'checkmate' };
		} else if (chessGame.isStalemate()) {
			gameResult = { outcome: 'draw', reason: 'stalemate' };
		} else if (chessGame.isThreefoldRepetition()) {
			gameResult = { outcome: 'draw', reason: 'threefold' };
		} else if (chessGame.isInsufficientMaterial()) {
			gameResult = { outcome: 'draw', reason: '' };
		} else {
			gameResult = { outcome: 'draw', reason: '' };
		}
		result = gameResult;
		saveBotGame(gameResult);
		return true;
	}

	function resign() {
		const gameResult: NonNullable<GameResult> = { outcome: 'loss', reason: 'resigned' };
		result = gameResult;
		isThinking = false;
		saveBotGame(gameResult);
	}

	function backToSetup() {
		result = null;
		savedBotGameId = null;
		phase = 'setup';
		viewIndex = null;
		// Reset board so it doesn't flicker
		chessGame = new Chess();
		fen = chessGame.fen();
		lastMove = null;
		moveHistory = [];
	}

	function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)); }
</script>

<svelte:head>
	<title>{$t.bot.page_title} — Chess</title>
</svelte:head>

<!-- ══════════════════════════════════════════════════════════════════════════
     SETUP PHASE
══════════════════════════════════════════════════════════════════════════ -->
{#if phase === 'setup'}
	<div class="setup-page">
		<a href="/play" class="back-link">{$t.bot.back_to_menu}</a>

		<div class="setup-card">
			<div class="setup-header">
				<span class="setup-icon">🤖</span>
				<h1>{$t.bot.choose_opponent}</h1>
				<p class="setup-sub">{$t.bot.engine_desc}</p>
			</div>

			<!-- Bot selection -->
			<section class="setup-section">
				<h2>{$t.bot.opponent_label}</h2>
				<div class="bots-grid">
					{#each BOTS as bot}
						<button
							class="bot-card"
							class:active={selectedBotId === bot.id}
							onclick={() => selectedBotId = bot.id}
							style="--bot-color:{bot.color}"
						>
							<span class="bot-piece">{bot.piece}</span>
							<span class="bot-name">{bot.name}</span>
							<span class="bot-stars">{'★'.repeat(bot.stars)}{'☆'.repeat(5 - bot.stars)}</span>
							<span class="bot-badge">{bot.badge}</span>
							<span class="bot-quote">"{bot.quote}"</span>
							<span class="bot-elo">ELO {bot.elo}</span>
						</button>
					{/each}
				</div>
			</section>

			<!-- Color selection -->
			<section class="setup-section">
				<h2>{$t.bot.choose_color}</h2>
				<div class="color-row">
					<button class="color-btn" class:active={selectedColor === 'white'} onclick={() => selectedColor = 'white'}>
						<span class="color-piece">♔</span>{$t.bot.white}
					</button>
					<button class="color-btn" class:active={selectedColor === 'black'} onclick={() => selectedColor = 'black'}>
						<span class="color-piece dark">♚</span>{$t.bot.black}
					</button>
					<button class="color-btn" class:active={selectedColor === 'random'} onclick={() => selectedColor = 'random'}>
						<span class="color-piece">🎲</span>{$t.bot.random}
					</button>
				</div>
			</section>

			<!-- Start button -->
			<button class="btn btn-primary start-btn" onclick={startGame} disabled={!engineReady}>
				{#if engineReady}
					{$t.bot.challenge(selectedBot.name)}
				{:else}
					{$t.bot.engine_loading}
				{/if}
			</button>
		</div>
	</div>

<!-- ══════════════════════════════════════════════════════════════════════════
     PLAYING PHASE
══════════════════════════════════════════════════════════════════════════ -->
{:else}
	<div class="game-layout">

		<!-- ── Board column ─────────────────────────────────────── -->
		<div class="board-col">

			<!-- Opponent row (top) -->
			<div class="player-row">
				<div class="player-info">
					<span class="player-name">{selectedBot.piece} {selectedBot.name}</span>
					<span class="player-elo">
						<span class="inline-badge" style="background:{selectedBot.color}">{selectedBot.badge}</span>
						· ELO {selectedBot.elo}
					</span>
					{#if botCaptured.length > 0 || botAdv > 0}
						<div class="captured-row">
							{#each botCaptured as p}<span class="cap-piece">{p}</span>{/each}
							{#if botAdv > 0}<span class="cap-adv">+{botAdv}</span>{/if}
						</div>
					{/if}
				</div>
				{#if isThinking}
					<div class="thinking-badge">
						<div class="thinking-dot"></div>
						{$t.bot.thinking}
					</div>
				{/if}
			</div>

			<!-- Striscia mosse (solo mobile) -->
			<div class="mobile-moves-strip" bind:this={stripEl}>
				{#each botHistory as entry, i}
					{@const isActive = (viewIndex ?? botHistory.length - 1) === i}
					<button
						class="move-chip"
						class:active={isActive}
						class:start-chip={i === 0}
						onclick={() => navTo(i)}
					>
						{#if i === 0}◆{:else if i % 2 === 1}{Math.ceil(i / 2)}.{entry.san}{:else}{entry.san}{/if}
					</button>
				{/each}
			</div>

			<!-- Board -->
			<div class="board-container">
				{#if result !== null && !isReviewing}
					<div class="overlay finished">
						<p class="result-text">{resultDisplayText}</p>
						<div class="overlay-btns">
							<button class="btn btn-primary" onclick={backToSetup}>{$t.bot.new_game}</button>
							<a href="/analysis/{savedBotGameId ?? 'bot'}?autoReview=1" class="btn btn-google">{$t.game.review}</a>
						</div>
					</div>
				{/if}

				<Board
					fen={displayFen}
					{playerColor}
					isMyTurn={canMove}
					lastMove={displayLastMove}
					onMove={handlePlayerMove}
				/>
			</div>

			<!-- Nav bar timeline (solo mobile) -->
			<div class="mobile-nav-bar">
				<button class="nav-btn" onclick={navFirst} disabled={atStart} title={$t.common.first_move}>⏮</button>
				<button class="nav-btn" onclick={navPrev}  disabled={atStart} title={$t.common.prev_move}>◀</button>
				<div class="nav-timeline">
					<div class="timeline-track">
						<div class="timeline-fill" style="width:{timelinePercent}%">
							<span class="timeline-thumb"></span>
						</div>
					</div>
					<span class="timeline-label" class:live={!isReviewing}>{navLabel}</span>
				</div>
				<button class="nav-btn" onclick={navNext}  disabled={atEnd} title={$t.common.next_move}>▶</button>
				<button class="nav-btn" onclick={navLast}  disabled={atEnd} title={$t.common.last_move}>⏭</button>
			</div>

			<!-- Player row (bottom) -->
			<div class="player-row">
				<div class="player-info">
					<span class="player-name">👤 {$user?.username ?? 'Tu'}</span>
					<span class="player-elo">ELO {$user?.elo_rapid ?? '—'}</span>
					{#if myCaptured.length > 0 || myAdv > 0}
						<div class="captured-row">
							{#each myCaptured as p}<span class="cap-piece">{p}</span>{/each}
							{#if myAdv > 0}<span class="cap-adv">+{myAdv}</span>{/if}
						</div>
					{/if}
				</div>
			</div>

			<!-- Pulsante toggle pannello (solo mobile) -->
			<button class="panel-toggle" onclick={() => panelOpen = !panelOpen}>
				{panelOpen ? $t.common.close : $t.common.moves_actions}
			</button>
		</div>

		<!-- Backdrop pannello (mobile) -->
		<div
			class="panel-backdrop"
			class:panel-open={panelOpen}
			onclick={() => panelOpen = false}
			aria-hidden="true"
		></div>

		<!-- ── Side column ──────────────────────────────────────── -->
		<div class="side-col" class:panel-open={panelOpen}>

			<!-- Handle + header (solo mobile) -->
			<div class="panel-drag-handle"></div>
			<div class="panel-header">
				<span>{$t.common.moves_actions_title}</span>
				<button class="panel-close" onclick={() => panelOpen = false}>✕</button>
			</div>

			<!-- Move list -->
			<div class="moves-panel">
				<h3>{$t.common.moves}</h3>
				{#if movePairs.length === 0}
					<p class="no-moves">—</p>
				{:else}
					<div class="moves-list">
						{#each movePairs as pair (pair.n)}
							<div class="move-row">
								<span class="move-num">{pair.n}.</span>
								<span class="move-san">{pair.w}</span>
								{#if pair.b}
									<span class="move-san">{pair.b}</span>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Actions -->
			{#if result === null}
				<div class="actions">
					<button
						class="btn"
						style="background:var(--danger);color:#fff;width:100%"
						onclick={resign}
						disabled={isThinking}
					>
						{$t.bot.resign}
					</button>
				</div>
			{/if}

			<!-- Status badge -->
			<div class="status-badge" class:active={isPlayerTurn} class:thinking={isThinking}>
				{#if result !== null}
					{$t.bot.status_finished}
				{:else if isThinking}
					{$t.bot.status_thinking}
				{:else if isPlayerTurn}
					{$t.bot.status_your_turn}
				{:else}
					{$t.bot.status_wait}
				{/if}
			</div>

			<!-- Navigazione mosse -->
			<div class="nav-row" class:reviewing={isReviewing}>
				<button class="nav-btn" onclick={navFirst} disabled={atStart} title={$t.common.first_move}>⏮</button>
				<button class="nav-btn" onclick={navPrev}  disabled={atStart} title={$t.common.prev_move}>◀</button>
				<span class="nav-label" class:live={!isReviewing}>{navLabel}</span>
				<button class="nav-btn" onclick={navNext}  disabled={atEnd}   title={$t.common.next_move}>▶</button>
				<button class="nav-btn" onclick={navLast}  disabled={atEnd}   title={$t.common.last_move}>⏭</button>
			</div>

			<!-- Back link -->
			<button class="btn btn-google" style="width:100%;font-size:0.85rem" onclick={backToSetup}>
				{$t.bot.back}
			</button>
		</div>
	</div>
{/if}

<style>
/* ══════════════════════════════════════════════════════
   SETUP
══════════════════════════════════════════════════════ */
.setup-page {
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	height: 100dvh;
	overflow: hidden;
	padding: clamp(0.5rem, 1.5dvh, 1.5rem) 1rem;
	gap: clamp(0.3rem, 0.8dvh, 1rem);
}

.back-link {
	align-self: flex-start;
	color: var(--text-muted);
	font-size: 0.9rem;
	text-decoration: none;
	transition: color 0.15s;
}
.back-link:hover { color: var(--text); }

.setup-card {
	background: var(--bg-card);
	border: 1px solid var(--border);
	border-radius: 16px;
	padding: clamp(1rem, 2dvh, 2.5rem) clamp(1rem, 2vw, 2rem);
	width: 100%;
	max-width: 540px;
	display: flex;
	flex-direction: column;
	gap: clamp(0.6rem, 1.5dvh, 2rem);
	max-height: calc(100dvh - 2.5rem);
	overflow-y: auto;
	scrollbar-width: none;
}
.setup-card::-webkit-scrollbar { display: none; }

.setup-header {
	text-align: center;
}
.setup-icon {
	font-size: clamp(1.6rem, 3.5dvh, 3rem);
	display: block;
	margin-bottom: 0.25rem;
}
.setup-header h1 {
	font-size: clamp(1.2rem, 2.5dvh, 1.8rem);
	margin-bottom: 0.2rem;
}
.setup-sub {
	color: var(--text-muted);
	font-size: clamp(0.72rem, 1.3dvh, 0.95rem);
}

.setup-section h2 {
	font-size: 0.78rem;
	text-transform: uppercase;
	letter-spacing: 0.07em;
	color: var(--text-muted);
	font-weight: 600;
	margin-bottom: clamp(0.35rem, 0.8dvh, 0.75rem);
}

/* Color buttons */
.color-row {
	display: flex;
	gap: 0.6rem;
}
.color-btn {
	flex: 1;
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 0.3rem;
	padding: clamp(0.45rem, 1dvh, 0.85rem) 0.5rem;
	background: var(--bg);
	border: 2px solid var(--border);
	border-radius: 10px;
	cursor: pointer;
	color: var(--text);
	font-size: 0.85rem;
	font-weight: 500;
	transition: border-color 0.15s, background 0.15s;
}
.color-btn:hover { border-color: var(--accent); }
.color-btn.active {
	border-color: var(--accent);
	background: color-mix(in srgb, var(--accent) 12%, transparent);
}
.color-piece {
	font-size: clamp(1.3rem, 2.5dvh, 1.8rem);
	line-height: 1;
	filter: drop-shadow(0 1px 2px rgba(0,0,0,0.3));
}
.color-piece.dark { filter: drop-shadow(0 1px 2px rgba(0,0,0,0.7)); }

/* Bot grid */
.bots-grid {
	display: grid;
	grid-template-columns: repeat(4, 1fr);
	gap: clamp(0.3rem, 0.7dvh, 0.6rem);
}
.bot-card {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: clamp(0.08rem, 0.2dvh, 0.2rem);
	padding: clamp(0.35rem, 0.9dvh, 0.75rem) 0.4rem clamp(0.25rem, 0.7dvh, 0.6rem);
	background: var(--bg);
	border: 2px solid var(--border);
	border-radius: 10px;
	cursor: pointer;
	text-align: center;
	transition: border-color 0.15s, background 0.15s, transform 0.1s;
}
.bot-card:hover {
	border-color: var(--bot-color, var(--accent));
	transform: translateY(-1px);
}
.bot-card.active {
	border-color: var(--bot-color, var(--accent));
	background: color-mix(in srgb, var(--bot-color, var(--accent)) 10%, transparent);
}
.bot-piece {
	font-size: clamp(1.1rem, 2dvh, 1.7rem);
	line-height: 1;
	color: var(--bot-color, var(--accent));
	filter: drop-shadow(0 1px 3px rgba(0,0,0,0.4));
}
.bot-name {
	font-size: clamp(0.62rem, 1dvh, 0.82rem);
	font-weight: 700;
	color: var(--text);
}
.bot-stars {
	font-size: clamp(0.45rem, 0.75dvh, 0.6rem);
	color: var(--bot-color, var(--accent));
	letter-spacing: 1px;
}
.bot-badge {
	font-size: clamp(0.4rem, 0.65dvh, 0.55rem);
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.04em;
	padding: 0.1rem 0.4rem;
	border-radius: 20px;
	background: var(--bot-color, var(--accent));
	color: #fff;
	white-space: nowrap;
}
.bot-quote {
	font-size: clamp(0.45rem, 0.7dvh, 0.58rem);
	color: var(--text-muted);
	font-style: italic;
	line-height: 1.3;
	display: none; /* visibile solo su card attiva */
}
.bot-card.active .bot-quote { display: block; }
.bot-elo {
	font-size: clamp(0.45rem, 0.7dvh, 0.6rem);
	color: var(--text-muted);
}
/* Badge inline nella riga giocatore */
.inline-badge {
	display: inline-block;
	font-size: 0.65rem;
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.04em;
	padding: 0.1rem 0.4rem;
	border-radius: 20px;
	color: #fff;
	vertical-align: middle;
}

.start-btn {
	width: 100%;
	padding: clamp(0.6rem, 1.3dvh, 1rem);
	font-size: clamp(0.9rem, 1.5dvh, 1.05rem);
}
.start-btn:disabled {
	opacity: 0.5;
	cursor: not-allowed;
}

/* ══════════════════════════════════════════════════════
   GAME LAYOUT (same as /game/[id])
══════════════════════════════════════════════════════ */
.game-layout {
	display: flex;
	gap: clamp(0.75rem, 1.5vw, 2rem);
	padding: clamp(0.4rem, 0.8dvh, 1rem) clamp(0.75rem, 1.5vw, 2rem);
	height: 100dvh;
	overflow: hidden;
	align-items: center;
	justify-content: center;
}

.board-col {
	display: flex;
	flex-direction: column;
	gap: 0.75rem;
}

.player-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: 1rem;
	min-height: 2.5rem;
}

.player-info {
	display: flex;
	flex-direction: column;
}
.player-name {
	font-weight: 600;
	font-size: 1rem;
}
.player-elo {
	font-size: 0.8rem;
	color: var(--text-muted);
}

.captured-row {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 0.04rem;
	margin-top: 0.12rem;
}
.cap-piece {
	font-size: 1.25rem;
	line-height: 1;
	opacity: 0.85;
}
.cap-adv {
	font-size: 0.85rem;
	font-weight: 700;
	color: var(--accent);
	margin-left: 0.3rem;
}

.thinking-badge {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	font-size: 0.8rem;
	color: var(--text-muted);
}
.thinking-dot {
	width: 8px;
	height: 8px;
	border-radius: 50%;
	background: var(--accent);
	animation: blink 0.8s ease infinite alternate;
}
@keyframes blink { to { opacity: 0.2; } }

.board-container {
	position: relative;
}

.overlay {
	position: absolute;
	inset: 0;
	background: rgba(0,0,0,0.65);
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	z-index: 5;
	border-radius: 4px;
	font-size: 1.2rem;
}
.result-text {
	font-size: 1.6rem;
	font-weight: 700;
	color: var(--accent);
	text-align: center;
	padding: 0 1rem;
}

.overlay-btns {
	display: flex;
	flex-direction: column;
	gap: 0.6rem;
	margin-top: 1.2rem;
	min-width: 180px;
}
.overlay-btns .btn,
.overlay-btns button { text-align: center; width: 100%; }

/* ── Side column ── */
.side-col {
	display: flex;
	flex-direction: column;
	gap: 1rem;
	width: 240px;
	padding-top: 0;
	height: 100%;
	justify-content: center;
}

.moves-panel {
	background: var(--bg-card);
	border: 1px solid var(--border);
	border-radius: 8px;
	padding: 1rem;
	flex: 1;
	min-height: 0;
	max-height: none;
	overflow-y: auto;
}
.moves-panel h3 {
	margin-bottom: 0.75rem;
	color: var(--text-muted);
	font-size: 0.85rem;
	text-transform: uppercase;
	letter-spacing: 0.05em;
}
.no-moves { color: var(--text-muted); font-size: 0.9rem; }

.moves-list {
	display: flex;
	flex-direction: column;
	gap: 0.2rem;
}
.move-row {
	display: flex;
	gap: 0.5rem;
	font-size: 0.85rem;
	line-height: 1.6;
}
.move-num {
	color: var(--text-muted);
	min-width: 1.6rem;
}
.move-san {
	font-family: monospace;
	min-width: 4rem;
}

.actions {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.status-badge {
	text-align: center;
	padding: 0.5rem;
	border-radius: 8px;
	font-size: 0.9rem;
	background: var(--bg-card);
	border: 1px solid var(--border);
	color: var(--text-muted);
}
.status-badge.active {
	border-color: var(--accent);
	color: var(--accent);
}
.status-badge.thinking {
	border-color: #e6a817;
	color: #e6a817;
}

/* ── Mobile moves strip & nav bar (nascosti su desktop) ── */
.mobile-moves-strip { display: none; }
.mobile-nav-bar     { display: none; }

/* ── Move navigation ── */
.nav-row {
	display: flex;
	align-items: center;
	gap: 0.25rem;
	background: var(--bg-card);
	border: 1px solid var(--border);
	border-radius: 8px;
	padding: 0.3rem 0.4rem;
	transition: border-color 0.2s;
}
.nav-row.reviewing {
	border-color: var(--accent);
}
.nav-btn {
	background: none;
	border: none;
	color: var(--text-muted);
	font-size: 0.78rem;
	padding: 0.3rem 0.45rem;
	cursor: pointer;
	border-radius: 5px;
	transition: background 0.12s, color 0.12s;
	line-height: 1;
	flex-shrink: 0;
}
.nav-btn:hover:not(:disabled) {
	background: rgba(255,255,255,0.08);
	color: var(--text);
}
.nav-btn:disabled {
	opacity: 0.3;
	cursor: default;
}
.nav-label {
	flex: 1;
	text-align: center;
	font-size: 0.72rem;
	font-weight: 600;
	color: var(--text-muted);
	letter-spacing: 0.02em;
	white-space: nowrap;
}
.nav-label.live {
	color: #e05050;
}

/* ── Panel toggle / backdrop (nascosti su desktop) ── */
.panel-toggle  { display: none; }
.panel-backdrop { display: none; }
.panel-drag-handle { display: none; }
.panel-header  { display: none; }

/* ══════════════════════════════════════════════════════
   MOBILE (≤ 768px)
══════════════════════════════════════════════════════ */
@media (max-width: 768px) {
	/* Setup card: padding ridotto, max-width pieno */
	.setup-card {
		padding: 1.5rem 1.25rem;
		max-width: 100%;
		border-radius: 12px;
	}
	.setup-page {
		padding: 1rem 0.75rem 2rem;
	}
	/* Bot grid: 2 colonne su mobile */
	.bots-grid {
		grid-template-columns: repeat(2, 1fr);
	}
	.bot-card.active .bot-quote { display: block; }

	/* ── Game layout mobile ── */
	.game-layout {
		flex-direction: column;
		padding: 0.5rem 0.5rem 1rem;
		gap: 0.4rem;
		align-items: center;
		min-height: 0;
	}
	.board-col {
		width: 100%;
		align-items: center;
	}
	.player-row {
		width: min(calc(100vw - 1rem), calc(100vh - 220px));
	}

	/* Pulsante toggle pannello */
	.panel-toggle {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.4rem;
		width: min(calc(100vw - 1rem), calc(100vh - 220px));
		padding: 0.55rem 1rem;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		color: var(--text);
		font-size: 0.9rem;
		cursor: pointer;
		font-weight: 500;
		transition: border-color 0.15s;
	}
	.panel-toggle:hover { border-color: var(--accent); }

	/* Backdrop */
	.panel-backdrop {
		display: block;
		position: fixed;
		inset: 0;
		background: rgba(0,0,0,0.45);
		z-index: 40;
		opacity: 0;
		pointer-events: none;
		transition: opacity 0.25s ease;
	}
	.panel-backdrop.panel-open {
		opacity: 1;
		pointer-events: auto;
	}

	/* Side col → bottom sheet */
	.side-col {
		position: fixed;
		bottom: 0; left: 0; right: 0;
		width: 100% !important;
		max-height: 72vh;
		background: var(--bg-card);
		border-top: 2px solid var(--border);
		border-radius: 16px 16px 0 0;
		padding: 0 1rem 2rem;
		z-index: 50;
		overflow-y: auto;
		transform: translateY(100%);
		transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
		gap: 0.75rem;
	}
	.side-col.panel-open {
		transform: translateY(0);
	}

	.panel-drag-handle {
		display: block;
		width: 36px;
		height: 4px;
		background: var(--border);
		border-radius: 2px;
		margin: 0.8rem auto 0.4rem;
		flex-shrink: 0;
	}
	.panel-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.5rem 0 0.65rem;
		position: sticky;
		top: 0;
		background: var(--bg-card);
		z-index: 1;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}
	.panel-header span {
		font-weight: 600;
		font-size: 0.9rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.panel-close {
		display: flex;
		align-items: center;
		justify-content: center;
		background: none;
		border: none;
		color: var(--text-muted);
		font-size: 1.1rem;
		cursor: pointer;
		padding: 0.25rem;
		line-height: 1;
		transition: color 0.15s;
	}
	.panel-close:hover { color: var(--text); }

	.moves-panel {
		flex: none;
		max-height: 200px;
		overflow-y: auto;
	}

	/* ── Striscia mosse mobile ── */
	.mobile-moves-strip {
		display: flex;
		overflow-x: auto;
		overflow-y: hidden;
		gap: 0.2rem;
		padding: 0.3rem 0.4rem;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		scrollbar-width: none;
		width: min(calc(100vw - 1rem), calc(100vh - 220px));
		-webkit-overflow-scrolling: touch;
		flex-shrink: 0;
	}
	.mobile-moves-strip::-webkit-scrollbar { display: none; }

	.move-chip {
		flex-shrink: 0;
		background: none;
		border: 1px solid transparent;
		border-radius: 4px;
		color: var(--text-muted);
		font-size: 0.65rem;
		font-family: monospace;
		padding: 0.18rem 0.32rem;
		cursor: pointer;
		white-space: nowrap;
		line-height: 1.4;
		transition: background 0.1s, color 0.1s, border-color 0.1s;
	}
	.move-chip:hover:not(.active) {
		background: rgba(255,255,255,0.06);
		color: var(--text);
	}
	.move-chip.active {
		background: var(--accent);
		border-color: var(--accent);
		color: #000;
		font-weight: 700;
	}
	.move-chip.start-chip {
		color: var(--accent);
		font-size: 0.55rem;
	}

	/* ── Nav bar mobile ── */
	.mobile-nav-bar {
		display: flex;
		align-items: center;
		gap: 0.3rem;
		width: min(calc(100vw - 1rem), calc(100vh - 220px));
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.4rem 0.5rem;
		flex-shrink: 0;
	}
	.mobile-nav-bar .nav-btn {
		font-size: 0.75rem;
		padding: 0.3rem 0.4rem;
	}

	.nav-timeline {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		min-width: 0;
	}
	.timeline-track {
		position: relative;
		height: 5px;
		background: var(--border);
		border-radius: 3px;
	}
	.timeline-fill {
		position: relative;
		height: 100%;
		background: var(--accent);
		border-radius: 3px;
		transition: width 0.15s ease;
		min-width: 6px;
	}
	.timeline-thumb {
		position: absolute;
		right: 0;
		top: 50%;
		transform: translate(50%, -50%);
		width: 11px;
		height: 11px;
		background: var(--accent);
		border-radius: 50%;
		border: 2px solid var(--bg-card);
		box-shadow: 0 0 0 1px var(--accent);
	}
	.timeline-label {
		text-align: center;
		font-size: 0.62rem;
		font-weight: 600;
		color: var(--text-muted);
		letter-spacing: 0.03em;
	}
	.timeline-label.live { color: #e05050; }

	/* Nascondi nav-row nel side-col su mobile */
	.side-col .nav-row { display: none; }
}
</style>
