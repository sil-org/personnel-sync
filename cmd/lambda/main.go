package main

import (
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/getsentry/sentry-go"

	sync "github.com/sil-org/personnel-sync/v6"
)

type LambdaConfig struct {
	ConfigPath string
}

func main() {
	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		initSentry(dsn)
		defer sentry.Flush(2 * time.Second)
	}

	lambda.Start(handler)
}

func handler(lambdaConfig LambdaConfig) error {
	return sync.RunSync(lambdaConfig.ConfigPath)
}

func initSentry(dsn string) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		EnableLogs:  true,
		Environment: getEnv("APP_ENV", "production"),
	})
	if err != nil {
		log.Printf("sentry.Init failure: %s", err)
	}
}

func getEnv(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
