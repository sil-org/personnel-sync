package main

import (
	"os"

	sync "github.com/sil-org/personnel-sync/v6"
)

func main() {
	configFile := ""
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}
	if err := sync.RunSync(configFile); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
