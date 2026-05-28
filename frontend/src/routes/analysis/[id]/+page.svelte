<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { Chess } from 'chess.js';
	import Board from '$lib/chess/Board.svelte';
	import { StockfishEngine, evalToPercent, formatScore, type AnalysisResult } from '$lib/chess/stockfish';
	import { API_URL as API } from '$lib/config';

	const gameId = $page.params.id;

	// ── Stato ──────────────────────────────────────────────────────────────
	let game = $state<any>(null);
	let positions = $state<string[]>([]);   // FEN per ogni mossa
	let moveLabels = $state<string[]>([]);  // es. "1. e4", "1... e5"
	let currentIdx = $state(0);
	let analysis = $state<AnalysisResult | null>(null);
	let analyzing = $state(false);
	let engine: StockfishEngine | null = null;
	let loading = $state(true);
	let error = $state('');

	// ── Caricamento partita ────────────────────────────────────────────────
	onMount(async () => {
		try {
			const res = await fetch(`${API}/api/games/${gameId}`);
			const json = await res.json();
			if (!json.success) throw new Error('Partita non trovata');
			game = json.data;

			// Parsa PGN → array di FEN
			parsePositions(game.pgn);

			// Inizializza Stockfish
			engine = new StockfishEngine();
			await engine.init();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}

		// Analizza posizione iniziale
		await analyzeCurrentPosition();
	});

	onDestroy(() => {
		engine?.destroy();
	});

	// ── Parser PGN → posizioni ─────────────────────────────────────────────
	function parsePositions(pgn: string) {
		const chess = new Chess();
		if (pgn) {
			try { chess.loadPgn(pgn); } catch {}
		}

		const history = chess.history({ verbose: true }) as any[];
		const fens: string[] = [];
		const labels: string[] = [];

		const replay = new Chess();
		fens.push(replay.fen()); // posizione iniziale
		labels.push('Inizio');

		for (let i = 0; i < history.length; i++) {
			const move = history[i];
			replay.move(move);
			fens.push(replay.fen());

			const moveNum = Math.floor(i / 2) + 1;
			const label = i % 2 === 0 ? `${moveNum}. ${move.san}` : `${moveNum}... ${move.san}`;
			labels.push(label);
		}

		positions = fens;
		moveLabels = labels;
	}

	// ── Navigazione ────────────────────────────────────────────────────────
	async function goTo(idx: number) {
		if (idx < 0 || idx >= positions.length) return;
		engine?.stop();
		currentIdx = idx;
		analysis = null;
		await analyzeCurrentPosition();
	}

	function goFirst() { goTo(0); }
	function goPrev()  { goTo(currentIdx - 1); }
	function goNext()  { goTo(currentIdx + 1); }
	function goLast()  { goTo(positions.length - 1); }

	// ── Analisi Stockfish ─────────────────────────────────────────────────
	async function analyzeCurrentPosition() {
		if (!engine || !positions[currentIdx]) return;
		analyzing = true;
		try {
			analysis = await engine.analyze(positions[currentIdx], 16);
		} finally {
			analyzing = false;
		}
	}

	// ── Tastiera ──────────────────────────────────────────────────────────
	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowLeft')  goPrev();
		if (e.key === 'ArrowRight') goNext();
		if (e.key === 'ArrowUp')    goFirst();
		if (e.key === 'ArrowDown')  goLast();
	}

	// ── Helper UI ─────────────────────────────────────────────────────────
	const evalPercent = $derived(analysis ? evalToPercent(analysis) : 50);
	const evalText    = $derived(analysis ? formatScore(analysis) : '...');

	function resultBadge(result: string | null): string {
		if (!result) return '';
		return { white: 'Bianco vince', black: 'Nero vince', draw: 'Patta' }[result] ?? result;
	}
</script>

<svelte:head>
	<title>Analisi — Chess</title>
</svelte:head>

<svelte:window onkeydown={handleKeydown} />

