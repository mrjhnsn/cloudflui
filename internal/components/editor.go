package components

import (
	"context"
	"strconv"
	"strings"

	"charm.land/bubbletea/v2"
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/zones"

	"cloudflui/internal/api"
	"cloudflui/internal/messages"
	"cloudflui/internal/styles"
)

// editorField is a single editable form field.
type editorField struct {
	label string
	value string
}

// EditorPane is the DNS record create/edit form.
type EditorPane struct {
	client  *api.Client
	zone    *zones.Zone
	record  *dns.RecordResponse // nil when creating
	fields  []editorField
	cursor  int
	proxied bool
	editing bool
	err     string
}

// NewEditorPane creates the editor pane.
func NewEditorPane(client *api.Client) *EditorPane {
	return &EditorPane{client: client}
}

// IsEditing reports whether the pane is in text-entry mode.
func (p *EditorPane) IsEditing() bool { return p.editing }

// StartEdit prepares the form for editing an existing record.
func (p *EditorPane) StartEdit(zone zones.Zone, rec dns.RecordResponse) {
	p.zone = &zone
	p.record = &rec
	p.fields = []editorField{
		{label: "Name", value: rec.Name},
		{label: "Type", value: string(rec.Type)},
		{label: "Content", value: rec.Content},
		{label: "TTL", value: strconv.Itoa(int(rec.TTL))},
		{label: "Comment", value: rec.Comment},
	}
	p.proxied = rec.Proxied
	p.cursor = 0
	p.editing = false
	p.err = ""
}

// StartNew prepares the form for creating a record.
func (p *EditorPane) StartNew(zone zones.Zone) {
	p.zone = &zone
	p.record = nil
	p.fields = []editorField{
		{label: "Name", value: ""},
		{label: "Type", value: "A"},
		{label: "Content", value: ""},
		{label: "TTL", value: "1"},
		{label: "Comment", value: ""},
	}
	p.proxied = true
	p.cursor = 0
	p.editing = false
	p.err = ""
}

// Update handles messages for the editor pane.
func (p *EditorPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if p.editing {
			p.fields[p.cursor].value += msg.Content
		}
	case tea.KeyPressMsg:
		if p.editing {
			return p.handleEditingKey(msg)
		}
		switch msg.Code {
		case tea.KeyUp, 'k':
			if p.cursor > 0 {
				p.cursor--
			}
		case tea.KeyDown, 'j':
			if p.cursor < len(p.fields) {
				p.cursor++
			}
		case tea.KeyEnter:
			if p.cursor == len(p.fields) {
				p.proxied = !p.proxied
			} else {
				p.editing = true
			}
		case ' ':
			p.proxied = !p.proxied
		case 's':
			return p.save()
		case tea.KeyEsc:
			return func() tea.Msg { return messages.Navigate{Pane: 1} }
		}
	}
	return nil
}

func (p *EditorPane) handleEditingKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Code {
	case tea.KeyEnter:
		p.editing = false
	case tea.KeyEsc:
		p.editing = false
	case tea.KeyBackspace:
		f := &p.fields[p.cursor]
		if len(f.value) > 0 {
			f.value = f.value[:len(f.value)-1]
		}
	default:
		if msg.Text != "" {
			p.fields[p.cursor].value += msg.Text
		}
	}
	return nil
}

func (p *EditorPane) save() tea.Cmd {
	if p.zone == nil {
		return nil
	}
	name := p.fields[0].value
	rtype := p.fields[1].value
	content := p.fields[2].value
	ttlStr := p.fields[3].value
	comment := p.fields[4].value

	ttl, err := strconv.Atoi(ttlStr)
	if err != nil {
		p.err = "TTL must be a number"
		return nil
	}
	if name == "" || content == "" {
		p.err = "Name and Content are required"
		return nil
	}

	zoneID := p.zone.ID
	proxied := p.proxied
	if p.record == nil {
		return func() tea.Msg {
			_, err := p.client.CreateRecord(context.Background(), zoneID, name, rtype, content, ttl, proxied, comment)
			return messages.RecordSaved{Err: err}
		}
	}
	recordID := p.record.ID
	return func() tea.Msg {
		_, err := p.client.UpdateRecord(context.Background(), zoneID, recordID, name, rtype, content, ttl, proxied, comment)
		return messages.RecordSaved{Err: err}
	}
}

// View renders the editor form.
func (p *EditorPane) View() string {
	var b strings.Builder
	if p.zone == nil {
		return styles.Dimmed.Render("Select a record to edit or press n to create one.")
	}
	title := "New DNS Record"
	if p.record != nil {
		title = "Edit DNS Record"
	}
	b.WriteString(styles.Title.Render(title+" — "+p.zone.Name) + "\n\n")

	for i, f := range p.fields {
		label := styles.Label.Render(f.label + ":")
		value := styles.Value.Render(f.value)
		if i == p.cursor {
			if p.editing {
				value = styles.FieldActive.Render(f.value + "▌")
			} else {
				label = styles.FieldActive.Render("> " + f.label + ":")
			}
		}
		b.WriteString("  " + label + "  " + value + "\n")
	}

	proxyLabel := styles.Label.Render("Proxied:")
	proxyValue := styles.ProxiedOff.Render("off")
	if p.proxied {
		proxyValue = styles.ProxiedOn.Render("on")
	}
	if p.cursor == len(p.fields) {
		proxyLabel = styles.FieldActive.Render("> Proxied:")
	}
	b.WriteString("  " + proxyLabel + "  " + proxyValue + "\n")

	if p.err != "" {
		b.WriteString("\n" + styles.ErrorText.Render(p.err))
	}
	return b.String()
}
