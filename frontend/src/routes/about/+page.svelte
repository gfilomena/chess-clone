<script lang="ts">
	import { t } from '$lib/i18n';
</script>

<svelte:head>
	<title>{$t.nav.about} — Chess</title>
	<meta name="description" content="Chess è una piattaforma di scacchi online gratuita con matchmaking ELO, bot Stockfish e analisi delle partite." />
</svelte:head>

<div class="about-page">
	<div class="about-card">

		<div class="hero-row">
			<span class="icon">♟</span>
			<div>
				<h1>Chess</h1>
				<p class="tagline">Scacchi online — senza distrazioni, gratis.</p>
			</div>
		</div>

		<section>
			<h2>Cos'è Chess?</h2>
			<p>
				Chess è una piattaforma di scacchi online gratuita pensata per chi vuole giocare senza
				pubblicità invasive, registrazioni obbligatorie o abbonamenti. Puoi sfidare altri giocatori
				in tempo reale tramite matchmaking ELO oppure allenarti contro il motore
				<strong>Stockfish</strong>, lo stesso usato dai campioni mondiali.
			</p>
		</section>

		<section>
			<h2>Funzionalità</h2>
			<ul class="feature-list">
				<li>
					<span class="feat-icon">🎮</span>
					<span><strong>Matchmaking ELO</strong> — Rapid, Blitz e Bullet con tempi personalizzabili.</span>
				</li>
				<li>
					<span class="feat-icon">🤖</span>
					<span><strong>Bot Stockfish</strong> — diversi livelli di difficoltà, dal principiante al master.</span>
				</li>
				<li>
					<span class="feat-icon">🔍</span>
					<span><strong>Analisi partite</strong> — revisione mossa per mossa con valutazione del motore.</span>
				</li>
				<li>
					<span class="feat-icon">🏆</span>
					<span><strong>Classifica globale</strong> — top 50 giocatori per ELO Rapid.</span>
				</li>
				<li>
					<span class="feat-icon">📱</span>
					<span><strong>Mobile friendly</strong> — ottimizzato per smartphone e tablet.</span>
				</li>
			</ul>
		</section>

		<section>
			<h2>Stack tecnologico</h2>
			<div class="tech-row">
				<span class="tech-chip">SvelteKit</span>
				<span class="tech-chip">Go</span>
				<span class="tech-chip">PostgreSQL</span>
				<span class="tech-chip">WebSocket</span>
				<span class="tech-chip">Stockfish WASM</span>
			</div>
		</section>

		<section>
			<h2>Contatti</h2>
			<p>
				Per segnalazioni, suggerimenti o richieste scrivi a:
				<a href="mailto:salsx@hotmail.it">salsx@hotmail.it</a>
			</p>
		</section>

		<div class="legal-links">
			<a href="/privacy">{$t.nav.privacy}</a>
		</div>

	</div>
</div>

<style>
	.about-page {
		max-width: 680px;
		margin: 0 auto;
		padding: 2rem 1.5rem 4rem;
	}

	.about-card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 2.5rem 2rem;
		display: flex;
		flex-direction: column;
		gap: 2rem;
	}

	/* Hero row */
	.hero-row {
		display: flex;
		align-items: center;
		gap: 1rem;
	}
	.icon {
		font-size: 2.8rem;
		line-height: 1;
	}
	h1 {
		font-size: 1.8rem;
		font-weight: 800;
		color: var(--text);
		margin: 0;
	}
	.tagline {
		font-size: 0.9rem;
		color: var(--text-muted);
		margin: 0.2rem 0 0;
	}

	/* Sections */
	section {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}

	h2 {
		font-size: 1rem;
		font-weight: 700;
		color: var(--accent);
		border-bottom: 1px solid var(--border);
		padding-bottom: 0.3rem;
		margin: 0;
	}

	p {
		font-size: 0.9rem;
		color: var(--text-muted);
		line-height: 1.7;
		margin: 0;
	}

	strong { color: var(--text); }
	a { color: var(--accent); }
	a:hover { text-decoration: underline; }

	/* Feature list */
	.feature-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
	}
	.feature-list li {
		display: flex;
		align-items: flex-start;
		gap: 0.6rem;
		font-size: 0.9rem;
		color: var(--text-muted);
		line-height: 1.5;
	}
	.feat-icon {
		font-size: 1rem;
		flex-shrink: 0;
		margin-top: 0.05rem;
	}

	/* Tech chips */
	.tech-row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.tech-chip {
		background: var(--bg-input);
		border: 1px solid var(--border);
		border-radius: 20px;
		padding: 0.25rem 0.75rem;
		font-size: 0.8rem;
		color: var(--text-muted);
		font-weight: 500;
	}

	/* Legal footer */
	.legal-links {
		border-top: 1px solid var(--border);
		padding-top: 1rem;
		font-size: 0.82rem;
		color: var(--text-muted);
	}

	@media (max-width: 768px) {
		.about-card { padding: 1.5rem 1rem; }
	}
</style>