{#if loading}
	<div class="center"><p>Caricamento partita...</p></div>
{:else if error}
	<div class="center"><p class="error-msg">{error}</p></div>
{:else}
<div class="analysis-layout">

	<!-- Eval bar verticale -->
	<div class="eval-bar" title="{evalText}">
		<div class="eval-black" style="height: {100 - evalPercent}%"></div>
		<div class="eval-white" style="height: {evalPercent}%"></div>
		<span class="eval-label" style="bottom:{evalPercent}%">
			{evalText}
		</span>
	</div>

	<!-- Scacchiera (sola lettura) -->
	<div class="board-col">
		<div class="game-info">
			<span class="player-tag black-tag">♟ {game.black_username} ({game.black_elo})</span>
			<span class="result-tag">{resultBadge(game.result)}</span>
			<span class="player-tag white-tag">♔ {game.white_username} ({game.white_elo})</span>
		</div>

		<Board
			fen={positions[currentIdx] ?? 'rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1'}
			playerColor="white"
			isMyTurn={false}
			lastMove={null}
			onMove={() => {}}
		/>

		<!-- Navigazione -->
		<div class="nav-controls">
			<button onclick={goFirst} title="Prima mossa">⏮</button>
			<button onclick={goPrev}  title="Mossa precedente (←)">◀</button>
			<span class="move-counter">{currentIdx} / {positions.length - 1}</span>
			<button onclick={goNext}  title="Mossa successiva (→)">▶</button>
			<button onclick={goLast}  title="Ultima mossa">⏭</button>
		</div>

		<!-- Info engine -->
		<div class="engine-info">
			{#if analyzing}
				<span class="analyzing">⚙ Analisi in corso...</span>
			{:else if analysis}
				<span>Depth {analysis.depth}</span>
				<span class="score-text" class:positive={analysis.score > 0} class:negative={analysis.score < 0}>
					{evalText}
				</span>
				{#if analysis.bestMove}
					<span>Mossa migliore: <strong>{analysis.bestMove}</strong></span>
				{/if}
			{/if}
		</div>
	</div>

	<!-- Lista mosse -->
	<div class="moves-col">
		<h3>Mosse</h3>
		<div class="moves-list">
			{#each moveLabels as label, i}
				<button
					class="move-item"
					class:active={i === currentIdx}
					onclick={() => goTo(i)}
				>
					{label}
				</button>
			{/each}
		</div>

		<div class="actions">
			<a
				href={`${API}/api/games/${gameId}/pgn`}
				class="btn btn-google"
				style="text-align:center"
			>
				⬇ Scarica PGN
			</a>
			<a href="/" class="btn btn-primary" style="text-align:center">
				Nuova partita
			</a>
		</div>
	</div>

</div>
{/if}

<style>
	.center {
		display: flex;
		justify-content: center;
		padding: 4rem;
	}

	.analysis-layout {
		display: flex;
		gap: 1rem;
		padding: 1.5rem;
		align-items: flex-start;
		justify-content: center;
	}

	/* Eval bar */
	.eval-bar {
		width: 20px;
		height: 480px;
		border-radius: 4px;
		overflow: hidden;
		position: relative;
		border: 1px solid var(--border);
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		margin-top: 4rem;
	}
	.eval-black { background: #1a1a1a; transition: height 0.4s; }
	.eval-white { background: #f0d9b5; transition: height 0.4s; }
	.eval-label {
		position: absolute;
		left: 50%;
		transform: translateX(-50%);
		font-size: 0.6rem;
		color: var(--text-muted);
		white-space: nowrap;
	}

	/* Board column */
	.board-col {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.game-info {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.85rem;
	}

	.player-tag {
		font-weight: 600;
		padding: 0.2rem 0.5rem;
		border-radius: 4px;
	}
	.black-tag { background: #2d3748; }
	.white-tag { background: #4a5568; }
	.result-tag {
		color: var(--accent);
		font-weight: 700;
	}

	/* Navigation */
	.nav-controls {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
	}
	.nav-controls button {
		background: var(--bg-card);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: 6px;
		padding: 0.4rem 0.8rem;
		font-size: 1rem;
		cursor: pointer;
		transition: border-color 0.2s;
	}
	.nav-controls button:hover { border-color: var(--accent); }
	.move-counter {
		color: var(--text-muted);
		font-size: 0.85rem;
		min-width: 60px;
		text-align: center;
	}

	/* Engine info */
	.engine-info {
		display: flex;
		gap: 1rem;
		align-items: center;
		font-size: 0.85rem;
		color: var(--text-muted);
		justify-content: center;
		min-height: 24px;
	}
	.analyzing { color: var(--accent); }
	.score-text { font-weight: 700; }
	.score-text.positive { color: #f0d9b5; }
	.score-text.negative { color: #666; }

	/* Moves column */
	.moves-col {
		width: 200px;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		padding-top: 3.5rem;
	}
	.moves-col h3 {
		color: var(--text-muted);
		font-size: 0.8rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.moves-list {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.5rem;
		max-height: 400px;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.move-item {
		background: none;
		border: none;
		color: var(--text);
		text-align: left;
		padding: 0.3rem 0.5rem;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.85rem;
		transition: background 0.15s;
	}
	.move-item:hover   { background: var(--bg-input); }
	.move-item.active  { background: var(--accent); color: #000; font-weight: 600; }

	.actions {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
</style>
