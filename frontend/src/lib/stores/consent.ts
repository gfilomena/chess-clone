import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export type ConsentState = 'pending' | 'accepted' | 'essential';

// ─── Publisher ID AdSense ────────────────────────────────────────────────────
// Sostituisci con il tuo ID dopo l'approvazione di AdSense:
// es. 'ca-pub-1234567890123456'
export const ADSENSE_PUBLISHER_ID = '';   // ← inserisci qui

// ─── Store ───────────────────────────────────────────────────────────────────
const stored = browser
  ? (localStorage.getItem('cookie_consent') as ConsentState | null)
  : null;

export const consent = writable<ConsentState>(stored ?? 'pending');

consent.subscribe(val => {
  if (browser && val !== 'pending') {
    localStorage.setItem('cookie_consent', val);
  }
  if (browser && val === 'accepted') {
    loadAdSense();
  }
});

// ─── Helpers ─────────────────────────────────────────────────────────────────
export function acceptAll() { consent.set('accepted'); }
export function acceptEssential() { consent.set('essential'); }

function loadAdSense() {
  if (!ADSENSE_PUBLISHER_ID || !browser) return;
  if (document.querySelector(`script[src*="${ADSENSE_PUBLISHER_ID}"]`)) return; // già caricato
  const s = document.createElement('script');
  s.async = true;
  s.src = `https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=${ADSENSE_PUBLISHER_ID}`;
  s.crossOrigin = 'anonymous';
  document.head.appendChild(s);
}
