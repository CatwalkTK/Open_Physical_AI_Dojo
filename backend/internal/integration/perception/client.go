package perception

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"open-physical-ai-dojo/backend/internal/domain"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) Health() error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("perception health returned %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Run(req domain.PerceptionRequest) (domain.PerceptionResult, error) {
	var result domain.PerceptionResult
	body, err := json.Marshal(req)
	if err != nil {
		return result, err
	}
	resp, err := c.client.Post(c.baseURL+"/perception", "application/json", bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return result, fmt.Errorf("perception returned %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}
