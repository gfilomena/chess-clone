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

/**
 * Classifica una mossa confrontando eval prima e dopo
 * Restituisce: 'best' | 'good' | 'inaccuracy' | 'mistake' | 'blunder'
 */
export function classifyMove(scoreBefore: number, scoreAfter: number): string {
	const delta = scoreBefore - scoreAfter; // perdita dal punto di vista di chi ha mosso
	if (delta < 10) return 'best';
	if (delta < 50) return 'good';
	if (delta < 100) return 'inaccuracy';
	if (delta < 200) return 'mistake';
	return 'blunder';
}
