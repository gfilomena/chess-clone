<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { Chess } from 'chess.js';
	import Board from '$lib/chess/Board.svelte';
	import { StockfishEngine } from '$lib/chess/stockfish';
	import { user, authLoading } from '$lib/stores/auth';

	// ── Auth guard ────────────────────────────────────────────────────────────
	$effect(() => {
		if (!$authLoading && !$user) goto('/login');
	});

	// ── ELO levels ────────────────────────────────────────────────────────────
	const ELO_LEVELS = [
		{ elo: 200,  label: 'Principiante' },
		{ elo: 400,  label: 'Novizio'       },
		{ elo: 600,  label: 'Dilettante'    },
		{ elo: 800,  label: 'Intermedio'    },
		{ elo: 1000, label: 'Club'          },
		{ elo: 1200, label: 'Avanzato'      },
		{ elo: 1400, label: 'Esperto'       },
		{ elo: 1600, label: 'Cand. Master'  },
		{ elo: 1800, label: 'Master'        },
		{ elo: 2000, label: 'Gran Maestro'  },
	] as const;

	// ── Setup state ───────────────────────────────────────────────────────────
	let phase: 'setup' | 'playing' = $state('setup');
	let selectedElo = $state(1000);
	let selectedColor: 'white' | 'black' | 'random' = $state('white');

	// ── Game state ────────────────────────────────────────────────────────────
	let playerColor: 'white' | 'black' = $state('white');
	let chessGame = new Chess();
	let fen = $state(chessGame.fen());
	let lastMove: { from: string; to: string } | null = $state(null);
	let isThinking = $state(false);
	let result: string | null = $state(null);
	let moveHistory: string[] = $state([]);

	// ── Engine ────────────────────────────────────────────────────────────────
	let engine: StockfishEngine | null = null;
	let engineReady = $state(false);

	onMount(async () => {
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

	const selectedLevel = $derived(ELO_LEVELS.find(l => l.elo === selectedElo) ?? ELO_LEVELS[4]);

	const movePairs = $derived(buildMovePairs(moveHistory));

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

		// Bot plays first if player chose black
		if (playerColor === 'black') {
			triggerBotMove();
		}
	}

	function handlePlayerMove(from: string, to: string, promotion?: string) {
		if (!isPlayerTurn) return;

		try {
			const moveObj: Record<string, string> = { from, to };
			if (promotion) moveObj.promotion = promotion;
			const move = chessGame.move(moveObj);
			if (!move) return;

			fen = chessGame.fen();
			lastMove = { from, to };
			moveHistory = [...moveHistory, move.san];

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
			const uciMove = await engine.getBestMove(chessGame.fen(), selectedElo);
			if (!uciMove) { isThinking = false; return; }

			// UCI format: "e2e4" or "e7e8q" (promotion)
			const from  = uciMove.slice(0, 2);
			const to    = uciMove.slice(2, 4);
			const promo = uciMove.length >= 5 ? uciMove[4] : undefined;

			// Small visual delay so the move doesn't appear instantly
			await sleep(350);

			const moveObj: Record<string, string> = { from, to };
			if (promo) moveObj.promotion = promo;
			const move = chessGame.move(moveObj);

			if (move) {
				fen = chessGame.fen();
				lastMove = { from, to };
				moveHistory = [...moveHistory, move.san];
				checkGameOver();
			}
		} catch {
			// engine error – ignore
		}

		isThinking = false;
	}

	function checkGameOver(): boolean {
		if (!chessGame.isGameOver()) return false;

		if (chessGame.isCheckmate()) {
			// chessGame.turn() is the color in checkmate (lost)
			const loserColor = chessGame.turn() === 'w' ? 'white' : 'black';
			result = loserColor === playerColor
				? 'Scacco matto — Hai perso!'
				: 'Scacco matto — Hai vinto! 🎉';
		} else if (chessGame.isStalemate()) {
			result = 'Patta — Stallo';
		} else if (chessGame.isThreefoldRepetition()) {
			result = 'Patta — Ripetizione';
		} else if (chessGame.isInsufficientMaterial()) {
			result = 'Patta — Materiale insufficiente';
		} else {
			result = 'Patta';
		}
		return true;
	}

	function resign() {
		result = 'Hai abbandonato';
		isThinking = false;
	}

	function backToSetup() {
		result = null;
		phase = 'setup';
		// Reset board so it doesn't flicker
		chessGame = new Chess();
		fen = chessGame.fen();
		lastMove = null;
		moveHistory = [];
	}

	function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)); }
</script>

<svelte:head>
	<title>Gioca vs Bot — Chess Clone</title>
</svelte:head>

<!-- ══════════════════════════════════════════════════════════════════════════
     SETUP PHASE
