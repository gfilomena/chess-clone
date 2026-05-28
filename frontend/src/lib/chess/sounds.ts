/**
 * Modulo audio per gli scacchi.
 *
 * Tema "Legno" — CC0, lavenderdotpet/CC0-Public-Domain-Sounds
 * Tema "Legno Reale" — CC0, Freesound simone_ds #366065
 *   (registrazione reale di pezzi su scacchiera in legno, estratta e normalizzata)
 */
import { browser } from '$app/environment';
import { writable, get } from 'svelte/store';

export type SoundName  = 'move' | 'capture' | 'check' | 'game_start' | 'game_over' | 'illegal';
export type SoundTheme = 'wood' | 'wood-real';

// ── Libreria temi ────────────────────────────────────────────────────────────

const THEMES: Record<SoundTheme, { label: string; files: Record<SoundName, string> }> = {
	'wood': {
		label: '🪵 Legno',
		files: {
			move:       '/sounds/wood_hit_01.ogg',
			capture:    '/sounds/wood_slam_01.ogg',
			check:      '/sounds/wood_hit_03.ogg',
			game_start: '/sounds/misc_01.ogg',
			game_over:  '/sounds/wood_slam_02.ogg',
			illegal:    '/sounds/wood_hit_05.ogg',
		},
	},
	'wood-real': {
		label: '♟️ Legno Reale',
		files: {
			move:       '/sounds/wood-real/move.mp3',
			capture:    '/sounds/wood-real/capture.mp3',
			check:      '/sounds/wood-real/check.mp3',
			game_start: '/sounds/wood-real/game_start.mp3',
			game_over:  '/sounds/wood-real/game_over.mp3',
			illegal:    '/sounds/wood-real/illegal.mp3',
		},
	},
};

export const THEME_KEYS = Object.keys(THEMES) as SoundTheme[];

// ── Stato globale ────────────────────────────────────────────────────────────

export const soundTheme = writable<SoundTheme>(
	(browser && (localStorage.getItem('soundTheme') as SoundTheme)) || 'wood-real'
);

let muted = false;

/**
 * Cache di tutti i temi, precaricata una sola volta in initSounds().
 * Il cambio tema è istantaneo: si punta semplicemente all'altro oggetto.
 */
const cache: Partial<Record<SoundTheme, Partial<Record<SoundName, HTMLAudioElement>>>> = {};

// ── API pubblica ─────────────────────────────────────────────────────────────

/**
 * Precarica TUTTI i temi audio.
 * Chiamare in onMount — eseguito una sola volta per sessione.
 */
export function initSounds(): void {
	if (!browser) return;
	for (const [theme, config] of Object.entries(THEMES) as [SoundTheme, typeof THEMES[SoundTheme]][]) {
		if (cache[theme]) continue;            // già caricato
		const bucket: Partial<Record<SoundName, HTMLAudioElement>> = {};
		for (const [name, src] of Object.entries(config.files) as [SoundName, string][]) {
			const el = new Audio(src);
			el.preload = 'auto';
			el.load();
			bucket[name as SoundName] = el;
		}
		cache[theme] = bucket;
	}
}

/** Riproduce un suono del tema attivo. Ignorato se muted. */
export function playSound(name: SoundName): void {
	if (!browser || muted) return;
	const theme = get(soundTheme);
	const el = cache[theme]?.[name];
	if (!el) return;
	el.currentTime = 0;
	el.play().catch(() => { /* autoplay blocked */ });
}

/** Alterna mute. Ritorna il nuovo stato (true = muted). */
export function toggleMute(): boolean {
	muted = !muted;
	return muted;
}

export function isMuted(): boolean { return muted; }

/** Cicla al tema successivo e lo persiste. */
export function cycleTheme(): SoundTheme {
	const current = get(soundTheme);
	const next = THEME_KEYS[(THEME_KEYS.indexOf(current) + 1) % THEME_KEYS.length];
	soundTheme.set(next);
	if (browser) localStorage.setItem('soundTheme', next);
	return next;
}

/** Etichetta leggibile del tema. */
export function themeLabel(theme: SoundTheme): string {
	return THEMES[theme]?.label ?? theme;
}

// ── Interno ──────────────────────────────────────────────────────────────────

/**
 * Rileva il suono da suonare dall'ultima mossa in notazione PGN.
 * "Nxf6+" → 'check', "exd5" → 'capture', "Nf3" → 'move'
 */
export function soundForPgnMove(pgn: string): SoundName {
	const tokens = pgn.trim().split(/\s+/);
	const moves  = tokens.filter(t => !/^\d+\./.test(t));
	const last   = moves.at(-1) ?? '';
	if (last.includes('#') || last.includes('+')) return 'check';
	if (last.includes('x'))                       return 'capture';
	return 'move';
}
