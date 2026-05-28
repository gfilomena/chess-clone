import { browser } from '$app/environment';

export interface AnalysisResult {
	depth: number;
	score: number;       // centipawns, normalizzato prospettiva bianco
	isMate: boolean;
	mateIn: number | null;
	bestMove: string;    // UCI: "e2e4"
	pv: string[];        // variante principale
}

export class StockfishEngine {
	private worker: Worker | null = null;
	private initialized = false;
	private onMessage: ((msg: string) => void) | null = null;

	async init(): Promise<void> {
		if (!browser || this.initialized) return;

		this.worker = new Worker('/stockfish.js');

		this.worker.onmessage = (e) => {
			const msg = typeof e.data === 'string' ? e.data : e.data.toString();
			this.onMessage?.(msg);
		};

		await this.waitFor('uci', 'uciok');
		this.send('setoption name Hash value 32');
		await this.waitFor('isready', 'readyok');

		this.initialized = true;
	}

	/**
	 * Analizza una posizione FEN.
	 * Il punteggio è sempre normalizzato dalla prospettiva del Bianco.
	 */
	analyze(fen: string, depth = 16): Promise<AnalysisResult> {
		return new Promise((resolve) => {
			let best: Partial<AnalysisResult> = {};

			this.onMessage = (msg: string) => {
				if (msg.startsWith('info') && msg.includes('score') && msg.includes('pv')) {
					const parsed = parseInfo(msg);
					if (parsed && (parsed.depth ?? 0) > (best.depth ?? 0)) {
						best = parsed;
					}
				}

				if (msg.startsWith('bestmove')) {
					const parts = msg.split(' ');
					const bestMove = parts[1] ?? '';
					this.onMessage = null;

					// Normalizza score in prospettiva bianco
					// (Stockfish dà score dal lato di chi muove)
					const turn = fen.split(' ')[1]; // 'w' o 'b'
					const rawScore = best.score ?? 0;
					const normalizedScore = turn === 'w' ? rawScore : -rawScore;

					resolve({
						depth: best.depth ?? 0,
						score: normalizedScore,
						isMate: best.isMate ?? false,
						mateIn: best.mateIn ?? null,
						bestMove,
						pv: best.pv ?? []
					});
				}
			};

			this.send(`position fen ${fen}`);
			this.send(`go depth ${depth}`);
		});
	}

	/**
	 * Ottiene la mossa migliore per la posizione FEN al livello ELO specificato.
	 * Usa UCI_LimitStrength + UCI_Elo per limitare la forza del motore.
	 * Ritorna la mossa in formato UCI (es. "e2e4", "e7e8q") o "" se non disponibile.
	 */
	getBestMove(fen: string, elo: number): Promise<string> {
		if (!this.initialized) return Promise.resolve('');

		return new Promise((resolve) => {
			this.onMessage = (msg: string) => {
				if (msg.startsWith('bestmove')) {
					const move = msg.split(' ')[1] ?? '';
					this.onMessage = null;
					resolve(move === '(none)' ? '' : move);
				}
			};

			this.send('setoption name UCI_LimitStrength value true');
			this.send(`setoption name UCI_Elo value ${Math.max(200, Math.min(2850, elo))}`);
			this.send(`position fen ${fen}`);

			// Movetime proporzionale all'ELO: 300ms (200 ELO) → 1500ms (2000 ELO)
			const movetime = Math.round(300 + ((elo - 200) / 1800) * 1200);
			this.send(`go movetime ${movetime}`);
		});
	}

	/**
	 * Analizza sequenzialmente tutti i FEN (una partita completa).
	 * depth consigliato: 14-16. onProgress(done, total) per la progress bar.
	 */
	async analyzeAll(
		fens: string[],
		depth = 14,
		onProgress?: (done: number, total: number) => void
	): Promise<Array<{ score: number; bestMove: string }>> {
		const results: Array<{ score: number; bestMove: string }> = [];
		for (let i = 0; i < fens.length; i++) {
			const r = await this.analyze(fens[i], depth);
			results.push({ score: r.score, bestMove: r.bestMove });
			onProgress?.(i + 1, fens.length);
		}
		return results;
	}

	stop() {
		this.send('stop');
	}

	destroy() {
		this.send('quit');
		this.worker?.terminate();
		this.worker = null;
		this.initialized = false;
	}

	private send(cmd: string) {
		this.worker?.postMessage(cmd);
	}

