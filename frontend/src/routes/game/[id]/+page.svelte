<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import Board from '$lib/chess/Board.svelte';
	import Timer from '$lib/chess/Timer.svelte';
	import { gameState, resetGame } from '$lib/stores/game';
	import { connectToGame, sendMove, sendResign, sendOfferDraw, sendDrawResponse, disconnect } from '$lib/ws/socket';
	import { user } from '$lib/stores/auth';

	const gameId = $page.params.id!;

	onMount(() => {
		resetGame();
		connectToGame(gameId);
	});

	onDestroy(() => {
		disconnect();
	});

	// Determinismo: è il mio turno?
	const isMyTurn = $derived(
		($gameState.status === 'active') &&
		(($gameState.playerColor === 'white' && $gameState.turn === 'w') ||
		 ($gameState.playerColor === 'black' && $gameState.turn === 'b'))
	);

	// Timer attivo per bianco/nero
	const isWhiteActive = $derived($gameState.status === 'active' && $gameState.turn === 'w');
	const isBlackActive = $derived($gameState.status === 'active' && $gameState.turn === 'b');

	// Ultima mossa per highlight
	let lastMove = $state<{ from: string; to: string } | null>(null);

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

	// Testo risultato finale
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
	<title>Partita — Chess Clone</title>
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
			</div>
			<Timer
				ms={$gameState.playerColor === 'white' ? $gameState.blackMs : $gameState.whiteMs}
				isActive={$gameState.playerColor === 'white' ? isBlackActive : isWhiteActive}
			/>
		</div>

		<!-- Scacchiera -->
		<div class="board-container">
			{#if $gameState.status === 'waiting'}
				<div class="overlay">
					<p>In attesa dell'avversario...</p>
				</div>
			{/if}

			{#if $gameState.status === 'finished'}
				<div class="overlay finished">
					<p class="result-text">{resultText($gameState.result, $gameState.finishReason)}</p>
					<a href="/" class="btn btn-primary" style="width:auto;margin-top:1rem">Nuova partita</a>
				</div>
			{/if}

			<Board
				fen={$gameState.fen}
				playerColor={$gameState.playerColor}
				{isMyTurn}
				{lastMove}
				onMove={handleMove}
			/>
		</div>

		<!-- Giocatore (in basso) -->
		<div class="player-row self">
			<div class="player-info">
				<span class="player-name">{$user?.username ?? 'Tu'}</span>
				<span class="player-elo">{$user?.elo_rapid ?? ''}</span>
			</div>
			<Timer
				ms={$gameState.playerColor === 'white' ? $gameState.whiteMs : $gameState.blackMs}
				isActive={$gameState.playerColor === 'white' ? isWhiteActive : isBlackActive}
			/>
		</div>
	</div>

	<!-- Colonna destra: mosse + azioni -->
	<div class="side-col">

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
</style>
