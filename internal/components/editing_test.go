package components

import (
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/cloudflare/cloudflare-go/v7/zones"

	"cloudflui/internal/config"
	"cloudflui/internal/messages"
)

func zoneFixture() zones.Zone {
	return zones.Zone{ID: "zone-1", Name: "example.com", Status: zones.ZoneStatusActive}
}

// TestConfigPaneEditingAcceptsAllCharacters verifies that characters used for
// navigation or actions elsewhere in the app are accepted as literal text
// while editing a field.
func TestConfigPaneEditingAcceptsAllCharacters(t *testing.T) {
	p := NewConfigPane(&config.Config{})
	p.startNew()
	p.editing = true

	input := "jack q1n5dpsr"
	for _, r := range input {
		p.handleEditingKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	if got := p.fields[0]; got != input {
		t.Fatalf("field value = %q, want %q", got, input)
	}
}

// TestConfigPaneEditingEnterConfirms verifies Enter ends field editing and
// keeps the typed text.
func TestConfigPaneEditingEnterConfirms(t *testing.T) {
	p := NewConfigPane(&config.Config{})
	p.startNew()
	p.editing = true
	p.handleEditingKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	p.handleEditingKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if p.editing {
		t.Fatal("expected editing to end after Enter")
	}
	if got := p.fields[0]; got != "j" {
		t.Fatalf("field value = %q, want %q", got, "j")
	}
}

// TestConfigPaneEditingEscCancels verifies Esc ends field editing without
// committing the typed text.
func TestConfigPaneEditingEscCancels(t *testing.T) {
	p := NewConfigPane(&config.Config{})
	p.startNew()
	p.editing = true
	p.handleEditingKey(tea.KeyPressMsg{Code: 'x', Text: "x"})
	p.handleEditingKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	if p.editing {
		t.Fatal("expected editing to end after Esc")
	}
	if got := p.fields[0]; got != "x" {
		t.Fatalf("field value = %q, want %q", got, "x")
	}
}

// TestConfigPaneFormNavigation verifies the full create-profile flow: open the
// form, edit the name field, confirm it, move to the next field, and edit it.
func TestConfigPaneFormNavigation(t *testing.T) {
	p := NewConfigPane(&config.Config{})
	p.startNew()

	if !p.formMode || p.editing {
		t.Fatal("expected form mode with no field being edited")
	}

	// Enter starts editing the name field.
	p.handleFormKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !p.editing {
		t.Fatal("expected editing after Enter")
	}

	// Type a name.
	for _, r := range "main" {
		p.handleEditingKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	// Enter confirms the field but stays in the form.
	p.handleFormKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if p.editing {
		t.Fatal("expected editing to end after Enter")
	}
	if !p.formMode {
		t.Fatal("expected to stay in form mode after confirming field")
	}

	// j moves to the next field.
	p.handleFormKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if p.fieldCursor != 1 {
		t.Fatalf("fieldCursor = %d, want 1", p.fieldCursor)
	}

	// Enter edits the API key field.
	p.handleFormKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !p.editing {
		t.Fatal("expected editing after Enter on second field")
	}
}

// TestConfigPaneFormSave verifies the form can be filled completely and saved.
func TestConfigPaneFormSave(t *testing.T) {
	p := NewConfigPane(&config.Config{})
	p.startNew()

	fill := func(text string) {
		p.handleFormKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		for _, r := range text {
			p.handleEditingKey(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		p.handleFormKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	}

	fill("main")
	p.handleFormKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	fill("token123")
	p.handleFormKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	fill("acct123")

	cmd := p.handleFormKey(tea.KeyPressMsg{Code: 's', Text: "s"})
	if cmd == nil {
		t.Fatal("expected save cmd")
	}

	if len(p.cfg.Profiles) != 1 {
		t.Fatalf("profiles = %+v, want exactly one", p.cfg.Profiles)
	}
	prof := p.cfg.Profiles[0]
	if prof.Name != "main" || prof.APIKey != "token123" || prof.AccountID != "acct123" {
		t.Fatalf("profile = %+v, want main/token123/acct123", prof)
	}
}

// TestConfigPaneFormEscCloses verifies Esc closes the form back to the list.
func TestConfigPaneFormEscCloses(t *testing.T) {
	p := NewConfigPane(&config.Config{})
	p.startNew()
	p.handleFormKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	if p.formMode {
		t.Fatal("expected form mode to end after Esc")
	}
}

// TestEditorPaneEditingAcceptsAllCharacters verifies the record editor also
// accepts navigation characters as literal text while editing.
func TestEditorPaneEditingAcceptsAllCharacters(t *testing.T) {
	p := NewEditorPane(nil)
	p.StartNew(zoneFixture())
	p.editing = true

	input := "jack q1n5dpsr"
	for _, r := range input {
		p.handleEditingKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	if got := p.fields[0].value; got != input {
		t.Fatalf("field value = %q, want %q", got, input)
	}
}

// TestEditorPanePaste verifies pasted text is appended to the active field.
func TestEditorPanePaste(t *testing.T) {
	p := NewEditorPane(nil)
	p.StartNew(zoneFixture())
	p.editing = true

	p.Update(tea.PasteMsg{Content: "api.example.com"})

	if got := p.fields[0].value; got != "api.example.com" {
		t.Fatalf("field value = %q, want %q", got, "api.example.com")
	}
}

// TestEditorPanePasteIgnoredWhenNotEditing verifies paste is ignored outside
// a text-entry field.
func TestEditorPanePasteIgnoredWhenNotEditing(t *testing.T) {
	p := NewEditorPane(nil)
	p.StartNew(zoneFixture())

	p.Update(tea.PasteMsg{Content: "ignored"})

	if got := p.fields[0].value; got != "" {
		t.Fatalf("field value = %q, want empty", got)
	}
}

// TestConfigPanePaste verifies pasted text is appended to the active field.
func TestConfigPanePaste(t *testing.T) {
	p := NewConfigPane(&config.Config{})
	p.startNew()
	p.editing = true

	p.Update(tea.PasteMsg{Content: "main-profile"})

	if got := p.fields[0]; got != "main-profile" {
		t.Fatalf("field value = %q, want %q", got, "main-profile")
	}
}

// TestConfigPanePasteIgnoredWhenNotEditing verifies paste is ignored outside
// a text-entry field.
func TestConfigPanePasteIgnoredWhenNotEditing(t *testing.T) {
	p := NewConfigPane(&config.Config{})
	p.startNew()

	p.Update(tea.PasteMsg{Content: "ignored"})

	if got := p.fields[0]; got != "" {
		t.Fatalf("field value = %q, want empty", got)
	}
}

// TestConfigPaneActivateKey verifies 'a' emits an ActivateProfile message for
// the selected profile.
func TestConfigPaneActivateKey(t *testing.T) {
	cfg := &config.Config{
		DefaultProfile: "main",
		Profiles: []config.Profile{
			{Name: "main", APIKey: "k1", AccountID: "a1"},
			{Name: "backup", APIKey: "k2", AccountID: "a2"},
		},
	}
	p := NewConfigPane(cfg)
	p.cursor = 1

	cmd := p.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if cmd == nil {
		t.Fatal("expected cmd from 'a'")
	}
	msg := cmd()
	am, ok := msg.(messages.ActivateProfile)
	if !ok {
		t.Fatalf("expected ActivateProfile, got %T", msg)
	}
	if am.Name != "backup" {
		t.Fatalf("ActivateProfile.Name = %q, want backup", am.Name)
	}
}

// TestConfigPaneDefaultKey verifies 'm' emits a SetDefaultProfile message for
// the selected profile.
func TestConfigPaneDefaultKey(t *testing.T) {
	cfg := &config.Config{
		DefaultProfile: "main",
		Profiles: []config.Profile{
			{Name: "main", APIKey: "k1", AccountID: "a1"},
			{Name: "backup", APIKey: "k2", AccountID: "a2"},
		},
	}
	p := NewConfigPane(cfg)
	p.cursor = 1

	cmd := p.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if cmd == nil {
		t.Fatal("expected cmd from 'm'")
	}
	msg := cmd()
	dm, ok := msg.(messages.SetDefaultProfile)
	if !ok {
		t.Fatalf("expected SetDefaultProfile, got %T", msg)
	}
	if dm.Name != "backup" {
		t.Fatalf("SetDefaultProfile.Name = %q, want backup", dm.Name)
	}
}

// TestConfigPaneViewMarkers verifies the list shows (active) and (default)
// markers.
func TestConfigPaneViewMarkers(t *testing.T) {
	cfg := &config.Config{
		DefaultProfile: "main",
		Profiles: []config.Profile{
			{Name: "main", APIKey: "k1", AccountID: "a1"},
			{Name: "backup", APIKey: "k2", AccountID: "a2"},
		},
	}
	p := NewConfigPane(cfg)
	p.SetActiveProfile("backup")

	view := p.View()
	if !strings.Contains(view, "(active)") {
		t.Fatalf("expected (active) marker in view:\n%s", view)
	}
	if !strings.Contains(view, "(default)") {
		t.Fatalf("expected (default) marker in view:\n%s", view)
	}
}
