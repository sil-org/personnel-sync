package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"

	sync "github.com/sil-org/personnel-sync/v8"
)

type LambdaConfig struct {
	ConfigPath string
}

func main() {
	lambda.Start(handler)
}

// handler receives a ctx whose deadline is the Lambda timeout, so a sync that runs long is stopped (and alerted on)
// before the Lambda is killed.
func handler(ctx context.Context, lambdaConfig LambdaConfig) error {
	return sync.RunSync(ctx, lambdaConfig.ConfigPath)
}
