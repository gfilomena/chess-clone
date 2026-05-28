/**
 * Chess piece SVGs — cburnett set (Colin Burnett, CC BY-SA 3.0)
 * Ported from Wikimedia Commons / lichess — viewBox 0 0 45 45
 */

export type PieceCode = 'wK'|'wQ'|'wR'|'wB'|'wN'|'wP'|'bK'|'bQ'|'bR'|'bB'|'bN'|'bP';

const s = (inner: string) =>
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 45 45">${inner}</svg>`;

export const PIECE_SVG: Record<PieceCode, string> = {

// ─── WHITE PIECES ─────────────────────────────────────────────────────────────

wP: s(`
<path d="M22,9 C19.79,9 18,10.79 18,13 C18,13.89 18.29,14.71 18.78,15.38
  C16.83,16.5 15.5,18.59 15.5,21 C15.5,23.03 16.44,24.84 17.91,26.03
  C14.91,27.09 10.5,31.58 10.5,39.5 L33.5,39.5
  C33.5,31.58 29.09,27.09 26.09,26.03 C27.56,24.84 28.5,23.03 28.5,21
  C28.5,18.59 27.17,16.5 25.22,15.38 C25.71,14.71 26,13.89 26,13
  C26,10.79 24.21,9 22,9 z"
  fill="#fff" stroke="#000" stroke-width="1.5" stroke-linecap="round"/>
<path d="M9,39 L36,39" fill="none" stroke="#000" stroke-width="1.5" stroke-linecap="round"/>
`),

wR: s(`
<g fill="#fff" stroke="#000" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <path d="M9,39 L36,39 L36,36 L9,36 L9,39 z" stroke-linecap="butt"/>
  <path d="M12.5,32 L14,29.5 L31,29.5 L32.5,32 L12.5,32 z" stroke-linecap="butt"/>
  <path d="M12,36 L12,32 L33,32 L33,36 L12,36 z" stroke-linecap="butt"/>
  <path d="M14,29.5 L14,16.5 L31,16.5 L31,29.5 L14,29.5 z" stroke-linecap="butt" stroke-linejoin="miter"/>
  <path d="M14,16.5 L11,14 L34,14 L31,16.5 L14,16.5 z" stroke-linecap="butt"/>
  <path d="M11,14 L11,9 L15,9 L15,11 L20,11 L20,9 L25,9 L25,11 L30,11 L30,9 L34,9 L34,14 z" stroke-linecap="butt"/>
</g>
`),

wB: s(`
<g fill="none" stroke="#000" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <g fill="#fff" stroke="#000" stroke-linecap="butt">
    <path d="M9,36 C12.39,35.03 19.11,36.43 22.5,34 C25.89,36.43 32.61,35.03 36,36
      C36,36 37.65,36.54 39,38 C38.32,38.97 37.35,38.99 36,38.5
      C32.61,37.53 25.89,38.96 22.5,37.5 C19.11,38.96 12.39,37.53 9,38.5
      C7.65,38.99 6.68,38.97 6,38 C7.35,36.54 9,36 9,36 z"/>
    <path d="M15,32 C17.5,34.5 27.5,34.5 30,32 C30.5,30.5 30,30 30,30
      C30,27.5 27.5,26 27.5,26 C33,24.5 33.5,14.5 22.5,10.5
      C11.5,14.5 12,24.5 17.5,26 C17.5,26 15,27.5 15,30
      C15,30 14.5,30.5 15,32 z"/>
    <path d="M25,8 A2.5,2.5,0,1,1,20,8 A2.5,2.5,0,1,1,25,8 z"/>
  </g>
  <path d="M17.5,26 L27.5,26 M15,30 L30,30 M22.5,15.5 L22.5,20.5 M20,18 L25,18" fill="none"/>
</g>
`),

wN: s(`
<g fill="#fff" stroke="#000" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <path d="M22,10 C32.5,11 38.5,18 38,39 L15,39 C15,30 25,32.5 23,18"/>
  <path d="M24,18 C24.38,20.91 18.45,25.37 16,27 C13,29 13.18,31.34 11,31
    C9.958,30.06 12.41,27.96 11,28 C10,28 11.19,29.23 10,30
    C9,30 5.997,31 6,26 C6,24 12,14 12,14
    C12,14 13.89,12.1 14,10.5 C13.27,9.506 13.5,8.5 13.5,7.5
    C14.5,6.5 16.5,10 16.5,10 L18.5,10
    C18.5,10 19.28,8.008 21,7 C22,7 22,10 22,10"/>
  <circle cx="9"   cy="25.5" r="0.5" fill="#000" stroke="#000"/>
  <circle cx="14.5" cy="15.5" r="0.5" fill="#000" stroke="#000"/>
</g>
`),

wQ: s(`
<g fill="#fff" stroke="#000" stroke-width="1.5" stroke-linejoin="round">
  <circle cx="6"    cy="12" r="2.75"/>
  <circle cx="14"   cy="9"  r="2.75"/>
  <circle cx="22.5" cy="8"  r="2.75"/>
  <circle cx="31"   cy="9"  r="2.75"/>
  <circle cx="39"   cy="12" r="2.75"/>
  <path d="M9,26 C17.5,24.5 30,24.5 36,26 L38.5,13.5 L31,25 L30.7,10.9
    L25.5,24.5 L22.5,10 L19.5,24.5 L14.3,10.9 L14,25 L6.5,13.5 L9,26 z"
    stroke-linecap="butt"/>
  <path d="M9,26 C9,28 10.5,28 11.5,30 C12.5,31.5 12.5,31 12,33.5
    C10.5,34.5 11,36 11,36 C9.5,37.5 11,38.5 11,38.5
    C17.5,39.5 27.5,39.5 34,38.5
    C34,38.5 35.5,37.5 34,36 C34,36 34.5,34.5 33,33.5
    C32.5,31 32.5,31.5 33.5,30 C34.5,28 36,28 36,26
    C27.5,24.5 17.5,24.5 9,26 z"
    stroke-linecap="butt"/>
  <path d="M11.5,30 C15,29 30,29 33.5,30" fill="none"/>
  <path d="M12,33.5 C15,32.5 30,32.5 33,33.5" fill="none"/>
</g>
`),

wK: s(`
<g fill="#fff" stroke="#000" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <path d="M22.5,11.63 L22.5,6"   stroke-linejoin="miter"/>
  <path d="M20,8 L25,8"           stroke-linejoin="miter"/>
  <path d="M22.5,25
    C22.5,25 27,17.5 25.5,14.5
    C25.5,14.5 24.5,12 22.5,12
    C20.5,12 19.5,14.5 19.5,14.5
    C18,17.5 22.5,25 22.5,25"
    fill="#fff" stroke-linecap="butt" stroke-linejoin="miter"/>
  <path d="M12.5,37
    C18,40.5 27,40.5 32.5,37
    L32.5,30
    C32.5,30 41.5,25.5 38.5,19.5
    C34.5,13 25,16 22.5,23.5
    L22.5,27 L22.5,23.5
    C20,16 10.5,13 6.5,19.5
    C3.5,25.5 12.5,30 12.5,30
    L12.5,37"
    fill="#fff"/>
  <path d="M12.5,30 C18,27 27,27 32.5,30"   fill="none"/>
  <path d="M12.5,33.5 C18,30.5 27,30.5 32.5,33.5" fill="none"/>
  <path d="M12.5,37 C18,34.5 27,34.5 32.5,37"     fill="none"/>
</g>
`),

// ─── BLACK PIECES ─────────────────────────────────────────────────────────────

bP: s(`
<path d="M22,9 C19.79,9 18,10.79 18,13 C18,13.89 18.29,14.71 18.78,15.38
  C16.83,16.5 15.5,18.59 15.5,21 C15.5,23.03 16.44,24.84 17.91,26.03
  C14.91,27.09 10.5,31.58 10.5,39.5 L33.5,39.5
  C33.5,31.58 29.09,27.09 26.09,26.03 C27.56,24.84 28.5,23.03 28.5,21
  C28.5,18.59 27.17,16.5 25.22,15.38 C25.71,14.71 26,13.89 26,13
  C26,10.79 24.21,9 22,9 z"
  fill="#000" stroke="#000" stroke-width="1.5" stroke-linecap="round"/>
<path d="M9,39 L36,39" fill="none" stroke="#fff" stroke-width="1" stroke-linecap="round"/>
`),

bR: s(`
<g fill="#000" stroke="#000" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <path d="M9,39 L36,39 L36,36 L9,36 L9,39 z" stroke-linecap="butt"/>
  <path d="M12.5,32 L14,29.5 L31,29.5 L32.5,32 L12.5,32 z" stroke-linecap="butt"/>
  <path d="M12,36 L12,32 L33,32 L33,36 L12,36 z" stroke-linecap="butt"/>
  <path d="M14,29.5 L14,16.5 L31,16.5 L31,29.5 L14,29.5 z" stroke-linecap="butt" stroke-linejoin="miter"/>
  <path d="M14,16.5 L11,14 L34,14 L31,16.5 L14,16.5 z" stroke-linecap="butt"/>
  <path d="M11,14 L11,9 L15,9 L15,11 L20,11 L20,9 L25,9 L25,11 L30,11 L30,9 L34,9 L34,14 z" stroke-linecap="butt"/>
</g>
`),

bB: s(`
<g fill="none" stroke="#000" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <g fill="#000" stroke="#000" stroke-linecap="butt">
    <path d="M9,36 C12.39,35.03 19.11,36.43 22.5,34 C25.89,36.43 32.61,35.03 36,36
      C36,36 37.65,36.54 39,38 C38.32,38.97 37.35,38.99 36,38.5
      C32.61,37.53 25.89,38.96 22.5,37.5 C19.11,38.96 12.39,37.53 9,38.5
      C7.65,38.99 6.68,38.97 6,38 C7.35,36.54 9,36 9,36 z"/>
    <path d="M15,32 C17.5,34.5 27.5,34.5 30,32 C30.5,30.5 30,30 30,30
      C30,27.5 27.5,26 27.5,26 C33,24.5 33.5,14.5 22.5,10.5
      C11.5,14.5 12,24.5 17.5,26 C17.5,26 15,27.5 15,30
      C15,30 14.5,30.5 15,32 z"/>
    <path d="M25,8 A2.5,2.5,0,1,1,20,8 A2.5,2.5,0,1,1,25,8 z"/>
  </g>
  <path d="M17.5,26 L27.5,26 M15,30 L30,30 M22.5,15.5 L22.5,20.5 M20,18 L25,18"
    fill="none" stroke="#fff" stroke-linejoin="miter"/>
</g>
`),

bN: s(`
<g fill="#000" stroke="#000" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <path d="M22,10 C32.5,11 38.5,18 38,39 L15,39 C15,30 25,32.5 23,18"/>
  <path d="M24,18 C24.38,20.91 18.45,25.37 16,27 C13,29 13.18,31.34 11,31
    C9.958,30.06 12.41,27.96 11,28 C10,28 11.19,29.23 10,30
    C9,30 5.997,31 6,26 C6,24 12,14 12,14
    C12,14 13.89,12.1 14,10.5 C13.27,9.506 13.5,8.5 13.5,7.5
    C14.5,6.5 16.5,10 16.5,10 L18.5,10
    C18.5,10 19.28,8.008 21,7 C22,7 22,10 22,10"/>
  <circle cx="9"    cy="25.5" r="0.5" fill="#fff" stroke="#fff"/>
  <circle cx="14.5" cy="15.5" r="0.5" fill="#fff" stroke="#fff"/>
</g>
`),

bQ: s(`
<g fill="#000" stroke="#000" stroke-width="1.5" stroke-linejoin="round">
  <circle cx="6"    cy="12" r="2.75" fill="#fff" stroke="#fff"/>
  <circle cx="14"   cy="9"  r="2.75" fill="#fff" stroke="#fff"/>
  <circle cx="22.5" cy="8"  r="2.75" fill="#fff" stroke="#fff"/>
  <circle cx="31"   cy="9"  r="2.75" fill="#fff" stroke="#fff"/>
  <circle cx="39"   cy="12" r="2.75" fill="#fff" stroke="#fff"/>
  <path d="M9,26 C17.5,24.5 30,24.5 36,26 L38.5,13.5 L31,25 L30.7,10.9
    L25.5,24.5 L22.5,10 L19.5,24.5 L14.3,10.9 L14,25 L6.5,13.5 L9,26 z"
    stroke-linecap="butt"/>
  <path d="M9,26 C9,28 10.5,28 11.5,30 C12.5,31.5 12.5,31 12,33.5
    C10.5,34.5 11,36 11,36 C9.5,37.5 11,38.5 11,38.5
    C17.5,39.5 27.5,39.5 34,38.5
    C34,38.5 35.5,37.5 34,36 C34,36 34.5,34.5 33,33.5
    C32.5,31 32.5,31.5 33.5,30 C34.5,28 36,28 36,26
    C27.5,24.5 17.5,24.5 9,26 z"
    stroke-linecap="butt"/>
  <path d="M11.5,30 C15,29 30,29 33.5,30" fill="none" stroke="#fff"/>
  <path d="M12,33.5 C15,32.5 30,32.5 33,33.5" fill="none" stroke="#fff"/>
</g>
`),

bK: s(`
<g fill="#000" stroke="#000" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <path d="M22.5,11.63 L22.5,6"   fill="none" stroke="#fff" stroke-linejoin="miter"/>
  <path d="M20,8 L25,8"           fill="none" stroke="#fff" stroke-linejoin="miter"/>
  <path d="M22.5,25
    C22.5,25 27,17.5 25.5,14.5
    C25.5,14.5 24.5,12 22.5,12
    C20.5,12 19.5,14.5 19.5,14.5
    C18,17.5 22.5,25 22.5,25"
    fill="#000" stroke-linecap="butt" stroke-linejoin="miter"/>
  <path d="M12.5,37
    C18,40.5 27,40.5 32.5,37
    L32.5,30
    C32.5,30 41.5,25.5 38.5,19.5
    C34.5,13 25,16 22.5,23.5
    L22.5,27 L22.5,23.5
    C20,16 10.5,13 6.5,19.5
    C3.5,25.5 12.5,30 12.5,30
    L12.5,37"
    fill="#000"/>
  <path d="M12.5,30 C18,27 27,27 32.5,30"         fill="none" stroke="#fff"/>
  <path d="M12.5,33.5 C18,30.5 27,30.5 32.5,33.5"  fill="none" stroke="#fff"/>
  <path d="M12.5,37 C18,34.5 27,34.5 32.5,37"      fill="none" stroke="#fff"/>
</g>
`),

};
