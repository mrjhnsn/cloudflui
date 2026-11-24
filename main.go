// Command cloudflui is a headless backend for managing Cloudflare DNS zones,
// records, and Workers deployments. It speaks newline-delimited JSON-RPC on
// stdin/stdout and is driven by the OpenTUI frontend in ui/.
package main

import (
	"fmt"
	"os"

	"cloudflui/internal/backend"
)

func main() {
	b, err := backend.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cloudflui:", err)
		os.Exit(1)
	}

	srv := backend.NewServer(b)
	if err := srv.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "cloudflui:", err)
		os.Exit(1)
	}
}
