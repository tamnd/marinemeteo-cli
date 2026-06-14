// Package marinemeteo exposes the Open-Meteo Marine Wave Forecast API as a
// kit Domain driver.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/marinemeteo-cli/marinemeteo"
//
// The same Domain also builds the standalone marinemeteo binary (see cli.NewApp).
package marinemeteo

import (
	"context"
	"fmt"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the marinemeteo driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "marinemeteo",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "marinemeteo",
			Short:  "Marine wave forecasts from Open-Meteo",
			Long: `marinemeteo fetches hourly and daily marine wave forecasts
from the free Open-Meteo Marine API (marine-api.open-meteo.com).
No API key or registration required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/marinemeteo-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// hourly: fetch hourly marine wave forecast
	kit.Handle(app, kit.OpMeta{
		Name:    "hourly",
		Group:   "read",
		List:    true,
		Summary: "Fetch hourly marine wave forecast for a location",
	}, hourlyOp)

	// daily: fetch daily marine wave forecast
	kit.Handle(app, kit.OpMeta{
		Name:    "daily",
		Group:   "read",
		List:    true,
		Summary: "Fetch daily marine wave forecast for a location",
	}, dailyOp)
}

// newClient builds the client from host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type hourlyInput struct {
	Lat    float64 `kit:"flag" help:"latitude in decimal degrees"`
	Lon    float64 `kit:"flag" help:"longitude in decimal degrees"`
	Days   int     `kit:"flag" help:"forecast days (default 3)"`
	Client *Client `kit:"inject"`
}

type dailyInput struct {
	Lat    float64 `kit:"flag" help:"latitude in decimal degrees"`
	Lon    float64 `kit:"flag" help:"longitude in decimal degrees"`
	Days   int     `kit:"flag" help:"forecast days (default 7)"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func hourlyOp(ctx context.Context, in hourlyInput, emit func(HourlyWave) error) error {
	days := in.Days
	if days <= 0 {
		days = 3
	}
	rows, err := in.Client.HourlyForecast(ctx, in.Lat, in.Lon, days)
	if err != nil {
		return mapErr(err)
	}
	for _, row := range rows {
		if err := emit(row); err != nil {
			return err
		}
	}
	return nil
}

func dailyOp(ctx context.Context, in dailyInput, emit func(DailyWave) error) error {
	days := in.Days
	if days <= 0 {
		days = 7
	}
	rows, err := in.Client.DailyForecast(ctx, in.Lat, in.Lon, days)
	if err != nil {
		return mapErr(err)
	}
	for _, row := range rows {
		if err := emit(row); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
// Inputs matching "lat,lon" (two floats separated by a comma) are classified as
// "latlon"; anything else is classified as "query".
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty marinemeteo reference")
	}
	parts := strings.SplitN(input, ",", 2)
	if len(parts) == 2 {
		p0 := strings.TrimSpace(parts[0])
		p1 := strings.TrimSpace(parts[1])
		if isFloat(p0) && isFloat(p1) {
			return "latlon", input, nil
		}
	}
	return "query", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "latlon":
		parts := strings.SplitN(id, ",", 2)
		if len(parts) != 2 {
			return "", errs.Usage("invalid latlon id %q", id)
		}
		lat := strings.TrimSpace(parts[0])
		lon := strings.TrimSpace(parts[1])
		return fmt.Sprintf("https://%s/v1/marine?latitude=%s&longitude=%s", Host, lat, lon), nil
	case "query":
		return fmt.Sprintf("https://%s/v1/marine?q=%s", Host, id), nil
	default:
		return "", errs.Usage("marinemeteo has no resource type %q", uriType)
	}
}

// --- helpers ---

func isFloat(s string) bool {
	if s == "" {
		return false
	}
	// allow leading minus, digits, and one dot
	dot := false
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' {
			if dot {
				return false
			}
			dot = true
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
