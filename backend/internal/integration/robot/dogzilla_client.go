package robot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"open-physical-ai-dojo/backend/internal/domain"
)

type DogzillaClient struct {
	baseURL string
	client  *http.Client
}

func (c *DogzillaClient) BaseURL() string {
	return c.baseURL
}

func NewDogzillaClient(baseURL string) *DogzillaClient {
	return &DogzillaClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *DogzillaClient) Health() error {
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
		return fmt.Errorf("dogzilla health returned %d", resp.StatusCode)
	}
	return nil
}

func (c *DogzillaClient) State() (domain.DogzillaState, error) {
	var state domain.DogzillaState
	resp, err := c.client.Get(c.baseURL + "/state")
	if err != nil {
		return state, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return state, fmt.Errorf("dogzilla state returned %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&state)
	return state, err
}

func (c *DogzillaClient) Stand() error {
	return c.post("/motion/stand", map[string]any{})
}

func (c *DogzillaClient) Sit() error {
	return c.post("/motion/sit", map[string]any{})
}

func (c *DogzillaClient) Move(step domain.ActionStep) error {
	return c.post("/motion/move", map[string]any{
		"linear_x":    step.LinearX,
		"linear_y":    step.LinearY,
		"yaw_deg":     step.YawDeg,
		"duration_ms": step.DurationMS,
	})
}

func (c *DogzillaClient) Stop() error {
	return c.post("/stop", map[string]any{})
}

func (c *DogzillaClient) post(path string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.client.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("dogzilla %s returned %d", path, resp.StatusCode)
	}
	return nil
}
