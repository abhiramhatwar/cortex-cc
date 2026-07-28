package sentiment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls the HuggingFace sentiment microservice.
type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Score returns a sentiment float in [-1.0, 1.0] for the given text.
// Negative = bad customer experience, positive = good.
func (c *Client) Score(text string) (float64, error) {
	body, _ := json.Marshal(map[string]string{"text": text})
	resp, err := c.http.Post(c.baseURL+"/analyze", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("sentiment service unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("sentiment error %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, fmt.Errorf("parse sentiment response: %w", err)
	}
	return result.Score, nil
}

// ScoreBatch scores multiple texts in a single HTTP call.
// Returns scores in the same order as the input slice.
func (c *Client) ScoreBatch(texts []string) ([]float64, error) {
	body, _ := json.Marshal(map[string]any{"texts": texts})
	resp, err := c.http.Post(c.baseURL+"/analyze/batch", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sentiment service unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sentiment batch error %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Results []struct {
			Score float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse batch response: %w", err)
	}

	scores := make([]float64, len(result.Results))
	for i, r := range result.Results {
		scores[i] = r.Score
	}
	return scores, nil
}

// Ping checks if the sentiment service is reachable.
func (c *Client) Ping() error {
	resp, err := c.http.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("sentiment service not reachable at %s: %w", c.baseURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sentiment health check failed: %d", resp.StatusCode)
	}
	return nil
}
