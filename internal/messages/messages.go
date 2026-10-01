// Package messages defines the Bubble Tea messages exchanged between the
// root model and the panes.
package messages

import (
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/workers"
	"github.com/cloudflare/cloudflare-go/v7/zones"
)

// ZonesLoaded reports the result of a zone list operation.
type ZonesLoaded struct {
	Zones []zones.Zone
	Err   error
}

// RecordsLoaded reports the result of a record list operation.
type RecordsLoaded struct {
	Records []dns.RecordResponse
	Err     error
}

// RecordSaved reports the result of a create or update operation.
type RecordSaved struct {
	Err error
}

// RecordDeleted reports the result of a delete operation.
type RecordDeleted struct {
	Err error
}

// WorkersLoaded reports the result of a Workers script list operation.
type WorkersLoaded struct {
	Scripts []workers.ScriptListResponse
	Err     error
}

// DeploymentsLoaded reports the result of a deployment list operation.
type DeploymentsLoaded struct {
	ScriptName  string
	Deployments []workers.Deployment
	Err         error
}

// ZoneSelected is emitted when the user picks a zone in the zones pane.
type ZoneSelected struct {
	Zone zones.Zone
}

// EditRecord is emitted when the user picks a record to edit.
type EditRecord struct {
	Zone   zones.Zone
	Record dns.RecordResponse
}

// NewRecord is emitted when the user wants to create a record.
type NewRecord struct {
	Zone zones.Zone
}

// Navigate requests a pane switch. Pane indices: 0 zones, 1 records,
// 2 editor, 3 config, 4 workers.
type Navigate struct {
	Pane int
}

// Status shows a transient status message in the status bar.
type Status struct {
	Text string
}

// ConfigSaved reports the result of saving the config file.
type ConfigSaved struct {
	Err error
}

// ActivateProfile switches the active profile for the current session.
type ActivateProfile struct {
	Name string
}

// SetDefaultProfile persists a profile as the default.
type SetDefaultProfile struct {
	Name string
}
