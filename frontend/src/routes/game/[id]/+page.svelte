<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { Chess } from 'chess.js';
	import Board from '$lib/chess/Board.svelte';
	import Timer from '$lib/chess/Timer.svelte';
	import { gameState, resetGame } from '$lib/stores/game';
	import { connectToGame, sendMove, sendResign, sendOfferDraw, sendDrawResponse, disconnect } from '$lib/ws/socket';
	import { user } from '$lib/stores/auth';
	import { initSounds, playSound, type SoundName } from '$lib/chess/sounds';
	import { computeCaptured } from '$lib/chess/captured';

	const gameId = $page.params.id!;

	let panelOpen = $state(false);

	// ── Move navigation ─────────────────────────────────────────────
	interface HistoryEntry {
		fen: string;
		move: { from: string; to: string } | null;
		sound: SoundName | null;
		san:   string | null;
	}

	function buildHistory(pgn: string): HistoryEntry[] {
		const entries: HistoryEntry[] = [{ fen: new Chess().fen(), move: null, sound: null, san: null }];
		if (!pgn) return entries;
		try {
			const temp = new Chess();
			temp.loadPgn(pgn);
			const moves = temp.history({ verbose: true }) as any[];
			const replay = new Chess();
			for (const mv of moves) {
				replay.move(mv.san);
				const inCheck   = replay.inCheck();
				const isCapture = mv.flags.includes('c') || mv.flags.includes('e');
				const sound: SoundName = inCheck ? 'check' : isCapture ? 'capture' : 'move';
				entries.push({ fen: replay.fen(), move: { from: mv.from, to: mv.to }, sound, san: mv.san });
			}
		} catch {}
		return entries;
	}

	let viewIndex = $state<number | null>(null); // null = live
	let stripEl   = $state<HTMLElement | null>(null);

	const history = $derived(buildHistory($gameState.pgn));

	const displayFen = $derived(
		viewIndex === null ? $gameState.fen : (history[viewIndex]?.fen ?? $gameState.fen)
	);

	const isReviewing     = $derived(viewIndex !== null);
	const atStart         = $derived(viewIndex === 0);
	const atEnd           = $derived(viewIndex === null);
	const timelinePercent = $derived(
		history.length <= 1 || viewIndex === null
			? 100
			: Math.round((viewIndex / (history.length - 1)) * 100)
	);

	const navLabel = $derived(
		viewIndex === null
			? 'Live'
			: `${viewIndex} / ${history.length - 1}`
	);

	function playNavSound(idx: number) {
		const s = history[idx]?.sound;
		if (s) playSound(s);
	}

	function navTo(idx: number) {
		if (idx <= 0)                   { viewIndex = 0; }
		else if (idx >= history.length - 1) { viewIndex = null; }
		else { viewIndex = idx; playNavSound(idx); }
	}
	function navFirst() { viewIndex = 0; }
	function navPrev() {
		const idx = viewIndex ?? history.length - 1;
		if (idx > 0) { viewIndex = idx - 1; playNavSound(idx - 1); }
	}
	function navNext() {
		if (viewIndex === null) return;
		if (viewIndex < history.length - 1) { viewIndex++; playNavSound(viewIndex); }
		else viewIndex = null;
	}
	function navLast() { viewIndex = null; }

	// Auto-scroll strip al chip attivo
	$effect(() => {
		const idx = viewIndex ?? history.length - 1;
		if (!stripEl) return;
		(stripEl.children[idx] as HTMLElement | undefined)
			?.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
	});

	// Se l'avversario muove mentre si è in revisione → torna a live
	let prevHistLen = 0;
	$effect(() => {
		const len = history.length;
		if (len > prevHistLen && prevHistLen > 0 && isReviewing) viewIndex = null;
		prevHistLen = len;
	});

	// ───────────────────────────────────────────────────────────────

	onMount(() => {
		initSounds();
		resetGame();
		connectToGame(gameId);
	});

	onDestroy(() => {
		disconnect();
	});

	// Auto-apri il pannello quando arriva un'offerta di patta
	$effect(() => {
		if ($gameState.drawOffered) panelOpen = true;
	});

	const isMyTurn = $derived(
		($gameState.status === 'active') &&
		(($gameState.playerColor === 'white' && $gameState.turn === 'w') ||
		 ($gameState.playerColor === 'black' && $gameState.turn === 'b'))
	);

	// In modalità revisione il board non accetta mosse
	const canMove = $derived(!isReviewing && isMyTurn);

	// ── Pezzi catturati ─────────────────────────────────────────────
	const captured   = $derived(computeCaptured(displayFen));
	const myCaptured = $derived($gameState.playerColor === 'white' ? captured.byWhite : captured.byBlack);
	const oppCaptured= $derived($gameState.playerColor === 'white' ? captured.byBlack : captured.byWhite);
	const myAdv      = $derived($gameState.playerColor === 'white' ? captured.advantage : -captured.advantage);
	const oppAdv     = $derived(-myAdv);

	const isWhiteActive = $derived($gameState.status === 'active' && $gameState.turn === 'w');
	const isBlackActive = $derived($gameState.status === 'active' && $gameState.turn === 'b');

	let lastMove = $state<{ from: string; to: string } | null>(null);

	// Evidenzia la mossa corrente nella navigazione storica
	const displayLastMove = $derived(
		viewIndex === null ? lastMove : (history[viewIndex]?.move ?? null)
	);

	function handleMove(from: string, to: string, promotion?: string) {
		lastMove = { from, to };
		sendMove(from, to, promotion);
	}

	function handleResign() {
		if (confirm('Sei sicuro di voler abbandonare?')) {
			sendResign();
		}
	}

	function handleDrawOffer() {
		sendOfferDraw();
	}

	function handleDrawResponse(accepted: boolean) {
		sendDrawResponse(accepted);
		gameState.update(s => ({ ...s, drawOffered: false }));
	}

	function resultText(result: string | null, reason: string | null): string {
		if (!result) return '';
		const who = result === 'draw' ? 'Patta' :
			result === $gameState.playerColor ? 'Hai vinto!' : 'Hai perso';
		const why: Record<string, string> = {
			checkmate: 'scacco matto',
			timeout: 'tempo scaduto',
			resigned: 'abbandono',
			stalemate: 'stallo',
			fifty_moves: 'regola 50 mosse',
			threefold: 'ripetizione',
			abandoned: 'abbandono avversario',
			draw_agreed: 'patta concordata'
		};
		return `${who} — ${why[reason ?? ''] ?? reason}`;
	}
