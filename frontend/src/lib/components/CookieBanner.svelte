<script lang="ts">
	import { consent, acceptAll, acceptEssential } from '$lib/stores/consent';
	import { t } from '$lib/i18n';
</script>

{#if $consent === 'pending'}
	<div class="cookie-bar" role="region" aria-label="Cookie consent">
		<p class="cookie-text">
			{$t.cookie.banner_desc}
			<a href="/privacy" class="cookie-link">{$t.cookie.privacy_link}</a>
		</p>
		<div class="cookie-actions">
			<button class="btn-essential" onclick={acceptEssential}>
				{$t.cookie.essential_only}
			</button>
			<button class="btn-accept" onclick={acceptAll}>
				{$t.cookie.accept_all}
			</button>
		</div>
	</div>
{/if}

<style>
	.cookie-bar {
		position: fixed;
		bottom: 0;
		left: 0;
		right: 0;
		z-index: 500;
		background: var(--bg-card);
		border-top: 1px solid var(--border);
		padding: 0.85rem 1.25rem;
		display: flex;
		align-items: center;
		gap: 1rem;
		flex-wrap: wrap;
		box-shadow: 0 -4px 20px rgba(0, 0, 0, 0.4);
	}

	.cookie-text {
		flex: 1;
		min-width: 220px;
		font-size: 0.82rem;
		color: var(--text-muted);
		line-height: 1.5;
		margin: 0;
	}

	.cookie-link {
		color: var(--accent);
		text-decoration: underline;
		white-space: nowrap;
	}

	.cookie-actions {
		display: flex;
		gap: 0.5rem;
		flex-shrink: 0;
	}

	.btn-essential {
		background: none;
		border: 1px solid var(--border);
		color: var(--text-muted);
		border-radius: var(--radius);
		padding: 0.45rem 0.9rem;
		font-size: 0.82rem;
		font-weight: 500;
		cursor: pointer;
		transition: border-color 0.15s, color 0.15s;
		white-space: nowrap;
	}
	.btn-essential:hover {
		border-color: var(--text-muted);
		color: var(--text);
	}

	.btn-accept {
		background: var(--accent);
		border: none;
		color: #000;
		border-radius: var(--radius);
		padding: 0.45rem 0.9rem;
		font-size: 0.82rem;
		font-weight: 700;
		cursor: pointer;
		transition: background 0.15s;
		white-space: nowrap;
	}
	.btn-accept:hover {
		background: var(--accent-dark);
		color: #fff;
	}

	@media (max-width: 600px) {
		.cookie-bar {
			flex-direction: column;
			align-items: stretch;
			gap: 0.75rem;
		}
		.cookie-actions {
			justify-content: flex-end;
		}
	}
</style>
