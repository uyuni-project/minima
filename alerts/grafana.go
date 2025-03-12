package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	alertsEndpoint      = "/alerts"
	healthCheckEndpoint = "/health"
)

type GrafanaConfig struct {
	Enabled    bool
	AlertTitle string
	APIUrl     string
	APIKey     string
}

type GrafanaAlerter struct {
	httpClient *http.Client
	key        string
	apiURL     string
}

type AlertPayload struct {
	Title       string `json:"title"`
	Message     string `json:"message"`
	AlertStatus string `json:"status"`
}

func NewGrafanaAlerter(config GrafanaConfig) (*GrafanaAlerter, error) {
	ga := &GrafanaAlerter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiURL:     config.APIUrl,
		key:        config.APIKey,
	}

	if err := ga.checkHealth(); err != nil {
		return nil, err
	}
	return ga, nil
}

// Alerter interface implementation
func (g *GrafanaAlerter) SendAlert(title, content string) error {
	fmt.Println("Sending Grafana alert")

	if err := g.postAlert(title, content); err != nil {
		return fmt.Errorf("failed to send Grafana alert: %v", err)
	}
	return nil
}

// CheckHealth verifies if the Grafana API is reachable
func (g *GrafanaAlerter) checkHealth() error {
	req, err := http.NewRequest("GET", g.apiURL+healthCheckEndpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.key)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed for Grafana API, status code: %d", resp.StatusCode)
	}

	fmt.Println("Grafana API is healthy!")
	return nil
}

func (g *GrafanaAlerter) postAlert(title, msg string) error {
	alert := AlertPayload{
		Title:       title,
		Message:     msg,
		AlertStatus: "alerting",
	}

	jsonData, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", g.apiURL+alertsEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.key)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send Grafana alert, status code: %d", resp.StatusCode)
	}
	return nil
}
