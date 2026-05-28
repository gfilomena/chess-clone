<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { Chess } from 'chess.js';
	import Board from '$lib/chess/Board.svelte';
	import { StockfishEngine } from '$lib/chess/stockfish';
	import { user, authLoading } from '$lib/stores/auth';
	import { initSounds, playSound, toggleMute, cycleTheme, soundTheme, themeLabel } from '$lib/chess/sounds';

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

	// ── Move navigation ────────────────────────────────────────────────────────
	interface HistoryEntry {
		fen: string;
		move: { from: string; to: string } | null;
		sound: import('$lib/chess/sounds').SoundName | null;
	}

	function buildBotHistory(moves: string[]): HistoryEntry[] {
		const entries: HistoryEntry[] = [{ fen: new Chess().fen(), move: null, sound: null }];
		const replay = new Chess();
		for (const san of moves) {
			try {
				const mv = replay.move(san) as any;
				if (mv) {
					const inCheck   = replay.inCheck();
					const isCapture = mv.flags.includes('c') || mv.flags.includes('e');
					const sound = inCheck ? 'check' : isCapture ? 'capture' : 'move';
					entries.push({ fen: replay.fen(), move: { from: mv.from, to: mv.to }, sound });
				}
			} catch {}
		}
		return entries;
	}

	let viewIndex = $state<number | null>(null); // null = live

	const botHistory  = $derived(buildBotHistory(moveHistory));
	const isReviewing = $derived(viewIndex !== null);
	const atStart     = $derived(viewIndex === 0);
	const atEnd       = $derived(viewIndex === null);

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

	function navFirst() { viewIndex = 0; }
	function navPrev() {
		const idx = viewIndex ?? botHistory.length - 1;
		if (idx > 0) { viewIndex = idx - 1; playNavSound(idx - 1); }
	}
	function navNext() {
		if (viewIndex === null) return;
		if (viewIndex < botHistory.length - 1) {
			viewIndex++;
			playNavSound(viewIndex);
		} else {
			viewIndex = null;
		}
	}
	function navLast() { viewIndex = null; }
	// ──────────────────────────────────────────────────────────────────────────

	// ── Engine ────────────────────────────────────────────────────────────────
	let engine: StockfishEngine | null = null;
	let engineReady = $state(false);

	let muted = $state(false);
	let panelOpen = $state(false);
	const currentTheme = $derived($soundTheme);

	onMount(async () => {
		initSounds();
		engine = new StockfishEngine();
		await engine.init();
		engineReady = true;
	});

	function handleToggleMute() { muted = toggleMute(); }

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
		playSound('game_start');

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
			const uciMove = await engine.getBestMove(chessGame.fen(), selectedElo);
			if (!uciMove) { isThinking = false; return; }

			const from  = uciMove.slice(0, 2);
			const to    = uciMove.slice(2, 4);
			const promo = uciMove.length >= 5 ? uciMove[4] : undefined;

			await sleep(350);

			const moveObj: Record<string, string> = { from, to };
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

	function checkGameOver(): boolean {
		if (!chessGame.isGameOver()) return false;

		playSound('game_over');

		if (chessGame.isCheckmate()) {
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
					fen={displayFen}
					{playerColor}
					isMyTurn={canMove}
					lastMove={displayLastMove}
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

			<!-- Pulsante toggle pannello (solo mobile) -->
			<button class="panel-toggle" onclick={() => panelOpen = !panelOpen}>
				{panelOpen ? '✕ Chiudi' : '📋 Mosse & Azioni'}
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
				<span>Mosse & Azioni</span>
				<button class="panel-close" onclick={() => panelOpen = false}>✕</button>
			</div>

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

			<!-- Audio controls -->
			<div class="audio-row">
				<button class="mute-btn" onclick={handleToggleMute} title={muted ? 'Attiva audio' : 'Disattiva audio'}>
					{muted ? '🔇' : '🔊'}
				</button>
				<button class="theme-btn" onclick={() => cycleTheme()} title="Cambia tema sonoro">
					{themeLabel(currentTheme)}
				</button>
			</div>

			<!-- Navigazione mosse -->
			<div class="nav-row" class:reviewing={isReviewing}>
				<button class="nav-btn" onclick={navFirst} disabled={atStart} title="Prima mossa">⏮</button>
				<button class="nav-btn" onclick={navPrev}  disabled={atStart} title="Mossa precedente">◀</button>
				<span class="nav-label" class:live={!isReviewing}>{navLabel}</span>
				<button class="nav-btn" onclick={navNext}  disabled={atEnd}   title="Mossa successiva">▶</button>
				<button class="nav-btn" onclick={navLast}  disabled={atEnd}   title="Ultima mossa">⏭</button>
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

.audio-row {
	display: flex;
	gap: 0.4rem;
}
.mute-btn {
	background: none;
	border: 1px solid var(--border);
	border-radius: 8px;
	color: var(--text-muted);
	font-size: 1rem;
	padding: 0.4rem 0.6rem;
	cursor: pointer;
	flex-shrink: 0;
	transition: border-color 0.15s, color 0.15s;
}
.mute-btn:hover { border-color: var(--accent); color: var(--text); }
.theme-btn {
	background: none;
	border: 1px solid var(--border);
	border-radius: 8px;
	color: var(--text-muted);
	font-size: 0.78rem;
	padding: 0.4rem 0.6rem;
	cursor: pointer;
	flex: 1;
	text-align: left;
	transition: border-color 0.15s, color 0.15s;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}
.theme-btn:hover { border-color: var(--accent); color: var(--text); }

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
	/* ELO grid: 2 colonne su mobile */
	.elo-grid {
		grid-template-columns: repeat(2, 1fr);
	}

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
}
</style>