</script>

<svelte:head>
	<title>Partita — Chess</title>
</svelte:head>

<div class="game-layout">

	<!-- Colonna sinistra: info avversario + scacchiera + info giocatore -->
	<div class="board-col">

		<!-- Avversario (in alto) -->
		<div class="player-row opponent">
			<div class="player-info">
				<span class="player-name">
					{$gameState.playerColor === 'white' ? 'Nero' : 'Bianco'}
				</span>
				{#if oppCaptured.length > 0 || oppAdv > 0}
					<div class="captured-row">
						{#each oppCaptured as p}<span class="cap-piece">{p}</span>{/each}
						{#if oppAdv > 0}<span class="cap-adv">+{oppAdv}</span>{/if}
					</div>
				{/if}
			</div>
			<Timer
				ms={$gameState.playerColor === 'white' ? $gameState.blackMs : $gameState.whiteMs}
				isActive={$gameState.playerColor === 'white' ? isBlackActive : isWhiteActive}
			/>
		</div>

		<!-- Striscia mosse (solo mobile) -->
		<div class="mobile-moves-strip" bind:this={stripEl}>
			{#each history as entry, i}
				{@const isActive = (viewIndex ?? history.length - 1) === i}
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

		<!-- Scacchiera -->
		<div class="board-container">
			{#if $gameState.status === 'waiting'}
				<div class="overlay">
					<p>In attesa dell'avversario...</p>
				</div>
			{/if}

			{#if $gameState.status === 'finished' && !isReviewing}
				<div class="overlay finished">
					<p class="result-text">{resultText($gameState.result, $gameState.finishReason)}</p>
					<a href="/" class="btn btn-primary" style="width:auto;margin-top:1rem">Nuova partita</a>
				</div>
			{/if}

			<Board
				fen={displayFen}
				playerColor={$gameState.playerColor}
				isMyTurn={canMove}
				lastMove={displayLastMove}
				onMove={handleMove}
			/>
		</div>

		<!-- Nav bar timeline (solo mobile) -->
		<div class="mobile-nav-bar">
			<button class="nav-btn" onclick={navFirst} disabled={atStart} title="Prima mossa">⏮</button>
			<button class="nav-btn" onclick={navPrev}  disabled={atStart} title="Mossa precedente">◀</button>
			<div class="nav-timeline">
				<div class="timeline-track">
					<div class="timeline-fill" style="width:{timelinePercent}%">
						<span class="timeline-thumb"></span>
					</div>
				</div>
				<span class="timeline-label" class:live={!isReviewing}>{navLabel}</span>
			</div>
			<button class="nav-btn" onclick={navNext}  disabled={atEnd} title="Mossa successiva">▶</button>
			<button class="nav-btn" onclick={navLast}  disabled={atEnd} title="Ultima mossa">⏭</button>
		</div>

		<!-- Giocatore (in basso) -->
		<div class="player-row self">
			<div class="player-info">
				<span class="player-name">{$user?.username ?? 'Tu'}</span>
				<span class="player-elo">{$user?.elo_rapid ?? ''}</span>
				{#if myCaptured.length > 0 || myAdv > 0}
					<div class="captured-row">
						{#each myCaptured as p}<span class="cap-piece">{p}</span>{/each}
						{#if myAdv > 0}<span class="cap-adv">+{myAdv}</span>{/if}
					</div>
				{/if}
			</div>
			<Timer
				ms={$gameState.playerColor === 'white' ? $gameState.whiteMs : $gameState.blackMs}
				isActive={$gameState.playerColor === 'white' ? isWhiteActive : isBlackActive}
			/>
		</div>

		<!-- Pulsante toggle pannello (solo mobile) -->
		<button class="panel-toggle" onclick={() => panelOpen = !panelOpen}>
			{#if $gameState.drawOffered}
				🤝 Offerta patta!
			{:else if panelOpen}
				✕ Chiudi
			{:else}
				📋 Mosse & Azioni
			{/if}
		</button>
	</div>

	<!-- Backdrop pannello (mobile) -->
	<div
		class="panel-backdrop"
		class:panel-open={panelOpen}
		onclick={() => panelOpen = false}
		aria-hidden="true"
	></div>

	<!-- Colonna destra: mosse + azioni -->
	<div class="side-col" class:panel-open={panelOpen}>

		<!-- Handle + header (solo mobile) -->
		<div class="panel-drag-handle"></div>
		<div class="panel-header">
			<span>Mosse & Azioni</span>
			<button class="panel-close" onclick={() => panelOpen = false}>✕</button>
		</div>

		<!-- Offerta patta ricevuta -->
		{#if $gameState.drawOffered}
			<div class="draw-offer">
				<p>L'avversario offre patta</p>
				<div style="display:flex;gap:0.5rem;margin-top:0.5rem">
					<button class="btn btn-primary" style="flex:1" onclick={() => handleDrawResponse(true)}>
						Accetta
					</button>
					<button class="btn btn-google" style="flex:1" onclick={() => handleDrawResponse(false)}>
						Rifiuta
					</button>
				</div>
			</div>
		{/if}

		<!-- Lista mosse (PGN semplificato) -->
		<div class="moves-panel">
			<h3>Mosse</h3>
			<div class="pgn-text">
				{$gameState.pgn || '—'}
			</div>
		</div>

		<!-- Azioni -->
		{#if $gameState.status === 'active'}
			<div class="actions">
				<button class="btn btn-google" onclick={handleDrawOffer} style="width:100%">
					Offri patta
				</button>
				<button class="btn" style="background:var(--danger);color:#fff;width:100%" onclick={handleResign}>
					Abbandona
				</button>
			</div>
		{/if}

		<!-- Navigazione mosse -->
		<div class="nav-row" class:reviewing={isReviewing}>
			<button class="nav-btn" onclick={navFirst} disabled={atStart} title="Prima mossa">⏮</button>
			<button class="nav-btn" onclick={navPrev}  disabled={atStart} title="Mossa precedente">◀</button>
			<span class="nav-label" class:live={!isReviewing}>{navLabel}</span>
			<button class="nav-btn" onclick={navNext}  disabled={atEnd}   title="Mossa successiva">▶</button>
			<button class="nav-btn" onclick={navLast}  disabled={atEnd}   title="Ultima mossa">⏭</button>
		</div>

		<!-- Status partita -->
		<div class="status-badge" class:active={$gameState.status === 'active'}>
			{#if $gameState.status === 'waiting'}
				⏳ In attesa...
			{:else if $gameState.status === 'active'}
				{isMyTurn ? '🟢 Tocca a te' : '⏳ Aspetta...'}
			{:else if $gameState.status === 'finished'}
				🏁 Partita terminata
			{/if}
		</div>
	</div>
</div>

<style>
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
		font-size: 0.82rem;
		line-height: 1;
		opacity: 0.8;
	}
	.cap-adv {
		font-size: 0.68rem;
		font-weight: 700;
		color: var(--accent);
		margin-left: 0.2rem;
	}

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
	}

	/* ── Panel toggle (default: nascosto, visibile solo mobile) ── */
	.panel-toggle {
		display: none;
	}

	/* ── Backdrop pannello mobile ── */
	.panel-backdrop {
		display: none;
	}

	/* ── Side column ── */
	.side-col {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		width: 240px;
		padding-top: 3rem;
	}

	/* ── Panel handle + header (nascosti su desktop) ── */
	.panel-drag-handle { display: none; }
	.panel-header      { display: none; }
	.panel-close       { display: none; }

	.moves-panel {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 1rem;
		flex: 1;
	}

	.moves-panel h3 {
		margin-bottom: 0.75rem;
		color: var(--text-muted);
		font-size: 0.85rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.pgn-text {
		font-size: 0.85rem;
		line-height: 1.6;
		color: var(--text);
		word-break: break-all;
	}

	.actions {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.draw-offer {
		background: var(--bg-card);
		border: 1px solid var(--accent);
		border-radius: 8px;
		padding: 1rem;
		font-size: 0.9rem;
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

	/* ═══════════════════════════════════════════════════════════
	   MOBILE (≤ 768px)
	   ═══════════════════════════════════════════════════════════ */
	@media (max-width: 768px) {
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
			gap: 0.5rem;
		}

		/* Le righe player si allineano alla larghezza della board */
		.player-row {
			width: min(calc(100vw - 1rem), calc(100vh - 220px));
		}

		/* Pulsante toggle pannello — visibile su mobile */
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

		/* Backdrop semitrasparente sotto il pannello */
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

		/* Handle di trascinamento visivo */
		.panel-drag-handle {
			display: block;
			width: 36px;
			height: 4px;
			background: var(--border);
			border-radius: 2px;
			margin: 0.8rem auto 0.4rem;
			flex-shrink: 0;
		}

		/* Header del pannello con titolo e tasto chiudi */
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

		/* Moves panel: altezza automatica, niente flex:1 */
		.moves-panel {
			flex: none;
			max-height: 220px;
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

		/* Nascondi nav-row nel side-col su mobile (già nella mobile-nav-bar) */
		.side-col .nav-row { display: none; }
	}
</style>
