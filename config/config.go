package config

import (
	"fmt"
	"os"
)

type AppConfig struct {
	APIVersion           string
	Port                 string
	DBURL                string
	GINMode              string
	MailgunAPIKey        string
	MailgunSendingDomain string
	SenderName           string
	SenderEmail          string
	StripeSecretKey      string
	StripeSigningSecret  string
}

func Load() (AppConfig, error) {
	appConfig := AppConfig{}

	var err error
	
	getEnv := func(key string) (string, error) {
		value := os.Getenv(key)
		if value == "" {
			return "", fmt.Errorf("environment variable %s not set", key)
		}
		return value, nil
	}

	if appConfig.APIVersion, err = getEnv("API_VERSION"); err != nil { return appConfig, err }
	if appConfig.Port, err = getEnv("PORT"); err != nil { return appConfig, err }
	if appConfig.DBURL, err = getEnv("DB_URL"); err != nil { return appConfig, err }
	if appConfig.GINMode, err = getEnv("GIN_MODE"); err != nil { return appConfig, err }
	if appConfig.MailgunAPIKey, err = getEnv("MAILGUN_API_KEY"); err != nil { return appConfig, err }
	if appConfig.MailgunSendingDomain, err = getEnv("MAILGUN_SENDING_DOMAIN"); err != nil { return appConfig, err }
	if appConfig.SenderName, err = getEnv("SENDER_NAME"); err != nil { return appConfig, err }
	if appConfig.SenderEmail, err = getEnv("SENDER_EMAIL"); err != nil { return appConfig, err }
	if appConfig.StripeSecretKey, err = getEnv("STRIPE_SECRET_KEY"); err != nil { return appConfig, err }
	if appConfig.StripeSigningSecret, err = getEnv("STRIPE_SIGNING_SECRET"); err != nil { return appConfig, err }

	return appConfig, nil
}