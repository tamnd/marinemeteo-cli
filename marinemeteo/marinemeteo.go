// Package marinemeteo is the library behind the marinemeteo command line:
// the HTTP client, request shaping, and the typed data models for the
// Open-Meteo Marine Wave Forecast API (marine-api.open-meteo.com).
//
// The API is free and open-source; no API key or registration is required.
// It provides hourly and daily marine wave forecasts for any ocean coordinate.
package marinemeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Host is the API host this client talks to.
const Host = "marine-api.open-meteo.com"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://marine-api.open-meteo.com",
		UserAgent: "marinemeteo-cli/0.1.0 (github.com/tamnd/marinemeteo-cli)",
		Rate:      0,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client talks to the Open-Meteo Marine API over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// HourlyWave is one hour of marine wave forecast data.
type HourlyWave struct {
	Time            string  `json:"time" kit:"id"`
	WaveHeight      float64 `json:"wave_height"`
	WaveDirection   float64 `json:"wave_direction"`
	WavePeriod      float64 `json:"wave_period"`
	WindWaveHeight  float64 `json:"wind_wave_height"`
	SwellWaveHeight float64 `json:"swell_wave_height"`
}

// DailyWave is one day of marine wave forecast data.
type DailyWave struct {
	Time                  string  `json:"time" kit:"id"`
	WaveHeightMax         float64 `json:"wave_height_max"`
	WaveDirectionDominant float64 `json:"wave_direction_dominant"`
	WavePeriodMax         float64 `json:"wave_period_max"`
}

// HourlyForecast fetches an hourly marine wave forecast for the given coordinates.
// days must be >= 1; values <= 0 default to 3.
func (c *Client) HourlyForecast(ctx context.Context, lat, lon float64, days int) ([]HourlyWave, error) {
	if days <= 0 {
		days = 3
	}
	u := fmt.Sprintf(
		"%s/v1/marine?latitude=%s&longitude=%s&hourly=wave_height,wave_direction,wave_period,wind_wave_height,swell_wave_height&forecast_days=%d",
		c.cfg.BaseURL,
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(lon, 'f', -1, 64),
		days,
	)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp wireMarineResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode hourly marine forecast: %w", err)
	}
	h := resp.Hourly
	n := len(h.Time)
	out := make([]HourlyWave, 0, n)
	for i := 0; i < n; i++ {
		row := HourlyWave{Time: h.Time[i]}
		if i < len(h.WaveHeight) {
			row.WaveHeight = h.WaveHeight[i]
		}
		if i < len(h.WaveDirection) {
			row.WaveDirection = h.WaveDirection[i]
		}
		if i < len(h.WavePeriod) {
			row.WavePeriod = h.WavePeriod[i]
		}
		if i < len(h.WindWaveHeight) {
			row.WindWaveHeight = h.WindWaveHeight[i]
		}
		if i < len(h.SwellWaveHeight) {
			row.SwellWaveHeight = h.SwellWaveHeight[i]
		}
		out = append(out, row)
	}
	return out, nil
}

// DailyForecast fetches a daily marine wave forecast for the given coordinates.
// days must be >= 1; values <= 0 default to 7.
func (c *Client) DailyForecast(ctx context.Context, lat, lon float64, days int) ([]DailyWave, error) {
	if days <= 0 {
		days = 7
	}
	u := fmt.Sprintf(
		"%s/v1/marine?latitude=%s&longitude=%s&daily=wave_height_max,wave_direction_dominant,wave_period_max&forecast_days=%d",
		c.cfg.BaseURL,
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(lon, 'f', -1, 64),
		days,
	)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp wireMarineResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode daily marine forecast: %w", err)
	}
	d := resp.Daily
	n := len(d.Time)
	out := make([]DailyWave, 0, n)
	for i := 0; i < n; i++ {
		row := DailyWave{Time: d.Time[i]}
		if i < len(d.WaveHeightMax) {
			row.WaveHeightMax = d.WaveHeightMax[i]
		}
		if i < len(d.WaveDirectionDominant) {
			row.WaveDirectionDominant = d.WaveDirectionDominant[i]
		}
		if i < len(d.WavePeriodMax) {
			row.WavePeriodMax = d.WavePeriodMax[i]
		}
		out = append(out, row)
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}

// --- internal wire types ---

type wireMarineResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Hourly    struct {
		Time            []string  `json:"time"`
		WaveHeight      []float64 `json:"wave_height"`
		WaveDirection   []float64 `json:"wave_direction"`
		WavePeriod      []float64 `json:"wave_period"`
		WindWaveHeight  []float64 `json:"wind_wave_height"`
		SwellWaveHeight []float64 `json:"swell_wave_height"`
	} `json:"hourly"`
	Daily struct {
		Time                  []string  `json:"time"`
		WaveHeightMax         []float64 `json:"wave_height_max"`
		WaveDirectionDominant []float64 `json:"wave_direction_dominant"`
		WavePeriodMax         []float64 `json:"wave_period_max"`
	} `json:"daily"`
}
