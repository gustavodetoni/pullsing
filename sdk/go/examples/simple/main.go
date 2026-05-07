package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/gustavodetoni/pullsing/sdk/go/client"
)

const (
	defaultAddr    = "localhost:50051"
	defaultFlagKey = "checkout-redesign"
)

func main() {
	ctx := context.Background()

	cfg, flagKey, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	sdk, err := client.NewClient(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := sdk.Close(); err != nil {
			log.Printf("close sdk: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	log.Printf("%s enabled=%t", flagKey, sdk.Enabled(flagKey))
}

func loadConfig() (client.Config, string, error) {
	envAPIKey := firstNonEmpty(os.Getenv("PULLSING_API_KEY"), os.Getenv("PULLSING_ENV_API_KEY"))
	if envAPIKey == "" {
		return client.Config{}, "", errors.New(
			"set PULLSING_API_KEY to the api_key returned by POST /v1/projects/{id}/environments",
		)
	}

	addr := os.Getenv("PULLSING_ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	flagKey := os.Getenv("PULLSING_FLAG_KEY")
	if flagKey == "" {
		flagKey = defaultFlagKey
	}

	return client.Config{
		Addr:      addr,
		EnvAPIKey: envAPIKey,
	}, flagKey, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
