// Package model contains the root Bubble Tea model for cloudflui.
package model

import (
	"charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"cloudflui/internal/api"
	"cloudflui/internal/components"
	"cloudflui/internal/config"
)

// Pane is the interface implemented by all five panes.
type Pane interface {
	Update(msg tea.Msg) tea.Cmd
	View() string
	// IsEditing reports whether the pane is currently in text-entry mode.
	// While editing, the root model must not intercept navigation or quit
	// keys so that every character reaches the text field.
	IsEditing() bool
}

// ViewportPane is implemented by panes that render a scrollable list. The
// root model hands them the content area they may draw into so they can
// window long lists to the space actually available. Panes that render fixed
// content (the editor and config forms) deliberately do not implement it.
type ViewportPane interface {
	Pane
	SetViewport(width, height int)
}

// Pane indices.
const (
	PaneZones = iota
	PaneRecords
	PaneEditor
	PaneConfig
	PaneWorkers
	paneCount
)

// Model is the root Bubble Tea model.
type Model struct {
	cfg           *config.Config
	client        *api.Client
	activeProfile string // session-only active profile override
	active        int

	zones   *components.ZonesPane
	records *components.RecordsPane
	editor  *components.EditorPane
	config  *components.ConfigPane
	workers *components.WorkersPane

	status string
	width  int
	height int
}

// New creates the root model, loading config and building the client.
func New() (*Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	m := &Model{cfg: cfg}
	m.rebuildClient()
	m.rebuildPanes()
	return m, nil
}

// currentProfile returns the session-active profile, falling back to the
// configured default (or first profile).
func (m *Model) currentProfile() *config.Profile {
	if m.activeProfile != "" {
		if prof := m.cfg.Profile(m.activeProfile); prof != nil {
			return prof
		}
	}
	return m.cfg.Active()
}

// rebuildClient recreates the API client from the active profile.
func (m *Model) rebuildClient() {
	prof := m.currentProfile()
	if prof == nil {
		m.client = nil
		m.status = "No profile configured. Add one in the Config pane (tab 4)."
		return
	}
	client, err := api.New(prof.APIKey)
	if err != nil {
		m.client = nil
		m.status = "Invalid API token for profile " + prof.Name
		return
	}
	m.client = client
	m.status = "Profile: " + prof.Name
}

// rebuildPanes recreates all panes with the current client and config.
func (m *Model) rebuildPanes() {
	m.zones = components.NewZonesPane(m.client)
	m.records = components.NewRecordsPane(m.client)
	m.editor = components.NewEditorPane(m.client)
	m.config = components.NewConfigPane(m.cfg)
	m.workers = components.NewWorkersPane(m.client)
	if prof := m.currentProfile(); prof != nil {
		m.workers.SetAccountID(prof.AccountID)
		m.config.SetActiveProfile(prof.Name)
	}
	// Fresh panes start at the default size, so re-sync them to the current
	// terminal to avoid a single oversized frame after a profile switch.
	m.syncViewports()
}

// syncViewports hands the current content area to every pane that paginates.
// Before the terminal reports a size there is nothing to measure, so panes
// keep the default height they were built with.
func (m *Model) syncViewports() {
	if m.height <= 0 {
		return
	}
	for _, p := range m.allPanes() {
		if vp, ok := p.(ViewportPane); ok {
			vp.SetViewport(m.paneContentWidth(), m.paneContentHeight())
		}
	}
}

// activePane returns the currently focused pane.
func (m *Model) activePane() Pane {
	switch m.active {
	case PaneRecords:
		return m.records
	case PaneEditor:
		return m.editor
	case PaneConfig:
		return m.config
	case PaneWorkers:
		return m.workers
	default:
		return m.zones
	}
}

// allPanes returns every pane for broadcast updates.
func (m *Model) allPanes() []Pane {
	return []Pane{m.zones, m.records, m.editor, m.config, m.workers}
}

// Init starts the initial data loads.
func (m *Model) Init() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return tea.Batch(m.zones.Load(), m.workers.Load())
}

// isQuitKey reports whether the key press should quit the app.
func isQuitKey(msg tea.KeyPressMsg) bool {
	if msg.Code == 'q' {
		return true
	}
	return msg.Mod.Contains(uv.ModCtrl) && msg.Code == 'c'
}
