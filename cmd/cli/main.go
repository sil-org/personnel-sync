package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	sync "github.com/sil-org/personnel-sync/v8"
)

func main() {
	configFile := ""
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	// Stop the sync cleanly (and report on it) on Ctrl-C or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := sync.RunSync(ctx, configFile)
	stop()
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
