package config

import (
	"fmt"
	"os"
)

type AppConfig struct {
	APIVersion                string
	Port                      string
	DBURL                     string
	GinMode                   string
	MailgunAPIKey             string
	MailgunSendingDomain      string
	SenderName                string
	SenderEmail               string
	StripeSecretKey           string
	StripeSigningSecret       string
	StripeRefundSigningSecret string
	TeamName                  string
	TeamEmail                 string
}

func getEnvironmentVariable(key string) (string, error) {
	value := os.Getenv(key)

	if value == "" {
		return "", fmt.Errorf("environment variable %s not set", key)
	}

	return value, nil
}

func LoadEnvironmentVariables() (AppConfig, error) {
	appConfig := AppConfig{}

	var err error

	if appConfig.APIVersion, err = getEnvironmentVariable("API_VERSION"); err != nil {
		return appConfig, err
	}
	if appConfig.Port, err = getEnvironmentVariable("PORT"); err != nil {
		return appConfig, err
	}
	if appConfig.DBURL, err = getEnvironmentVariable("DB_URL"); err != nil {
		return appConfig, err
	}
	if appConfig.GinMode, err = getEnvironmentVariable("GIN_MODE"); err != nil {
		return appConfig, err
	}
	if appConfig.MailgunAPIKey, err = getEnvironmentVariable("MAILGUN_API_KEY"); err != nil {
		return appConfig, err
	}
	if appConfig.MailgunSendingDomain, err = getEnvironmentVariable("MAILGUN_SENDING_DOMAIN"); err != nil {
		return appConfig, err
	}
	if appConfig.SenderName, err = getEnvironmentVariable("SENDER_NAME"); err != nil {
		return appConfig, err
	}
	if appConfig.SenderEmail, err = getEnvironmentVariable("SENDER_EMAIL"); err != nil {
		return appConfig, err
	}
	if appConfig.StripeSecretKey, err = getEnvironmentVariable("STRIPE_SECRET_KEY"); err != nil {
		return appConfig, err
	}
	if appConfig.StripeSigningSecret, err = getEnvironmentVariable("STRIPE_SIGNING_SECRET"); err != nil {
		return appConfig, err
	}
	if appConfig.StripeRefundSigningSecret, err = getEnvironmentVariable("STRIPE_WEBHOOK_SECRET"); err != nil {
		return appConfig, err
	}
	if appConfig.StripeRefundSigningSecret, err = getEnvironmentVariable("STRIPE_REFUND_SIGNING_SECRET"); err != nil {
		return appConfig, err
	}
	if appConfig.TeamName, err = getEnvironmentVariable("TEAM_NAME"); err != nil {
		return appConfig, err
	}
	if appConfig.TeamEmail, err = getEnvironmentVariable("TEAM_EMAIL"); err != nil {
		return appConfig, err
	}

	return appConfig, nil
}