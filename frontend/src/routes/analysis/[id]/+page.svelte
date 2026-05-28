<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { Chess } from 'chess.js';
	import Board from '$lib/chess/Board.svelte';
	import { StockfishEngine, evalToPercent, formatScore, classifyMove, type AnalysisResult, type MoveClassification } from '$lib/chess/stockfish';
	import { API_URL as API } from '$lib/config';
	import { t } from '$lib/i18n';

	const gameId = $page.params.id;

	// ── Stato base ─────────────────────────────────────────────────────────
	let game      = $state<any>(null);
	let positions = $state<string[]>([]);    // FEN per ogni posizione
	let moveLabels= $state<string[]>([]);    // "1. e4", "1... e5" …
	let moveUcis  = $state<string[]>([]);    // mossa UCI da pos[i] → pos[i+1]
	let currentIdx= $state(0);
	let analysis  = $state<AnalysisResult | null>(null);
	let analyzing = $state(false);
	let engine: StockfishEngine | null = null;
	let loading   = $state(true);
	let error     = $state('');

	// ── Revisione ─────────────────────────────────────────────────────────
	let reviewMode    = $state(false);
	let reviewRunning = $state(false);
	let reviewProgress= $state(0);
	let reviewTotal   = $state(0);
	let reviewData    = $state<Array<{ score: number; bestMove: string }>>([]);
	let reviewDone    = $state(false);

	// ── Mount ──────────────────────────────────────────────────────────────
	onMount(async () => {
		try {
			const res  = await fetch(`${API}/api/games/${gameId}`);
			const json = await res.json();
			if (!json.success) throw new Error('Partita non trovata');
			game = json.data;
			parsePositions(game.pgn);
			engine = new StockfishEngine();
			await engine.init();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
		await analyzeCurrentPosition();
	});

	onDestroy(() => engine?.destroy());

	// ── Parser PGN ─────────────────────────────────────────────────────────
	function parsePositions(pgn: string) {
		const chess = new Chess();
		if (pgn) { try { chess.loadPgn(pgn); } catch {} }

		const history = chess.history({ verbose: true }) as any[];
		const fens: string[]   = [];
		const labels: string[] = [];
		const ucis: string[]   = [];

		const replay = new Chess();
		fens.push(replay.fen());
		labels.push('Inizio');

		for (let i = 0; i < history.length; i++) {
			const mv = history[i];
			replay.move(mv);
			fens.push(replay.fen());
			const n = Math.floor(i / 2) + 1;
			labels.push(i % 2 === 0 ? `${n}. ${mv.san}` : `${n}... ${mv.san}`);
			ucis.push(`${mv.from}${mv.to}${mv.promotion ?? ''}`);
		}

		positions  = fens;
		moveLabels = labels;
		moveUcis   = ucis;
	}

	// ── Navigazione ────────────────────────────────────────────────────────
	async function goTo(idx: number) {
		if (idx < 0 || idx >= positions.length) return;
		engine?.stop();
		currentIdx = idx;
		if (!reviewMode) {
			analysis = null;
			await analyzeCurrentPosition();
		}
	}

	function goFirst() { goTo(0); }
	function goPrev()  { goTo(currentIdx - 1); }
	function goNext()  { goTo(currentIdx + 1); }
	function goLast()  { goTo(positions.length - 1); }

	// ── Analisi live ───────────────────────────────────────────────────────
	async function analyzeCurrentPosition() {
		if (!engine || !positions[currentIdx]) return;
		analyzing = true;
		try {
			analysis = await engine.analyze(positions[currentIdx], 16);
		} finally {
			analyzing = false;
		}
	}

	// ── Revisione batch ────────────────────────────────────────────────────
	async function startReview() {
		if (!engine || reviewRunning) return;
		engine.stop();
		reviewMode     = true;
		reviewRunning  = true;
		reviewProgress = 0;
		reviewTotal    = positions.length;
		reviewDone     = false;
		reviewData     = [];
		analysis       = null;

		try {
			reviewData = await engine.analyzeAll(positions, 14, (done, total) => {
				reviewProgress = done;
				reviewTotal    = total;
			});
			reviewDone = true;
		} finally {
			reviewRunning = false;
		}
	}

	function exitReview() {
		reviewMode = false;
		analyzeCurrentPosition();
	}

	// ── Tastiera ──────────────────────────────────────────────────────────
	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowLeft')  goPrev();
		if (e.key === 'ArrowRight') goNext();
		if (e.key === 'ArrowUp')    goFirst();
		if (e.key === 'ArrowDown')  goLast();
	}

	// ── Derived: eval live ─────────────────────────────────────────────────
	const evalPercent = $derived(analysis ? evalToPercent(analysis) : 50);
	const evalText    = $derived(analysis ? formatScore(analysis) : '...');

	// ── Derived: revisione ─────────────────────────────────────────────────
	/** Classificazione di ogni mossa: moveClassifications[i] = mossa da pos[i]→pos[i+1] */
	const moveClassifications = $derived(
		reviewDone && reviewData.length >= positions.length
			? moveUcis.map((uci, i) => {
				const before = reviewData[i];
				const after  = reviewData[i + 1];
				if (!before || !after) return null as MoveClassification | null;
				const whiteToMove = positions[i].split(' ')[1] === 'w';
				return classifyMove(before.score, after.score, whiteToMove, uci, before.bestMove);
			})
			: ([] as (MoveClassification | null)[])
	);

	/** Classificazione della mossa che ha portato alla posizione corrente */
	const currentClassification = $derived(
		reviewDone && currentIdx > 0
			? (moveClassifications[currentIdx - 1] ?? null)
			: null
	);

	/** Frecce sul board */
	const boardArrows = $derived((() => {
		if (reviewMode && reviewDone) {
			if (currentIdx === 0) return [];

			const played = moveUcis[currentIdx - 1] ?? '';
			const bm     = reviewData[currentIdx - 1]?.bestMove ?? '';
			const isBest = bm.length >= 4 && bm !== '(none)' && played.slice(0, 4) === bm.slice(0, 4);

			// solo freccia verde motore (se diversa dalla mossa giocata)
			if (!isBest && bm.length >= 4 && bm !== '(none)') {
				return [{
					from:  bm.slice(0, 2),
					to:    bm.slice(2, 4),
					color: 'rgba(100,190,100,0.9)'
				}];
			}

			return [];
		}
		return [];
	})());

	/** Eval bar: in revisione usa il punteggio batch */
	const reviewEvalPercent = $derived(
		reviewDone && reviewData[currentIdx] !== undefined
			? 50 + 50 * Math.tanh(reviewData[currentIdx].score / 400)
			: 50
	);
	const reviewEvalText = $derived(
		reviewDone && reviewData[currentIdx] !== undefined
			? (() => {
				const d = reviewData[currentIdx];
				const pawns = d.score / 100;
				return (pawns >= 0 ? '+' : '') + pawns.toFixed(1);
			})()
			: '...'
	);

	const displayEvalPercent = $derived(reviewMode ? reviewEvalPercent : evalPercent);
	const displayEvalText    = $derived(reviewMode ? reviewEvalText    : evalText);

	function resultBadge(result: string | null): string {
		if (!result) return '';
		return ({
			white: $t.result.white_wins,
			black: $t.result.black_wins,
			draw:  $t.result.draw
		} as Record<string,string>)[result] ?? result;
	}

	/** Badge classificazione sull'angolo in alto a destra del pezzo mosso */
	const squareBadge = $derived(
		reviewDone && currentIdx > 0 && currentClassification
			? {
				square: moveUcis[currentIdx - 1].slice(2, 4),
				symbol: currentClassification.symbol,
				color:  currentClassification.color
			}
			: null
	);

	/** Percentuale barra progresso */
	const progressPct = $derived(reviewTotal > 0 ? Math.round((reviewProgress / reviewTotal) * 100) : 0);

	/** Summary classificazioni con split bianco/nero */
	const reviewSummary = $derived(
		reviewDone
			? moveClassifications.reduce((acc, c, i) => {
				if (c) {
					const isWhite = positions[i].split(' ')[1] === 'w';
					if (!acc[c.key]) acc[c.key] = { white: 0, black: 0 };
					if (isWhite) acc[c.key].white++;
					else         acc[c.key].black++;
				}
				return acc;
			}, {} as Record<string, { white: number; black: number }>)
			: {}
	);
