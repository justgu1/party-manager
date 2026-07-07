package rentals

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/guilherme/help-party/internal/httpx"
)

type placeView struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	ImageURL string    `json:"image_url"`
	Score    int       `json:"score"`
}

type matchView struct {
	ID       string     `json:"id"`
	Position int        `json:"position"`
	A        *placeView `json:"a"`
	B        *placeView `json:"b"`
	Winner   *uuid.UUID `json:"winner"` // current leader (live)
}

type roundView struct {
	Round   int         `json:"round"`
	Label   string      `json:"label"`
	Matches []matchView `json:"matches"`
}

type bracketView struct {
	Active     bool        `json:"active"`
	Rounds     int         `json:"rounds"`
	Places     int         `json:"places"`
	Champion   *placeView  `json:"champion"`
	RoundsData []roundView `json:"rounds_data"`
}

// scores returns a map of rental id -> net vote score.
func (h *Handler) scores(ctx context.Context) map[uuid.UUID]int {
	out := map[uuid.UUID]int{}
	rows, err := h.pool.Query(ctx,
		`SELECT rental_id, COALESCE(SUM(value),0)::int FROM rental_votes GROUP BY rental_id`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var s int
		if rows.Scan(&id, &s) == nil {
			out[id] = s
		}
	}
	return out
}

// Tournament builds the whole bracket automatically and live: every place is a
// seed (in a stable creation order), the field is padded with byes to the next
// power of two, and each match winner is the current leader (higher net score)
// propagated up to the champion. It updates on every vote — there is no manual
// start/advance step and no separate list.
func (h *Handler) Tournament(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	view := bracketView{RoundsData: []roundView{}}

	// Seeds in stable creation order (so the bracket shape doesn't reshuffle as
	// votes change — only the winners do).
	rows, err := h.pool.Query(ctx, `SELECT id, title, image_url FROM rentals ORDER BY created_at ASC`)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao montar chave")
		return
	}
	places := map[uuid.UUID]placeView{}
	var order []uuid.UUID
	for rows.Next() {
		var p placeView
		if rows.Scan(&p.ID, &p.Title, &p.ImageURL) == nil {
			order = append(order, p.ID)
			places[p.ID] = p
		}
	}
	rows.Close()

	view.Places = len(order)
	if len(order) < 2 {
		httpx.JSON(w, http.StatusOK, view) // Active stays false → UI shows empty state.
		return
	}

	sc := h.scores(ctx)
	for id, p := range places {
		p.Score = sc[id]
		places[id] = p
	}
	pv := func(id *uuid.UUID) *placeView {
		if id == nil {
			return nil
		}
		p := places[*id]
		return &p
	}

	// Pad to the next power of two with byes (nil), standard 1-vs-N seeding.
	size := powerOfTwoCeil(len(order))
	seeds := make([]*uuid.UUID, size)
	for i := range order {
		seeds[i] = &order[i]
	}
	type pair struct{ a, b *uuid.UUID }
	cur := make([]pair, 0, size/2)
	for j := 0; j < size/2; j++ {
		cur = append(cur, pair{a: seeds[j], b: seeds[size-1-j]})
	}

	rounds := log2(size)
	view.Active = true
	view.Rounds = rounds

	for rnd := 0; ; rnd++ {
		matches := make([]matchView, 0, len(cur))
		winners := make([]*uuid.UUID, 0, len(cur))
		for i, p := range cur {
			win := decideWinner(p.a, p.b, sc)
			matches = append(matches, matchView{
				ID:       fmt.Sprintf("r%d-%d", rnd, i),
				Position: i,
				A:        pv(p.a),
				B:        pv(p.b),
				Winner:   win,
			})
			winners = append(winners, win)
		}
		view.RoundsData = append(view.RoundsData, roundView{
			Round:   rnd,
			Label:   stageLabel(rnd, rounds),
			Matches: matches,
		})
		if len(cur) == 1 {
			view.Champion = pv(winners[0])
			break
		}
		next := make([]pair, 0, len(winners)/2)
		for k := 0; k+1 < len(winners); k += 2 {
			next = append(next, pair{a: winners[k], b: winners[k+1]})
		}
		cur = next
	}
	httpx.JSON(w, http.StatusOK, view)
}

// decideWinner returns the higher-scoring place; byes and ties favour A.
func decideWinner(a, b *uuid.UUID, sc map[uuid.UUID]int) *uuid.UUID {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if sc[*b] > sc[*a] {
		return b
	}
	return a
}

// powerOfTwoCeil returns the smallest power of two ≥ n (min 2).
func powerOfTwoCeil(n int) int {
	p := 2
	for p < n {
		p *= 2
	}
	return p
}

func log2(n int) int {
	r := 0
	for n > 1 {
		n /= 2
		r++
	}
	return r
}

// stageLabel names a round given the total number of rounds.
func stageLabel(round, rounds int) string {
	switch rounds - round - 1 {
	case 0:
		return "Final"
	case 1:
		return "Semifinal"
	case 2:
		return "Quartas de final"
	case 3:
		return "Oitavas de final"
	default:
		return fmt.Sprintf("Rodada %d", round+1)
	}
}
