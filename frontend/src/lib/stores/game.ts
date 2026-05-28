import { writable } from 'svelte/store';

export type GameStatus = 'waiting' | 'active' | 'paused' | 'finished';
export type GameResult = 'white' | 'black' | 'draw' | 'abandoned' | null;
export type PlayerColor = 'white' | 'black';

export interface GameState {
	id: string;
	fen: string;                  // posizione corrente
	pgn: string;                  // mosse in formato PGN
	turn: 'w' | 'b';             // chi deve muovere
	status: GameStatus;
	result: GameResult;
	finishReason: string | null;
	whiteMs: number;              // tempo rimanente bianco in ms
	blackMs: number;              // tempo rimanente nero in ms
	playerColor: PlayerColor;     // il nostro colore
	drawOffered: boolean;
}

const initialState: GameState = {
	id: '',
	fen: 'rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1',
	pgn: '',
	turn: 'w',
	status: 'waiting',
	result: null,
	finishReason: null,
	whiteMs: 600000,
	blackMs: 600000,
	playerColor: 'white',
	drawOffered: false
};

export const gameState = writable<GameState>(initialState);

export function resetGame() {
	gameState.set(initialState);
}
