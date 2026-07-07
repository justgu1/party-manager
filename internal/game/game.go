// Package game implements the authoritative draw for the roulette / horse-race
// mini-game and keeps a history of results.
package game

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guilherme/help-party/internal/auth"
	"github.com/guilherme/help-party/internal/httpx"
)

type Handler struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func (h *Handler) Routes(r chi.Router, admin func(http.Handler) http.Handler) {
	r.Get("/game/history", h.History) // anyone can see past draws
	r.With(admin).Post("/game/spin", h.Spin)
}

type spinReq struct {
	Options []string `json:"options"`
	Mode    string   `json:"mode"` // "roulette" | "race"
}

type spinResp struct {
	Options     []string `json:"options"`
	Mode        string   `json:"mode"`
	WinnerIndex int      `json:"winner_index"`
	Winner      string   `json:"winner"`
	Seed        int64    `json:"seed"`
}

type drawView struct {
	ID        uuid.UUID `json:"id"`
	Mode      string    `json:"mode"`
	Options   []string  `json:"options"`
	Winner    string    `json:"winner"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// Spin picks a uniformly-random winner among the provided options and records it.
func (h *Handler) Spin(w http.ResponseWriter, r *http.Request) {
	var in spinReq
	if !httpx.Decode(w, r, &in) {
		return
	}

	opts := make([]string, 0, len(in.Options))
	for _, o := range in.Options {
		if s := strings.TrimSpace(o); s != "" {
			opts = append(opts, s)
		}
	}
	if len(opts) < 2 {
		httpx.Error(w, http.StatusBadRequest, "informe ao menos 2 opções")
		return
	}
	if len(opts) > 100 {
		httpx.Error(w, http.StatusBadRequest, "máximo de 100 opções")
		return
	}

	mode := in.Mode
	if mode != "race" {
		mode = "roulette"
	}
	winner := randInt(len(opts))
	seed := randInt(1 << 30)

	if h.pool != nil {
		u, _ := auth.FromContext(r.Context())
		raw, _ := json.Marshal(opts)
		var createdBy any
		if u.ID != uuid.Nil {
			createdBy = u.ID
		}
		_, _ = h.pool.Exec(r.Context(),
			`INSERT INTO game_draws (mode, options, winner, winner_index, created_by)
			 VALUES ($1,$2,$3,$4,$5)`,
			mode, raw, opts[winner], winner, createdBy)
	}

	httpx.JSON(w, http.StatusOK, spinResp{
		Options:     opts,
		Mode:        mode,
		WinnerIndex: winner,
		Winner:      opts[winner],
		Seed:        int64(seed),
	})
}

// History returns the most recent draws.
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT g.id, g.mode, g.options, g.winner, COALESCE(u.name, '—'), g.created_at
		 FROM game_draws g
		 LEFT JOIN users u ON u.id = g.created_by
		 ORDER BY g.created_at DESC
		 LIMIT 50`)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler histórico")
		return
	}
	defer rows.Close()
	out := []drawView{}
	for rows.Next() {
		var d drawView
		var raw []byte
		if err := rows.Scan(&d.ID, &d.Mode, &raw, &d.Winner, &d.CreatedBy, &d.CreatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "erro ao ler sorteio")
			return
		}
		_ = json.Unmarshal(raw, &d.Options)
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// randInt returns a crypto-random int in [0, n).
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}