</script>

<svelte:head>
	<title>Chess</title>
</svelte:head>

<svelte:window onkeydown={handleKeydown} />

{#if loading}
	<div class="center"><p>{$t.analysis.loading}</p></div>
{:else if error}
	<div class="center"><p class="error-msg">{error}</p></div>
{:else}
<div class="analysis-layout">

	<!-- Scacchiera + eval bar -->
	<div class="board-col">
		<div class="game-info">
			<span class="player-tag black-tag">♟ {game.black_username} ({game.black_elo})</span>
			<span class="result-tag">{resultBadge(game.result)}</span>
			<span class="player-tag white-tag">♔ {game.white_username} ({game.white_elo})</span>
		</div>

		<div class="board-row">
			<!-- Eval bar verticale — stessa altezza della scacchiera -->
			<div class="eval-bar" title="{displayEvalText}">
				<div class="eval-black" style="height: {100 - displayEvalPercent}%"></div>
				<div class="eval-white" style="height: {displayEvalPercent}%"></div>
				<span class="eval-label" style="bottom:{displayEvalPercent}%">
					{displayEvalText}
				</span>
			</div>

			<Board
				fen={positions[currentIdx] ?? 'rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1'}
				playerColor="white"
				isMyTurn={false}
				lastMove={null}
				onMove={() => {}}
				arrows={boardArrows}
				squareBadge={squareBadge}
			/>
		</div>

		<!-- Navigazione -->
		<div class="nav-controls">
			<button onclick={goFirst} title="Prima mossa">⏮</button>
			<button onclick={goPrev}  title="Mossa precedente (←)">◀</button>
			<span class="move-counter">{currentIdx} / {positions.length - 1}</span>
			<button onclick={goNext}  title="Mossa successiva (→)">▶</button>
			<button onclick={goLast}  title="Ultima mossa">⏭</button>
		</div>

		<!-- Info engine / classifica mossa corrente -->
		{#if reviewMode}
			{#if currentClassification}
				<div class="move-badge-bar" style="--clr:{currentClassification.color}">
					<span class="badge-symbol">{currentClassification.symbol}</span>
					<span class="badge-label">{($t.classifications as any)[currentClassification.key]}</span>
					{#if currentClassification.delta > 5}
						<span class="badge-delta">−{(currentClassification.delta / 100).toFixed(1)} {$t.analysis.loss_pts}</span>
					{/if}
					{#if reviewData[currentIdx - 1]?.bestMove && reviewData[currentIdx - 1].bestMove !== '(none)'}
						{@const bm = reviewData[currentIdx - 1].bestMove}
						{@const played = moveUcis[currentIdx - 1] ?? ''}
						{#if bm.slice(0,4) !== played.slice(0,4)}
							<span class="badge-best">
								{$t.analysis.best_move}: {bm.slice(0,2)}→{bm.slice(2,4)}
							</span>
						{/if}
					{/if}
				</div>
			{:else if currentIdx === 0}
				<div class="move-badge-bar neutral">{$t.analysis.initial_pos}</div>
			{/if}
		{:else}
			<div class="engine-info">
				{#if analyzing}
					<span class="analyzing">{$t.analysis.engine_analyzing}</span>
				{:else if analysis}
					<span>{$t.analysis.depth} {analysis.depth}</span>
					<span class="score-text" class:positive={analysis.score > 0} class:negative={analysis.score < 0}>
						{evalText}
					</span>
					{#if analysis.bestMove && analysis.bestMove !== '(none)'}
						<span>↗ <strong>{analysis.bestMove.slice(0,2)}→{analysis.bestMove.slice(2,4)}</strong></span>
					{/if}
				{/if}
			</div>
		{/if}
	</div>

	<!-- Pannello mosse + azioni -->
	<div class="moves-col">
		<h3>{$t.analysis.moves}</h3>
		<div class="moves-list">
			{#each moveLabels as label, i}
				{@const cls = i > 0 ? (moveClassifications[i - 1] ?? null) : null}
				<button
					class="move-item"
					class:active={i === currentIdx}
					onclick={() => goTo(i)}
				>
					<span class="move-label-text">{label}</span>
					{#if cls}
						<span
							class="move-cls-badge"
							class:has-symbol={!!cls.symbol}
							style="color:{cls.color}"
							title="{cls.label}{cls.delta > 5 ? ' (−' + (cls.delta/100).toFixed(1) + ')' : ''}"
						>{cls.symbol || '·'}</span>
					{/if}
				</button>
			{/each}
		</div>

		<!-- Tabellino revisione -->
		{#if reviewDone && Object.keys(reviewSummary).length > 0}
			<div class="review-summary">
				<div class="summary-header">
					<span class="summary-sym"></span>
					<span class="summary-lbl"></span>
					<span class="summary-player-head">♔</span>
					<span class="summary-player-head">♟</span>
				</div>
				{#each [
					{ key:'best',       symbol:'!!', color:'#5B8E55' },
					{ key:'excellent',  symbol:'!',  color:'#81B64C' },
					{ key:'good',       symbol:'',   color:'#5080C0' },
					{ key:'inaccuracy', symbol:'?!', color:'#C9A020' },
					{ key:'mistake',    symbol:'?',  color:'#D97706' },
					{ key:'blunder',    symbol:'??', color:'#DC2626' },
				] as row}
					{@const counts = reviewSummary[row.key]}
					{#if counts && (counts.white + counts.black) > 0}
						<div class="summary-row">
							<span class="summary-sym" style="color:{row.color}">{row.symbol || '·'}</span>
							<span class="summary-lbl">{($t.classifications as any)[row.key]}</span>
							<span class="summary-cnt">{counts.white}</span>
							<span class="summary-cnt">{counts.black}</span>
						</div>
					{/if}
				{/each}
			</div>
		{/if}

		<!-- Sezione revisione -->
		<div class="review-section">
			{#if reviewRunning}
				<div class="review-progress">
					<div class="progress-bar">
						<div class="progress-fill" style="width:{progressPct}%"></div>
					</div>
					<span class="progress-label">{$t.analysis.progress(reviewProgress, reviewTotal)}</span>
				</div>
			{:else if reviewDone}
				<button class="btn btn-google" style="width:100%;font-size:0.8rem" onclick={exitReview}>
					{$t.analysis.back_live}
				</button>
			{:else}
				<button class="btn btn-primary" style="width:100%;font-size:0.88rem" onclick={startReview}>
					{$t.analysis.start_review}
				</button>
			{/if}
		</div>

		<div class="actions">
			<a
				href={`${API}/api/games/${gameId}/pgn`}
				class="btn btn-google"
				style="text-align:center"
			>{$t.analysis.download_pgn}</a>
			<a href="/" class="btn btn-primary" style="text-align:center">{$t.analysis.new_game}</a>
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

	/* ── Board column ── */
	.board-col {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	/* Riga che contiene eval-bar + scacchiera, allineati in altezza */
	.board-row {
		display: flex;
		gap: 0.5rem;
		align-items: stretch;
	}

	/* ── Eval bar — stessa altezza della scacchiera (stretch) ── */
	.eval-bar {
		width: 36px;
		flex-shrink: 0;
		border-radius: 4px;
		overflow: hidden;
		position: relative;
		border: 1px solid var(--border);
		display: flex;
		flex-direction: column;
	}
	.eval-black { background: #1a1a1a; transition: height 0.4s; }
	.eval-white { background: #f0d9b5; transition: height 0.4s; }
	.eval-label {
		position: absolute;
		left: 50%;
		transform: translateX(-50%);
		font-size: 0.68rem;
		font-weight: 600;
		color: var(--text-muted);
		white-space: nowrap;
	}

	.game-info {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.85rem;
	}
	.player-tag { font-weight: 600; padding: 0.2rem 0.5rem; border-radius: 4px; }
	.black-tag  { background: #2d3748; }
	.white-tag  { background: #4a5568; }
	.result-tag { color: var(--accent); font-weight: 700; }

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

	/* ── Engine info (live) ── */
	.engine-info {
		display: flex;
		gap: 1rem;
		align-items: center;
		font-size: 0.85rem;
		color: var(--text-muted);
		justify-content: center;
		min-height: 36px;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.4rem 0.8rem;
	}
	.analyzing   { color: var(--accent); }
	.score-text  { font-weight: 700; }
	.score-text.positive { color: #f0d9b5; }
	.score-text.negative { color: #888; }

	/* ── Badge mossa corrente (revisione) ── */
	.move-badge-bar {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.5rem 0.9rem;
		background: var(--bg-card);
		border: 1px solid var(--clr, var(--border));
		border-radius: 8px;
		min-height: 36px;
		font-size: 0.85rem;
	}
	.move-badge-bar.neutral {
		color: var(--text-muted);
		border-color: var(--border);
	}
	.badge-symbol {
		font-weight: 800;
		font-size: 0.95rem;
		color: var(--clr, var(--text));
		min-width: 1.4rem;
		text-align: center;
	}
	.badge-label {
		font-weight: 600;
		color: var(--clr, var(--text));
	}
	.badge-delta {
		font-size: 0.75rem;
		color: var(--text-muted);
	}
	.badge-best {
		font-size: 0.75rem;
		color: var(--text-muted);
		font-family: monospace;
	}

	/* ── Moves column ── */
	.moves-col {
		width: 210px;
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
		max-height: 360px;
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
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.3rem;
	}
	.move-item:hover  { background: var(--bg-input); }
	.move-item.active { background: var(--accent); color: #000; font-weight: 600; }
	.move-item.active .move-cls-badge { color: #000 !important; }

	.move-label-text { flex: 1; }
	.move-cls-badge {
		font-weight: 700;
		font-size: 0.72rem;
		letter-spacing: 0.02em;
		flex-shrink: 0;
		min-width: 1.2rem;
		text-align: right;
	}

	/* ── Review summary ── */
	.review-summary {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.6rem 0.75rem;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.summary-header,
	.summary-row {
		display: grid;
		grid-template-columns: 1.6rem 1fr 1.8rem 1.8rem;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.8rem;
	}
	.summary-header {
		border-bottom: 1px solid var(--border);
		padding-bottom: 0.25rem;
		margin-bottom: 0.1rem;
	}
	.summary-player-head {
		font-size: 0.75rem;
		color: var(--text-muted);
		text-align: center;
	}
	.summary-sym { font-weight: 800; }
	.summary-lbl { color: var(--text-muted); }
	.summary-cnt { font-weight: 700; text-align: center; }

	/* ── Progress bar ── */
	.review-section {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}
	.review-progress { display: flex; flex-direction: column; gap: 0.4rem; }
	.progress-bar {
		width: 100%;
		height: 6px;
		background: var(--border);
		border-radius: 3px;
		overflow: hidden;
	}
	.progress-fill {
		height: 100%;
		background: var(--accent);
		border-radius: 3px;
		transition: width 0.2s ease;
	}
	.progress-label {
		font-size: 0.75rem;
		color: var(--text-muted);
		text-align: center;
	}

	.actions {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	/* ── Mobile ── */
	@media (max-width: 768px) {
		.analysis-layout {
			flex-direction: column;
			padding: 0.75rem;
			align-items: center;
		}
		.eval-bar   { display: none; }
		.moves-col  { width: 100%; padding-top: 0; max-width: 480px; }
		.moves-list { max-height: 200px; }
	}
</style>
