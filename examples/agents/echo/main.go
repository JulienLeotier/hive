// Echo agent — smallest possible Hive adapter. Every request comes back
// as the output so operators can smoke-test routing, capability matching,
// and workflow DAGs without any external API dependency.
//
// Run it:
//
//	go run .                       # listens on :9100
//	PORT=8099 go run .             # custom port
//
// Then register against a running hive:
//
//	hive add-agent --name echo --type http --url http://localhost:9100
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type capabilities struct {
	Name       string   `json:"name"`
	TaskTypes  []string `json:"task_types"`
	CostPerRun float64  `json:"cost_per_run,omitempty"`
	Version    string   `json:"version,omitempty"`
}

type task struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Input any    `json:"input"`
}

type result struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func main() {
	port := flag.String("port", envOr("PORT", "9100"), "port to listen on")
	flag.Parse()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "healthy"})
	})

	http.HandleFunc("/declare", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, capabilities{
			Name:       "echo",
			TaskTypes:  []string{"echo", "debug"},
			CostPerRun: 0.0,
			Version:    "1.0.0",
		})
	})

	http.HandleFunc("/invoke", func(w http.ResponseWriter, r *http.Request) {
		var t task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeJSON(w, result{TaskID: t.ID, Status: "failed", Error: "decode: " + err.Error()})
			return
		}
		writeJSON(w, result{
			TaskID: t.ID,
			Status: "completed",
			Output: map[string]any{
				"echoed_at":  time.Now().UTC().Format(time.RFC3339),
				"task_type":  t.Type,
				"input":      t.Input,
				"agent_name": "echo",
			},
		})
	})

	addr := ":" + *port
	log.Printf("echo agent listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	_ = fmt.Sprintf // silence
	return fallback
}
