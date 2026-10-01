package components

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/cloudflare/cloudflare-go/v7/workers"

	"cloudflui/internal/api"
	"cloudflui/internal/messages"
	"cloudflui/internal/styles"
)

// workersChrome is the number of rows the workers pane spends on its own
// chrome: the section title, a blank spacer, and the page indicator.
const workersChrome = 3

// WorkersPane lists Workers scripts and their deployments.
type WorkersPane struct {
	client         *api.Client
	accountID      string
	scripts        []workers.ScriptListResponse
	deployments    []workers.Deployment
	selectedScript string
	cursor         int
	width          int
	height         int
	loading        bool
	err            string
}

// NewWorkersPane creates the workers pane.
func NewWorkersPane(client *api.Client) *WorkersPane {
	return &WorkersPane{client: client, height: defaultViewportHeight}
}

// SetViewport records the content area the pane may render into.
func (p *WorkersPane) SetViewport(width, height int) {
	p.width = width
	p.height = height
}

// listRows returns how many list rows fit in the current viewport.
func (p *WorkersPane) listRows() int {
	return viewportRows(p.height, workersChrome)
}

// moveCursor sets the cursor, clamped to the active list bounds.
func (p *WorkersPane) moveCursor(to int) {
	p.cursor = clampCursor(to, p.listLen())
}

// IsEditing reports whether the pane is in text-entry mode.
func (p *WorkersPane) IsEditing() bool { return false }

// SetAccountID sets the account used for Workers API calls.
func (p *WorkersPane) SetAccountID(id string) {
	p.accountID = id
}

// Load fetches Workers scripts asynchronously.
func (p *WorkersPane) Load() tea.Cmd {
	if p.client == nil {
		p.err = "No API token configured. Add a profile in the Config pane."
		return nil
	}
	if p.accountID == "" {
		p.err = "No account ID configured. Set it in the Config pane."
		return nil
	}
	p.loading = true
	p.err = ""
	accountID := p.accountID
	return func() tea.Msg {
		scripts, err := p.client.ListWorkers(context.Background(), accountID)
		return messages.WorkersLoaded{Scripts: scripts, Err: err}
	}
}

func (p *WorkersPane) loadDeployments(scriptName string) tea.Cmd {
	if p.accountID == "" {
		p.err = "No account ID configured. Set it in the Config pane."
		return nil
	}
	p.loading = true
	p.err = ""
	accountID := p.accountID
	return func() tea.Msg {
		deployments, err := p.client.ListDeployments(context.Background(), accountID, scriptName)
		return messages.DeploymentsLoaded{ScriptName: scriptName, Deployments: deployments, Err: err}
	}
}

// Update handles messages for the workers pane.
func (p *WorkersPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case messages.WorkersLoaded:
		p.loading = false
		if msg.Err != nil {
			p.err = msg.Err.Error()
			return nil
		}
		p.err = ""
		p.scripts = msg.Scripts
		p.cursor = clampCursor(p.cursor, len(p.scripts))
	case messages.DeploymentsLoaded:
		p.loading = false
		if msg.Err != nil {
			p.err = msg.Err.Error()
			return nil
		}
		p.err = ""
		p.deployments = msg.Deployments
		p.selectedScript = msg.ScriptName
		p.cursor = 0
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			p.moveCursor(p.cursor - 1)
		case tea.KeyDown, 'j':
			p.moveCursor(p.cursor + 1)
		case tea.KeyPgUp:
			p.moveCursor(p.cursor - p.listRows())
		case tea.KeyPgDown:
			p.moveCursor(p.cursor + p.listRows())
		case 'g', tea.KeyHome:
			p.moveCursor(0)
		case 'G', tea.KeyEnd:
			p.moveCursor(p.listLen() - 1)
		case tea.KeyEnter:
			if p.selectedScript == "" {
				if len(p.scripts) == 0 {
					return nil
				}
				return p.loadDeployments(p.scripts[p.cursor].ID)
			}
		case tea.KeyEsc:
			if p.selectedScript != "" {
				p.selectedScript = ""
				p.deployments = nil
				p.cursor = 0
			}
		case 'r':
			if p.selectedScript != "" {
				return p.loadDeployments(p.selectedScript)
			}
			return p.Load()
		}
	}
	return nil
}

func (p *WorkersPane) listLen() int {
	if p.selectedScript != "" {
		return len(p.deployments)
	}
	return len(p.scripts)
}

// View renders the workers pane.
func (p *WorkersPane) View() string {
	var b strings.Builder
	if p.loading {
		b.WriteString(styles.Dimmed.Render("Loading..."))
		return b.String()
	}
	if p.err != "" {
		b.WriteString(styles.ErrorText.Render(p.err))
		return b.String()
	}

	if p.selectedScript != "" {
		b.WriteString(styles.Title.Render("Deployments: "+p.selectedScript) + "\n\n")
		if len(p.deployments) == 0 {
			b.WriteString(styles.Dimmed.Render("No deployments found."))
			return b.String()
		}
		rows := p.listRows()
		start, end := visibleWindow(len(p.deployments), p.cursor, rows)
		for i := start; i < end; i++ {
			line := fmt.Sprintf("%s  %s", shortTime(p.deployments[i].CreatedOn), p.deployments[i].Strategy)
			if i == p.cursor {
				b.WriteString(styles.Clip(styles.Selected.Render("> "+line), p.width))
			} else {
				b.WriteString(styles.Clip(styles.Dimmed.Render("  "+line), p.width))
			}
			b.WriteString("\n")
		}
		if indicator := pageIndicator(start, end, p.cursor, len(p.deployments), rows); indicator != "" {
			b.WriteString(styles.HelpBar.Render(indicator))
		}
		return b.String()
	}

	b.WriteString(styles.Title.Render("Workers Scripts") + "\n\n")
	if len(p.scripts) == 0 {
		b.WriteString(styles.Dimmed.Render("No scripts found. Press r to refresh."))
		return b.String()
	}
	rows := p.listRows()
	start, end := visibleWindow(len(p.scripts), p.cursor, rows)
	for i := start; i < end; i++ {
		line := fmt.Sprintf("%s  %s", shortTime(p.scripts[i].CreatedOn), p.scripts[i].ID)
		if i == p.cursor {
			b.WriteString(styles.Clip(styles.Selected.Render("> "+line), p.width))
		} else {
			b.WriteString(styles.Clip(styles.Dimmed.Render("  "+line), p.width))
		}
		b.WriteString("\n")
	}
	if indicator := pageIndicator(start, end, p.cursor, len(p.scripts), rows); indicator != "" {
		b.WriteString(styles.HelpBar.Render(indicator))
	}
	return b.String()
}

func shortTime(ts time.Time) string {
	return ts.Format("2006-01-02")
}