	private waitFor(cmd: string, expected: string): Promise<void> {
		return new Promise((resolve) => {
			const handler = (e: MessageEvent) => {
				const msg = typeof e.data === 'string' ? e.data : e.data.toString();
				if (msg.includes(expected)) {
					this.worker!.removeEventListener('message', handler);
					resolve();
				}
			};
			this.worker!.addEventListener('message', handler);
			this.send(cmd);
		});
	}
}

// ── Parser UCI ────────────────────────────────────────────────────────────

function parseInfo(msg: string): Partial<AnalysisResult> | null {
	const result: Partial<AnalysisResult> = {};

	const depthM = msg.match(/\bdepth (\d+)/);
	if (depthM) result.depth = parseInt(depthM[1]);

	const cpM = msg.match(/\bscore cp (-?\d+)/);
	const mateM = msg.match(/\bscore mate (-?\d+)/);

	if (cpM) {
		result.score = parseInt(cpM[1]);
		result.isMate = false;
		result.mateIn = null;
	} else if (mateM) {
		const m = parseInt(mateM[1]);
		result.score = m > 0 ? 30000 : -30000;
		result.isMate = true;
		result.mateIn = m;
	} else {
		return null;
	}

	const pvM = msg.match(/\bpv (.+)$/);
	if (pvM) result.pv = pvM[1].trim().split(' ');

	return result;
}

// ── Helpers UI ────────────────────────────────────────────────────────────

/**
 * Converte centipawns in percentuale per l'eval bar (0-100, 50 = pari)
 * Usa tanh per avere una curva naturale
 */
export function evalToPercent(result: AnalysisResult): number {
	if (result.isMate) {
		return result.mateIn! > 0 ? 98 : 2;
	}
	return 50 + 50 * Math.tanh(result.score / 400);
}

/**
 * Formatta lo score in testo leggibile: "+1.2", "-0.5", "M3", "M-2"
 */
export function formatScore(result: AnalysisResult): string {
	if (result.isMate) {
		return `M${result.mateIn}`;
	}
	const pawns = result.score / 100;
	const sign = pawns >= 0 ? '+' : '';
	return `${sign}${pawns.toFixed(1)}`;
}

// ── Move classification ───────────────────────────────────────────────────

export interface MoveClassification {
	key:    'best' | 'excellent' | 'good' | 'inaccuracy' | 'mistake' | 'blunder';
	label:  string;   // testo italiano
	symbol: string;   // annotazione scacchistica
	color:  string;   // CSS color
	delta:  number;   // perdita in centipawns
}

/**
 * Classifica una mossa confrontando eval prima e dopo.
 * scoreBefore/scoreAfter: centipawns normalizzati prospettiva BIANCO.
 * whiteToMove: true se era il turno del bianco.
 * playedUci: mossa giocata in UCI (es. "e2e4").
 * bestUci: mossa migliore del motore (es. "e2e4" o "(none)").
 */
export function classifyMove(
	scoreBefore: number,
	scoreAfter:  number,
	whiteToMove: boolean,
	playedUci:   string,
	bestUci:     string
): MoveClassification {
	// Perdita dal punto di vista di chi ha mosso
	const delta = Math.max(0,
		whiteToMove
			? scoreBefore - scoreAfter    // bianco vuole score alto
			: scoreAfter  - scoreBefore   // nero vuole score basso (white score cala)
	);

	const isBest = playedUci.slice(0, 4) === bestUci.slice(0, 4) && bestUci.slice(0, 4) !== '';

	if (isBest || delta < 5)   return { key: 'best',        label: 'Ottima',       symbol: '!!', color: '#5B8E55', delta };
	if (delta < 25)            return { key: 'excellent',   label: 'Eccellente',   symbol: '!',  color: '#81B64C', delta };
	if (delta < 60)            return { key: 'good',        label: 'Buona',        symbol: '',   color: '#5080C0', delta };
	if (delta < 120)           return { key: 'inaccuracy',  label: 'Imprecisione', symbol: '?!', color: '#C9A020', delta };
	if (delta < 300)           return { key: 'mistake',     label: 'Errore',       symbol: '?',  color: '#D97706', delta };
	return                            { key: 'blunder',     label: 'Gaffe',        symbol: '??', color: '#DC2626', delta };
}

/**
 * Analizza in sequenza tutti i FEN passati.
 * Ritorna un array parallel di { score, bestMove } per ciascuna posizione.
 * onProgress(done, total) viene chiamata dopo ogni posizione.
 */
