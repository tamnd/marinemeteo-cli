package marinemeteo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/marinemeteo-cli/marinemeteo"
)

// sampleResponse is a minimal valid API response for both hourly and daily.
var sampleResponse = map[string]any{
	"latitude":  48.791664,
	"longitude": -4.5416565,
	"hourly": map[string]any{
		"time":              []string{"2026-06-14T00:00", "2026-06-14T01:00"},
		"wave_height":       []float64{1.54, 1.48},
		"wave_direction":    []float64{292, 292},
		"wave_period":       []float64{9.9, 9.75},
		"wind_wave_height":  []float64{0.5, 0.48},
		"swell_wave_height": []float64{1.2, 1.15},
	},
	"daily": map[string]any{
		"time":                    []string{"2026-06-14"},
		"wave_height_max":         []float64{2.1},
		"wave_direction_dominant": []float64{285},
		"wave_period_max":         []float64{10.5},
	},
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sampleResponse)
	}))
}

func TestHourlyForecast(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	cfg := marinemeteo.DefaultConfig()
	cfg.BaseURL = ts.URL
	c := marinemeteo.NewClient(cfg)

	rows, err := c.HourlyForecast(context.Background(), 48.8, -4.5, 2)
	if err != nil {
		t.Fatalf("HourlyForecast: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	r := rows[0]
	if r.Time != "2026-06-14T00:00" {
		t.Errorf("Time = %q, want 2026-06-14T00:00", r.Time)
	}
	if r.WaveHeight != 1.54 {
		t.Errorf("WaveHeight = %v, want 1.54", r.WaveHeight)
	}
	if r.WaveDirection != 292 {
		t.Errorf("WaveDirection = %v, want 292", r.WaveDirection)
	}
	if r.WavePeriod != 9.9 {
		t.Errorf("WavePeriod = %v, want 9.9", r.WavePeriod)
	}
	if r.WindWaveHeight != 0.5 {
		t.Errorf("WindWaveHeight = %v, want 0.5", r.WindWaveHeight)
	}
	if r.SwellWaveHeight != 1.2 {
		t.Errorf("SwellWaveHeight = %v, want 1.2", r.SwellWaveHeight)
	}
}

func TestDailyForecast(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	cfg := marinemeteo.DefaultConfig()
	cfg.BaseURL = ts.URL
	c := marinemeteo.NewClient(cfg)

	rows, err := c.DailyForecast(context.Background(), 48.8, -4.5, 5)
	if err != nil {
		t.Fatalf("DailyForecast: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.Time != "2026-06-14" {
		t.Errorf("Time = %q, want 2026-06-14", r.Time)
	}
	if r.WaveHeightMax != 2.1 {
		t.Errorf("WaveHeightMax = %v, want 2.1", r.WaveHeightMax)
	}
	if r.WaveDirectionDominant != 285 {
		t.Errorf("WaveDirectionDominant = %v, want 285", r.WaveDirectionDominant)
	}
	if r.WavePeriodMax != 10.5 {
		t.Errorf("WavePeriodMax = %v, want 10.5", r.WavePeriodMax)
	}
}

func TestRetryOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sampleResponse)
	}))
	defer ts.Close()

	cfg := marinemeteo.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Retries = 5
	c := marinemeteo.NewClient(cfg)

	rows, err := c.HourlyForecast(context.Background(), 48.8, -4.5, 1)
	if err != nil {
		t.Fatalf("HourlyForecast after retries: %v", err)
	}
	if len(rows) == 0 {
		t.Error("expected rows after recovery")
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}
