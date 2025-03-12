package alerts

import (
	"fmt"
	"log"
)

type Alerter interface {
	SendAlert(title, content string) error
}

type AlertsConfig struct {
	FailOnError bool
	Gmail       MailerConfig
	Grafana     GrafanaConfig
}

type AlertsManager struct {
	failOnError bool
	alerters    []Alerter
}

func NewAlertsManager(config AlertsConfig) (*AlertsManager, error) {
	alerters := []Alerter{}

	if config.Gmail.Enabled {
		log.Println("Initializig Gmail alerter ...")
		gmailAlerter, err := NewGmailAlerter(config.Gmail)
		if err != nil {
			return nil, err
		}

		alerters = append(alerters, gmailAlerter)
		log.Println("Gmail alerter ready")
	}

	if config.Grafana.Enabled {
		log.Println("Initializig Grafana alerter ...")
		grafanaAlerter := NewGrafanaAlerter(config.Grafana)
		alerters = append(alerters, grafanaAlerter)
		log.Println("Grafana alerter ready")
	}

	return &AlertsManager{
		failOnError: config.FailOnError,
		alerters:    alerters,
	}, nil
}

func (am *AlertsManager) DispatchAlert(repo string, cause error) error {
	title := fmt.Sprintf("Minima - failure for %s", repo)
	content := fmt.Sprintf("Cause: %v", cause)

	for _, a := range am.alerters {
		if err := a.SendAlert(title, content); err != nil {
			log.Printf("Alerter error: %v\n", err)
			if am.failOnError {
				return err
			}
		}
	}
	return nil
}