══════════════════════════════════════════════════════════════════════════ -->
{#if phase === 'setup'}
	<div class="setup-page">
		<a href="/play" class="back-link">← Torna al menu</a>

		<div class="setup-card">
			<div class="setup-header">
				<span class="setup-icon">🤖</span>
				<h1>Gioca contro il Bot</h1>
				<p class="setup-sub">Sfida Stockfish al livello che preferisci</p>
			</div>

			<!-- Color selection -->
			<section class="setup-section">
				<h2>Scegli il tuo colore</h2>
				<div class="color-row">
					<button
						class="color-btn"
						class:active={selectedColor === 'white'}
						onclick={() => selectedColor = 'white'}
					>
						<span class="color-piece">♔</span>
						Bianco
					</button>
					<button
						class="color-btn"
						class:active={selectedColor === 'black'}
						onclick={() => selectedColor = 'black'}
					>
						<span class="color-piece dark">♚</span>
						Nero
					</button>
					<button
						class="color-btn"
						class:active={selectedColor === 'random'}
						onclick={() => selectedColor = 'random'}
					>
						<span class="color-piece">🎲</span>
						Casuale
					</button>
				</div>
			</section>

			<!-- ELO level selection -->
			<section class="setup-section">
				<h2>Livello di difficoltà</h2>
				<div class="elo-grid">
					{#each ELO_LEVELS as level}
						<button
							class="elo-btn"
							class:active={selectedElo === level.elo}
							onclick={() => selectedElo = level.elo}
						>
							<span class="elo-num">{level.elo}</span>
							<span class="elo-lbl">{level.label}</span>
						</button>
					{/each}
				</div>
				<p class="selected-info">
					Livello selezionato: <strong>{selectedLevel.elo} ELO</strong> · {selectedLevel.label}
				</p>
			</section>

			<!-- Start button -->
			<button
				class="btn btn-primary start-btn"
				onclick={startGame}
				disabled={!engineReady}
			>
				{#if engineReady}
					▶ Inizia partita
				{:else}
					Caricamento motore...
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
					<span class="player-name">🤖 Bot</span>
					<span class="player-elo">ELO {selectedElo} · {selectedLevel.label}</span>
				</div>
				{#if isThinking}
					<div class="thinking-badge">
						<div class="thinking-dot"></div>
						Sta pensando...
					</div>
				{/if}
			</div>

			<!-- Board -->
			<div class="board-container">
				{#if result !== null}
					<div class="overlay finished">
						<p class="result-text">{result}</p>
						<button class="btn btn-primary" style="width:auto;margin-top:1rem" onclick={backToSetup}>
							Nuova partita
						</button>
					</div>
				{/if}

				<Board
					{fen}
					{playerColor}
					isMyTurn={isPlayerTurn}
					{lastMove}
					onMove={handlePlayerMove}
				/>
			</div>

			<!-- Player row (bottom) -->
			<div class="player-row">
				<div class="player-info">
					<span class="player-name">👤 {$user?.username ?? 'Tu'}</span>
					<span class="player-elo">ELO {$user?.elo_rapid ?? '—'}</span>
				</div>
			</div>
		</div>

		<!-- ── Side column ──────────────────────────────────────── -->
		<div class="side-col">

			<!-- Move list -->
			<div class="moves-panel">
				<h3>Mosse</h3>
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
						Abbandona
					</button>
				</div>
			{/if}

			<!-- Status badge -->
			<div class="status-badge" class:active={isPlayerTurn} class:thinking={isThinking}>
				{#if result !== null}
					🏁 Partita terminata
				{:else if isThinking}
					🤔 Bot sta pensando...
				{:else if isPlayerTurn}
					🟢 Tocca a te
				{:else}
					⏳ Aspetta...
				{/if}
			</div>

			<!-- Back link -->
			<button class="btn btn-google" style="width:100%;font-size:0.85rem" onclick={backToSetup}>
				← Nuova partita
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
	padding: 2rem 1rem 3rem;
	gap: 1.5rem;
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
	padding: 2.5rem 2rem;
	width: 100%;
	max-width: 540px;
	display: flex;
	flex-direction: column;
	gap: 2rem;
}

.setup-header {
	text-align: center;
}
.setup-icon {
	font-size: 3rem;
	display: block;
	margin-bottom: 0.5rem;
}
.setup-header h1 {
	font-size: 1.8rem;
	margin-bottom: 0.35rem;
}
.setup-sub {
	color: var(--text-muted);
	font-size: 0.95rem;
}

.setup-section h2 {
	font-size: 0.8rem;
	text-transform: uppercase;
	letter-spacing: 0.07em;
	color: var(--text-muted);
	font-weight: 600;
	margin-bottom: 0.75rem;
}

/* Color buttons */
.color-row {
	display: flex;
	gap: 0.75rem;
}
.color-btn {
	flex: 1;
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 0.4rem;
	padding: 0.85rem 0.5rem;
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
	font-size: 1.8rem;
	line-height: 1;
	filter: drop-shadow(0 1px 2px rgba(0,0,0,0.3));
}
.color-piece.dark { filter: drop-shadow(0 1px 2px rgba(0,0,0,0.7)); }

/* ELO grid */
.elo-grid {
	display: grid;
	grid-template-columns: repeat(5, 1fr);
	gap: 0.5rem;
}
.elo-btn {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 0.2rem;
	padding: 0.6rem 0.25rem;
	background: var(--bg);
	border: 2px solid var(--border);
	border-radius: 8px;
	cursor: pointer;
	color: var(--text);
	transition: border-color 0.15s, background 0.15s;
}
.elo-btn:hover { border-color: var(--accent); }
.elo-btn.active {
	border-color: var(--accent);
	background: color-mix(in srgb, var(--accent) 14%, transparent);
}
.elo-num {
	font-size: 0.95rem;
	font-weight: 700;
}
.elo-lbl {
	font-size: 0.65rem;
	color: var(--text-muted);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
	max-width: 100%;
}
.selected-info {
	margin-top: 0.75rem;
	text-align: center;
	font-size: 0.9rem;
	color: var(--text-muted);
}

.start-btn {
	width: 100%;
	padding: 1rem;
	font-size: 1.05rem;
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
	gap: 2rem;
	padding: 1.5rem 2rem;
	min-height: 100vh;
	align-items: flex-start;
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

/* ── Side column ── */
.side-col {
	display: flex;
	flex-direction: column;
	gap: 1rem;
	width: 240px;
	padding-top: 3rem;
}

.moves-panel {
	background: var(--bg-card);
	border: 1px solid var(--border);
	border-radius: 8px;
	padding: 1rem;
	flex: 1;
	max-height: 340px;
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
</style>
