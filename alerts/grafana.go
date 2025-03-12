package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
	alertsUrl  string
}

type AlertPayload struct {
	Title       string `json:"title"`
	Message     string `json:"message"`
	AlertStatus string `json:"status"`
}

func NewGrafanaAlerter(config GrafanaConfig) *GrafanaAlerter {
	client := &http.Client{Timeout: 30 * time.Second}

	return &GrafanaAlerter{
		httpClient: client,
		alertsUrl:  config.APIUrl + "alerts",
		key:        config.APIKey,
	}
}

// Alerter interface implementation
func (g *GrafanaAlerter) SendAlert(title, content string) error {
	fmt.Println("Sending Grafana alert")

	if err := g.postGrafanaAlert(title, content); err != nil {
		return fmt.Errorf("failed to send Grafana alert: %v", err)
	}
	return nil
}

func (g *GrafanaAlerter) postGrafanaAlert(title, msg string) error {
	alert := AlertPayload{
		Title:       title,
		Message:     msg,
		AlertStatus: "alerting",
	}

	jsonData, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", g.alertsUrl, bytes.NewBuffer(jsonData))
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
