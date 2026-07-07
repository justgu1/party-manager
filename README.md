# help-party 🎉

App para o grupo organizar o aluguel/festa de fim de ano. Três funcionalidades:

1. **Lugares** — cole links de anúncios (Airbnb, Booking, sites BR ou qualquer URL); um
   worker raspa infos + disponibilidade de datas; ranqueia por **upvote** do grupo.
2. **Jukebox + Prendas** — cole um link do YouTube e escolha uma **prenda** (simbólica); a
   música só destrava para tocar quando a prenda for marcada como cumprida.
3. **Sorteio** — roleta / corrida de cavalos para decidir coisas diversas.

## Stack

- **Backend:** Go (chi, pgx, JWT, River queue sobre Postgres, chromedp/goquery p/ scraping)
- **Frontend:** Svelte + Vite (SPA), embutido no binário Go via `embed.FS`
- **DB:** Postgres · **Auth:** email/senha (bcrypt) + JWT
- **Deploy:** Docker Compose

## Rodando com Docker (recomendado)

```bash
cp .env.example .env   # ajuste JWT_SECRET
docker compose up --build
```

- App em `http://localhost:8080` (API + SPA no mesmo binário).
- Postgres em `localhost:5432` (migrations aplicadas automaticamente no boot da API).

## Desenvolvimento local (sem Docker para o app)

```bash
# 1. suba só o Postgres
docker compose up -d db
# 2. rode a API (aplica migrations no boot)
make dev-api
# 3. em outro terminal, o frontend com hot-reload (proxy /api -> :8080)
make dev-web   # abre em http://localhost:5173
```

## Estrutura

- `cmd/api` — servidor HTTP + SPA embutida
- `cmd/worker` — worker de scraping (River)
- `internal/` — auth, users, rentals, scraper, music, prendas, game, db, httpx
- `web/` — app Svelte (build vai para `web/dist`, embutido pela API)
