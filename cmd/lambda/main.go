package main

import (
	"github.com/aws/aws-lambda-go/lambda"

	sync "github.com/sil-org/personnel-sync/v6"
)

type LambdaConfig struct {
	ConfigPath string
}

func main() {
	lambda.Start(handler)
}

func handler(lambdaConfig LambdaConfig) error {
	return sync.RunSync(lambdaConfig.ConfigPath)
}
