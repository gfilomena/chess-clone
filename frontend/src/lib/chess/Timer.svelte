<script lang="ts">
	let {
		ms,
		isActive,
		onFlag
	}: { ms: number; isActive: boolean; onFlag?: () => void } = $props();

	let displayMs = $state(0);
	let intervalId: ReturnType<typeof setInterval> | null = null;
	let flagSent = $state(false);

	// Sincronizza con il valore dal server ad ogni mossa; resetta il flag
	$effect(() => {
		displayMs = ms;
		flagSent = false;
	});

	// Countdown locale tra una mossa e l'altra
	$effect(() => {
		if (intervalId) clearInterval(intervalId);
		if (isActive) {
			intervalId = setInterval(() => {
				displayMs = Math.max(0, displayMs - 100);
				if (displayMs === 0 && !flagSent) {
					flagSent = true;
					onFlag?.();
				}
			}, 100);
		}
		return () => {
			if (intervalId) clearInterval(intervalId);
		};
	});

	function format(ms: number): string {
		const totalSecs = Math.ceil(ms / 1000);
		const mins = Math.floor(totalSecs / 60);
		const secs = totalSecs % 60;
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}

	const isLow = $derived(displayMs < 30_000);
</script>

<div class="timer" class:active={isActive} class:low={isLow}>
	{format(displayMs)}
</div>

<style>
	.timer {
		font-size: clamp(1rem, 2.2dvh, 1.4rem);
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		padding: 0.2rem 0.65rem;
		border-radius: 6px;
		background: var(--bg-card);
		border: 2px solid var(--border);
		color: var(--text-muted);
		min-width: 72px;
		text-align: center;
		transition: border-color 0.3s, color 0.3s;
		line-height: 1.4;
	}
	.timer.active {
		border-color: var(--accent);
		color: var(--text);
	}
	.timer.low {
		border-color: var(--danger);
		color: var(--danger);
		animation: pulse 1s ease-in-out infinite;
	}
	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.6; }
	}
</style>
