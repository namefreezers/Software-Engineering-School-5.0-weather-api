package openweathermap

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/config"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather/types"
	"net/http"
)

type Client struct {
	apiKey string
}

func NewClient(cfg *config.Config) (*Client, error) {
	key := cfg.OpenWeatherMapOrgKey // might be missing
	if key == "" {
		return nil, fmt.Errorf("OPENWEATHERMAP_ORG_API_KEY is not set")
	}
	return &Client{apiKey: key}, nil
}

func (c *Client) FetchCurrent(ctx context.Context, city string) (types.Weather, error) {
	url := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric",
		city, c.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return types.Weather{}, fmt.Errorf("openweathermap: failed to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return types.Weather{}, fmt.Errorf("openweathermap: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Weather{}, fmt.Errorf(
			"openweathermap: unexpected status %d %s",
			resp.StatusCode, http.StatusText(resp.StatusCode),
		)
	}

	var body struct {
		Main struct {
			Temp     float64 `json:"temp"`
			Humidity int     `json:"humidity"`
		} `json:"main"`
		Weather []struct {
			Description string `json:"description"`
		} `json:"weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return types.Weather{}, fmt.Errorf("openweathermap: JSON decode error: %w", err)
	}
	if len(body.Weather) == 0 {
		return types.Weather{}, fmt.Errorf("openweathermap: no weather data in response")
	}

	return types.Weather{
		Temp:        body.Main.Temp,
		Humidity:    body.Main.Humidity,
		Description: body.Weather[0].Description,
	}, nil
}
