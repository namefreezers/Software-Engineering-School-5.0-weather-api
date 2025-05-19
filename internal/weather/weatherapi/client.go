package weatherapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/config"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather/types"
	"net/http"
)

// Client queries the WeatherAPI.com current.json endpoint.
type Client struct {
	apiKey string
}

// NewClient returns a new Client, or an error if the API key is not set.
func NewClient(cfg *config.Config) (*Client, error) {
	key := cfg.WeatherAPIComKey // might be missing
	if key == "" {
		return nil, fmt.Errorf("environment variable WEATHERAPI_COM_API_KEY is not set")
	}
	return &Client{apiKey: key}, nil
}

// FetchCurrent implements weather.Fetcher.
// It returns temperature (°C), humidity (%), and a brief description.
func (c *Client) FetchCurrent(ctx context.Context, city string) (types.Weather, error) {
	url := fmt.Sprintf(
		"http://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no",
		c.apiKey, city,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return types.Weather{}, fmt.Errorf("weatherapi: failed to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return types.Weather{}, fmt.Errorf("weatherapi: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Weather{}, fmt.Errorf(
			"weatherapi: unexpected status %d %s",
			resp.StatusCode, http.StatusText(resp.StatusCode),
		)
	}

	var body struct {
		Current struct {
			TempC     float64 `json:"temp_c"`
			Humidity  int     `json:"humidity"`
			Condition struct {
				Text string `json:"text"`
			} `json:"condition"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return types.Weather{}, fmt.Errorf("weatherapi: JSON decode error: %w", err)
	}

	return types.Weather{
		Temp:        body.Current.TempC,
		Humidity:    body.Current.Humidity,
		Description: body.Current.Condition.Text,
	}, nil
}
