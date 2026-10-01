package components

import (
	"strings"

	"charm.land/bubbletea/v2"

	"cloudflui/internal/config"
	"cloudflui/internal/messages"
	"cloudflui/internal/styles"
)

// ConfigPane manages the TOML profiles.
type ConfigPane struct {
	cfg           *config.Config
	profiles      []config.Profile
	cursor        int
	activeProfile string // session-active profile name for display
	formMode      bool   // true while the profile form is open
	editing       bool   // true while a specific field is being edited
	fieldCursor   int
	fields        []string // name, api_key, account_id
	err           string
}

// NewConfigPane creates the config pane.
func NewConfigPane(cfg *config.Config) *ConfigPane {
	p := &ConfigPane{cfg: cfg}
	p.reload()
	return p
}

// IsEditing reports whether the pane is in text-entry mode.
func (p *ConfigPane) IsEditing() bool { return p.editing }

// SetActiveProfile sets the session-active profile name for display.
func (p *ConfigPane) SetActiveProfile(name string) {
	p.activeProfile = name
}

func (p *ConfigPane) reload() {
	p.profiles = append([]config.Profile(nil), p.cfg.Profiles...)
	p.cursor = 0
	p.formMode = false
	p.editing = false
	p.err = ""
}

// Update handles messages for the config pane.
func (p *ConfigPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case messages.ConfigSaved:
		if msg.Err == nil {
			p.reload()
		} else {
			p.err = msg.Err.Error()
		}
	case tea.PasteMsg:
		if p.editing {
			p.fields[p.fieldCursor] += msg.Content
		}
	case tea.KeyPressMsg:
		if p.formMode {
			return p.handleFormKey(msg)
		}
		switch msg.Code {
		case tea.KeyUp, 'k':
			if p.cursor > 0 {
				p.cursor--
			}
		case tea.KeyDown, 'j':
			if p.cursor < len(p.profiles)-1 {
				p.cursor++
			}
		case tea.KeyEnter:
			if len(p.profiles) == 0 {
				return nil
			}
			p.startEdit()
		case 'n':
			p.startNew()
		case 'd':
			if len(p.profiles) == 0 {
				return nil
			}
			p.deleteProfile()
		case 'a':
			if len(p.profiles) == 0 {
				return nil
			}
			name := p.profiles[p.cursor].Name
			return func() tea.Msg { return messages.ActivateProfile{Name: name} }
		case 'm':
			if len(p.profiles) == 0 {
				return nil
			}
			name := p.profiles[p.cursor].Name
			return func() tea.Msg { return messages.SetDefaultProfile{Name: name} }
		case 'r':
			p.reload()
		}
	}
	return nil
}

// handleFormKey handles keys while the profile form is open. When a field is
// being edited, all keys are text input; otherwise j/k navigate fields,
// Enter starts editing, s saves, and Esc closes the form.
func (p *ConfigPane) handleFormKey(msg tea.KeyPressMsg) tea.Cmd {
	if p.editing {
		return p.handleEditingKey(msg)
	}
	switch msg.Code {
	case tea.KeyUp, 'k':
		if p.fieldCursor > 0 {
			p.fieldCursor--
		}
	case tea.KeyDown, 'j':
		if p.fieldCursor < len(p.fields)-1 {
			p.fieldCursor++
		}
	case tea.KeyEnter:
		p.editing = true
	case 's':
		return p.save()
	case tea.KeyEsc:
		p.formMode = false
		p.editing = false
	}
	return nil
}

func (p *ConfigPane) startEdit() {
	prof := p.profiles[p.cursor]
	p.fields = []string{prof.Name, prof.APIKey, prof.AccountID}
	p.fieldCursor = 0
	p.formMode = true
	p.editing = false
	p.err = ""
}

func (p *ConfigPane) startNew() {
	p.fields = []string{"", "", ""}
	p.fieldCursor = 0
	p.formMode = true
	p.editing = false
	p.err = ""
}

func (p *ConfigPane) handleEditingKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Code {
	case tea.KeyEnter:
		p.editing = false
	case tea.KeyEsc:
		p.editing = false
	case tea.KeyBackspace:
		if len(p.fields[p.fieldCursor]) > 0 {
			p.fields[p.fieldCursor] = p.fields[p.fieldCursor][:len(p.fields[p.fieldCursor])-1]
		}
	default:
		if msg.Text != "" {
			p.fields[p.fieldCursor] += msg.Text
		}
	}
	return nil
}

func (p *ConfigPane) deleteProfile() {
	name := p.profiles[p.cursor].Name
	for i := range p.cfg.Profiles {
		if p.cfg.Profiles[i].Name == name {
			p.cfg.Profiles = append(p.cfg.Profiles[:i], p.cfg.Profiles[i+1:]...)
			break
		}
	}
	if p.cfg.DefaultProfile == name {
		p.cfg.DefaultProfile = ""
	}
	p.reload()
}

func (p *ConfigPane) save() tea.Cmd {
	name := p.fields[0]
	if name == "" {
		p.err = "Profile name is required"
		return nil
	}
	apiKey := p.fields[1]
	accountID := p.fields[2]

	// Update existing profile or append a new one.
	found := false
	for i := range p.cfg.Profiles {
		if p.cfg.Profiles[i].Name == name {
			p.cfg.Profiles[i].APIKey = apiKey
			p.cfg.Profiles[i].AccountID = accountID
			found = true
			break
		}
	}
	if !found {
		p.cfg.Profiles = append(p.cfg.Profiles, config.Profile{
			Name:      name,
			APIKey:    apiKey,
			AccountID: accountID,
		})
	}
	if p.cfg.DefaultProfile == "" {
		p.cfg.DefaultProfile = name
	}

	return func() tea.Msg {
		err := p.cfg.Save()
		return messages.ConfigSaved{Err: err}
	}
}

// View renders the config pane.
func (p *ConfigPane) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Profiles") + "\n\n")

	if p.formMode {
		b.WriteString(p.renderEditor())
		return b.String()
	}

	if len(p.profiles) == 0 {
		b.WriteString(styles.Dimmed.Render("No profiles configured. Press n to add one.") + "\n")
	} else {
		for i, prof := range p.profiles {
			line := prof.Name
			if prof.Name == p.activeProfile {
				line += " " + styles.Selected.Render("(active)")
			}
			if prof.Name == p.cfg.DefaultProfile {
				line += " " + styles.ProxiedOn.Render("(default)")
			}
			if i == p.cursor {
				b.WriteString(styles.Selected.Render("> " + line))
			} else {
				b.WriteString(styles.Dimmed.Render("  " + line))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n" + styles.HelpBar.Render("j/k: move  enter: edit  n: new  a: activate  m: default  d: delete  r: reload"))

	if p.err != "" {
		b.WriteString("\n" + styles.ErrorText.Render(p.err))
	}
	return b.String()
}

func (p *ConfigPane) renderEditor() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Edit Profile") + "\n\n")
	labels := []string{"Name", "API Key", "Account ID"}
	for i, label := range labels {
		l := styles.Label.Render(label + ":")
		v := styles.Value.Render(p.fields[i])
		if i == p.fieldCursor {
			if p.editing {
				v = styles.FieldActive.Render(p.fields[i] + "▌")
			} else {
				l = styles.FieldActive.Render("> " + label + ":")
			}
		}
		b.WriteString("  " + l + "  " + v + "\n")
	}
	b.WriteString("\n" + styles.HelpBar.Render("j/k: move  enter: edit field  s: save  esc: back"))
	if p.err != "" {
		b.WriteString("\n" + styles.ErrorText.Render(p.err))
	}
	return b.String()
}
