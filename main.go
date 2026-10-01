// Command cloudflui is a TUI for managing Cloudflare DNS zones, records,
// and Workers deployments.
package main

import (
	"fmt"
	"os"

	"charm.land/bubbletea/v2"

	"cloudflui/internal/model"
)

func main() {
	m, err := model.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cloudflui:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "cloudflui:", err)
		os.Exit(1)
	}
}
