package model

import (
	"charm.land/bubbletea/v2"

	"cloudflui/internal/messages"
)

// Update handles all messages for the root model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncViewports()
		return m, nil

	case tea.KeyPressMsg:
		if !m.activePane().IsEditing() {
			if isQuitKey(msg) {
				return m, tea.Quit
			}
			switch msg.Code {
			case tea.KeyTab:
				m.active = (m.active + 1) % paneCount
				return m, nil
			case '1', '2', '3', '4', '5':
				m.active = int(msg.Code - '1')
				return m, nil
			}
		}
		cmd := m.activePane().Update(msg)
		return m, cmd

	case messages.ZoneSelected:
		m.records.SetZone(msg.Zone)
		m.active = PaneRecords
		return m, m.records.Load()

	case messages.EditRecord:
		m.editor.StartEdit(msg.Zone, msg.Record)
		m.active = PaneEditor
		return m, nil

	case messages.NewRecord:
		m.editor.StartNew(msg.Zone)
		m.active = PaneEditor
		return m, nil

	case messages.Navigate:
		m.active = msg.Pane
		return m, nil

	case messages.Status:
		m.status = msg.Text
		return m, nil

	case messages.RecordSaved:
		if msg.Err != nil {
			m.status = "Record save failed: " + msg.Err.Error()
		} else {
			m.status = "Record saved"
		}
		m.active = PaneRecords
		return m, m.records.Update(msg)

	case messages.RecordDeleted:
		if msg.Err != nil {
			m.status = "Record delete failed: " + msg.Err.Error()
		} else {
			m.status = "Record deleted"
		}
		return m, m.records.Update(msg)

	case messages.ConfigSaved:
		if msg.Err != nil {
			m.status = "Config save failed: " + msg.Err.Error()
			return m, nil
		}
		m.rebuildClient()
		m.rebuildPanes()
		m.status = "Config saved"
		if m.client == nil {
			return m, nil
		}
		return m, tea.Batch(m.zones.Load(), m.workers.Load())

	case messages.ActivateProfile:
		if m.cfg.Profile(msg.Name) == nil {
			m.status = "Unknown profile: " + msg.Name
			return m, nil
		}
		m.activeProfile = msg.Name
		m.rebuildClient()
		m.rebuildPanes()
		m.status = "Active profile: " + msg.Name
		if m.client == nil {
			return m, nil
		}
		return m, tea.Batch(m.zones.Load(), m.workers.Load())

	case messages.SetDefaultProfile:
		if m.cfg.Profile(msg.Name) == nil {
			m.status = "Unknown profile: " + msg.Name
			return m, nil
		}
		m.activeProfile = msg.Name
		m.cfg.DefaultProfile = msg.Name
		if err := m.cfg.Save(); err != nil {
			m.status = "Failed to save default profile: " + err.Error()
			return m, nil
		}
		m.rebuildClient()
		m.rebuildPanes()
		m.status = "Default profile: " + msg.Name
		if m.client == nil {
			return m, nil
		}
		return m, tea.Batch(m.zones.Load(), m.workers.Load())

	default:
		// Broadcast data messages to all panes.
		var cmds []tea.Cmd
		for _, p := range m.allPanes() {
			if cmd := p.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}
}
