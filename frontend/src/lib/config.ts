// Configurazione centralizzata — cambia qui le porte se necessario
export const API_URL  = import.meta.env.VITE_API_URL  ?? 'http://localhost:8080';
export const WS_URL   = import.meta.env.VITE_WS_URL   ?? 'ws://localhost:8080';
export const DEV_MODE = import.meta.env.VITE_DEV_MODE === 'true';
