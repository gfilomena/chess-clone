<script lang="ts">
	import { goto } from '$app/navigation';
	import { pendingInvite, acceptInvite, declineInvite } from '$lib/stores/invitations';

	let accepting = $state(false);
	let acceptError = $state('');

	async function handleAccept() {
		if (!$pendingInvite) return;
		accepting = true;
		acceptError = '';
		try {
			const gameId = await acceptInvite($pendingInvite.from_id);
			pendingInvite.set(null);
			goto(`/game/${gameId}`);
		} catch (err: any) {
			acceptError = err.message ?? 'Errore';
			accepting = false;
		}
	}

	async function handleDecline() {
		if (!$pendingInvite) return;
		await declineInvite($pendingInvite.from_id);
	}
</script>

{#if $pendingInvite}
	<div class="invite-toast" role="alert" aria-live="polite">
		<div class="invite-header">
			<span class="invite-icon">⚔️</span>
			<div class="invite-info">
				<span class="invite-name">{$pendingInvite.from_username}</span>
				<span class="invite-sub">ti sfida a Rapid 10' · ELO {$pendingInvite.from_elo}</span>
			</div>
		</div>

		{#if acceptError}
			<p class="invite-err">{acceptError}</p>
		{/if}

		<div class="invite-actions">
			<button class="btn btn-primary inv-btn" onclick={handleAccept} disabled={accepting}>
				{accepting ? '...' : 'Accetta'}
			</button>
			<button class="btn inv-btn decline-btn" onclick={handleDecline} disabled={accepting}>
				Rifiuta
			</button>
		</div>
	</div>
{/if}

<style>
	.invite-toast {
		position: fixed;
		bottom: 1.5rem;
		right: 1.5rem;
		background: var(--bg-card);
		border: 2px solid var(--accent);
		border-radius: 14px;
		padding: 1rem 1.25rem;
		box-shadow: 0 8px 40px rgba(0, 0, 0, 0.5);
		z-index: 9999;
		min-width: 290px;
		max-width: 340px;
		animation: slideIn 0.35s cubic-bezier(0.22, 1, 0.36, 1);
	}

	@keyframes slideIn {
		from {
			transform: translateX(120%);
			opacity: 0;
		}
		to {
			transform: translateX(0);
			opacity: 1;
		}
	}

	.invite-header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.85rem;
	}

	.invite-icon {
		font-size: 1.75rem;
		line-height: 1;
	}

	.invite-info {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}

	.invite-name {
		font-weight: 700;
		font-size: 1rem;
		color: var(--text);
	}

	.invite-sub {
		font-size: 0.8rem;
		color: var(--text-muted);
	}

	.invite-err {
		font-size: 0.8rem;
		color: #e74c3c;
		margin: 0 0 0.5rem;
	}

	.invite-actions {
		display: flex;
		gap: 0.5rem;
	}

	.inv-btn {
		flex: 1;
		padding: 0.5rem !important;
		font-size: 0.9rem !important;
		width: auto !important;
		border-radius: 8px !important;
	}

	.decline-btn {
		background: var(--border);
		color: var(--text);
	}

	.decline-btn:hover:not(:disabled) {
		background: #555;
	}
</style>
