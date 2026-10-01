package model

import (
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"

	"cloudflui/internal/config"
	"cloudflui/internal/messages"
)

// TestGlobalKeysIgnoredWhileEditing verifies that quit, tab, and number keys
// are not intercepted by the root model while a pane is in text-entry mode.
func TestGlobalKeysIgnoredWhileEditing(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	m.active = PaneConfig

	// Open the new-profile form and start editing the name field.
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"}); cmd != nil {
		t.Fatalf("unexpected cmd from 'n': %v", cmd)
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatalf("unexpected cmd from Enter: %v", cmd)
	}
	if !m.config.IsEditing() {
		t.Fatal("expected config pane to be editing a field")
	}

	// 'q' must not quit while editing.
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil {
		t.Fatalf("expected nil cmd for 'q' while editing, got %v", cmd)
	}

	// Tab must not switch panes while editing.
	before := m.active
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.active != before {
		t.Fatalf("Tab switched pane while editing: %d -> %d", before, m.active)
	}

	// Number keys must not switch panes while editing.
	m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	if m.active != before {
		t.Fatalf("number key switched pane while editing: %d -> %d", before, m.active)
	}
}

// TestQuitKeyWorksWhenNotEditing verifies 'q' still quits outside edit mode.
func TestQuitKeyWorksWhenNotEditing(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	m.active = PaneConfig

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected quit cmd when not editing")
	}
}

// TestTabSwitchesPanesWhenNotEditing verifies Tab still switches panes
// outside edit mode.
func TestTabSwitchesPanesWhenNotEditing(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	m.active = PaneZones

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.active != PaneRecords {
		t.Fatalf("Tab did not switch pane: got %d, want %d", m.active, PaneRecords)
	}
}

// TestPasteBroadcastToEditingPane verifies a PasteMsg is broadcast to the
// active pane and lands in the field being edited.
func TestPasteBroadcastToEditingPane(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	m.active = PaneConfig
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if _, cmd := m.Update(tea.PasteMsg{Content: "pasted-token"}); cmd != nil {
		t.Fatalf("unexpected cmd from PasteMsg: %v", cmd)
	}

	if view := m.View().Content; !strings.Contains(view, "pasted-token") {
		t.Fatalf("expected pasted text in view, got:\n%s", view)
	}
}

// testModel builds a model with two profiles and a working client.
func testModel(t *testing.T) *Model {
	t.Helper()
	m := &Model{
		cfg: &config.Config{
			DefaultProfile: "main",
			Profiles: []config.Profile{
				{Name: "main", APIKey: "key1", AccountID: "acct1"},
				{Name: "backup", APIKey: "key2", AccountID: "acct2"},
			},
		},
	}
	m.rebuildClient()
	m.rebuildPanes()
	return m
}

// TestActivateProfile verifies ActivateProfile switches the session profile
// and rebuilds the client and panes.
func TestActivateProfile(t *testing.T) {
	m := testModel(t)

	_, cmd := m.Update(messages.ActivateProfile{Name: "backup"})
	if cmd == nil {
		t.Fatal("expected load cmd after activating profile")
	}
	if m.activeProfile != "backup" {
		t.Fatalf("activeProfile = %q, want backup", m.activeProfile)
	}
	if m.status != "Active profile: backup" {
		t.Fatalf("status = %q, want Active profile: backup", m.status)
	}
	// The config pane should now mark backup as active.
	if view := m.config.View(); !strings.Contains(view, "(active)") {
		t.Fatalf("expected (active) marker in config view:\n%s", view)
	}
}

// TestActivateProfileUnknown verifies an unknown profile is rejected.
func TestActivateProfileUnknown(t *testing.T) {
	m := testModel(t)

	_, cmd := m.Update(messages.ActivateProfile{Name: "missing"})
	if cmd != nil {
		t.Fatalf("expected nil cmd for unknown profile, got %v", cmd)
	}
	if m.activeProfile != "" {
		t.Fatalf("activeProfile = %q, want empty", m.activeProfile)
	}
}

// TestSetDefaultProfile verifies SetDefaultProfile persists the default and
// makes it the active profile.
func TestSetDefaultProfile(t *testing.T) {
	config.SetPath(filepath.Join(t.TempDir(), "config.toml"))
	defer config.SetPath("")

	m := testModel(t)

	_, cmd := m.Update(messages.SetDefaultProfile{Name: "backup"})
	if cmd == nil {
		t.Fatal("expected load cmd after setting default")
	}
	if m.cfg.DefaultProfile != "backup" {
		t.Fatalf("DefaultProfile = %q, want backup", m.cfg.DefaultProfile)
	}
	if m.activeProfile != "backup" {
		t.Fatalf("activeProfile = %q, want backup", m.activeProfile)
	}
	if m.status != "Default profile: backup" {
		t.Fatalf("status = %q, want Default profile: backup", m.status)
	}
}
