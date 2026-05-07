package main

import (
	"context"
	"log"
	"time"

	"github.com/gustavodetoni/pullsing/sdk/go/client"
)

func main() {
	ctx := context.Background()

	sdk, err := client.NewClient(ctx, client.Config{
		Addr:   "localhost:50051",
		EnvKey: "dev-secret-key",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := sdk.Close(); err != nil {
			log.Printf("close sdk: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	log.Printf("new_button enabled=%t", sdk.Enabled("new_button"))
}
