package game

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func spin(t *testing.T, body string) (*httptest.ResponseRecorder, spinResp) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/game/spin", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	New(nil).Spin(rec, req)
	var out spinResp
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestSpinPicksValidWinner(t *testing.T) {
	rec, out := spin(t, `{"options":["a","b","c"],"mode":"race"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if out.Mode != "race" {
		t.Errorf("mode = %q", out.Mode)
	}
	if out.WinnerIndex < 0 || out.WinnerIndex >= len(out.Options) {
		t.Fatalf("winner index out of range: %d", out.WinnerIndex)
	}
	if out.Winner != out.Options[out.WinnerIndex] {
		t.Errorf("winner %q != options[%d] %q", out.Winner, out.WinnerIndex, out.Options[out.WinnerIndex])
	}
}

func TestSpinRejectsTooFewOptions(t *testing.T) {
	rec, _ := spin(t, `{"options":["só uma"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSpinTrimsAndDefaultsMode(t *testing.T) {
	_, out := spin(t, `{"options":["  a ","","b"]}`)
	if len(out.Options) != 2 {
		t.Fatalf("expected empty options trimmed, got %v", out.Options)
	}
	if out.Mode != "roulette" {
		t.Errorf("expected default roulette, got %q", out.Mode)
	}
}
