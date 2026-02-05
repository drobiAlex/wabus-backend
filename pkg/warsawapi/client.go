package warsawapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"wabus/internal/domain"
)

type Client struct {
	baseURL    string
	apiKey     string
	resourceID string
	httpClient *http.Client
}

func New(baseURL, apiKey, resourceID string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		resourceID: resourceID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

type apiResponse struct {
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
}

type apiVehicle struct {
	Lines         string  `json:"Lines"`
	Lon           float64 `json:"Lon"`
	VehicleNumber string  `json:"VehicleNumber"`
	Time          string  `json:"Time"`
	Lat           float64 `json:"Lat"`
	Brigade       string  `json:"Brigade"`
}

func (c *Client) Fetch(ctx context.Context, vehicleType domain.VehicleType) ([]*domain.Vehicle, error) {
	params := url.Values{}
	params.Set("resource_id", c.resourceID)
	params.Set("apikey", c.apiKey)
	params.Set("type", fmt.Sprintf("%d", vehicleType))

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if apiResp.Error != "" {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	var apiVehicles []apiVehicle
	if err := json.Unmarshal(apiResp.Result, &apiVehicles); err != nil {
		return nil, fmt.Errorf("decoding vehicles: %w", err)
	}

	return c.toDomain(apiVehicles, vehicleType), nil
}

func (c *Client) toDomain(apiVehicles []apiVehicle, vType domain.VehicleType) []*domain.Vehicle {
	result := make([]*domain.Vehicle, 0, len(apiVehicles))

	loc, _ := time.LoadLocation("Europe/Warsaw")

	for _, av := range apiVehicles {
		if av.VehicleNumber == "" {
			continue
		}

		ts, err := time.ParseInLocation("2006-01-02 15:04:05", av.Time, loc)
		if err != nil {
			ts = time.Now()
		}

		key := fmt.Sprintf("%d:%s", vType, av.VehicleNumber)
		result = append(result, &domain.Vehicle{
			Key:           key,
			VehicleNumber: av.VehicleNumber,
			Type:          vType,
			Line:          av.Lines,
			Brigade:       av.Brigade,
			Lat:           av.Lat,
			Lon:           av.Lon,
			Timestamp:     ts,
		})
	}

	return result
}
