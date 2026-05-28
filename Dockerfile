# ── Stage 1: Build frontend SvelteKit ─────────────────────────────────────
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./

# Build SPA statica (adapter-static → output in /app/frontend/build)
RUN npm run build

# ── Stage 2: Build backend Go ──────────────────────────────────────────────
FROM golang:1.22-alpine AS backend-builder

WORKDIR /app/backend

# Dipendenze Go (cache layer separato)
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# Copia il build frontend nella directory embedded del binary Go
COPY --from=frontend-builder /app/frontend/build ./cmd/server/static

# Compila il binario (embed include il frontend)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o chess-server ./cmd/server

# ── Stage 3: Runtime minimale ─────────────────────────────────────────────
FROM alpine:3.19

# Certificati SSL necessari per Google OAuth e connessioni TLS
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /app/backend/chess-server .

EXPOSE 8080

CMD ["./chess-server"]
