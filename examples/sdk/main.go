// Small end-to-end demo of the Hive Go SDK. Runs against a locally running
// hive (http://localhost:8233 by default) and exercises the common paths:
// list agents → register one → fire a workflow → poll its tasks.
//
//	go run ./examples/sdk -base http://localhost:8233 -key hive_XXXX
//
// Use `hive api-key create demo` to mint a key, or drop `-key` to exercise
// a dev deployment that has no keys configured.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/JulienLeotier/hive/sdk"
)

func main() {
	baseURL := flag.String("base", "http://localhost:8233", "Hive server base URL")
	apiKey := flag.String("key", "", "Hive API key (empty = dev mode)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c := sdk.NewClient(*baseURL, *apiKey)

	agents, err := c.Agents().List(ctx)
	if err != nil {
		log.Fatalf("list agents: %v", err)
	}
	fmt.Printf("registered agents: %d\n", len(agents))
	for _, a := range agents {
		fmt.Printf("  - %s [%s v%s] %s\n", a.Name, a.Type, versionOr(a.Version), a.HealthStatus)
	}

	events, err := c.Events().List(ctx, sdk.QueryOpts{Limit: 5})
	if err != nil {
		log.Fatalf("list events: %v", err)
	}
	fmt.Printf("\nlatest %d events:\n", len(events))
	for _, e := range events {
		fmt.Printf("  [%s] %-22s %s\n", e.CreatedAt.Format("15:04:05"), e.Type, e.Source)
	}
}

func versionOr(v string) string {
	if v == "" {
		return "1.0.0"
	}
	return v
}
