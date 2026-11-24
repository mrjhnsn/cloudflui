// Package backend exposes cloudflui's data operations over a JSON-RPC
// protocol spoken on stdin/stdout. It replaces the old Bubble Tea UI with a
// headless server that an OpenTUI frontend (written in Rust) drives.
package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudflare/cloudflare-go/v7/dns"

	"cloudflui/internal/api"
	"cloudflui/internal/config"
)

// Zone is a Cloudflare zone as exposed to the frontend.
type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Record is a DNS record as exposed to the frontend.
type Record struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
	Comment string `json:"comment"`
}

// WorkerScript is a Workers script summary.
type WorkerScript struct {
	ID        string `json:"id"`
	CreatedOn string `json:"created_on"`
}

// Deployment is a Workers script deployment.
type Deployment struct {
	CreatedOn string `json:"created_on"`
	Strategy  string `json:"strategy"`
}

// Profile is a config profile.
type Profile struct {
	Name      string `json:"name"`
	APIKey    string `json:"api_key"`
	AccountID string `json:"account_id"`
}

// Config is the config view the frontend renders.
type Config struct {
	DefaultProfile string    `json:"default_profile"`
	ActiveProfile  string    `json:"active_profile"`
	Profiles       []Profile `json:"profiles"`
}

// Backend holds the config and API client and answers RPC calls.
type Backend struct {
	cfg           *config.Config
	client        *api.Client
	activeProfile string // session-only active profile override
}

// New loads the config and builds the client for the active profile.
func New() (*Backend, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	b := &Backend{cfg: cfg}
	b.rebuildClient()
	return b, nil
}

// currentProfile returns the session-active profile, falling back to the
// configured default (or first profile).
func (b *Backend) currentProfile() *config.Profile {
	if b.activeProfile != "" {
		if prof := b.cfg.Profile(b.activeProfile); prof != nil {
			return prof
		}
	}
	return b.cfg.Active()
}

// activeName returns the name of the resolved active profile, or "".
func (b *Backend) activeName() string {
	if prof := b.currentProfile(); prof != nil {
		return prof.Name
	}
	return ""
}

// rebuildClient recreates the API client from the active profile.
func (b *Backend) rebuildClient() {
	prof := b.currentProfile()
	if prof == nil {
		b.client = nil
		return
	}
	client, err := api.New(prof.APIKey)
	if err != nil {
		b.client = nil
		return
	}
	b.client = client
}

// clientOrErr returns the API client or an error explaining why it is absent.
func (b *Backend) clientOrErr() (*api.Client, error) {
	if b.client == nil {
		return nil, errors.New("no API token configured; add a profile in the Config pane")
	}
	return b.client, nil
}

// configView serializes the current config plus the resolved active profile.
func (b *Backend) configView() Config {
	out := Config{
		DefaultProfile: b.cfg.DefaultProfile,
		ActiveProfile:  b.activeName(),
		Profiles:       make([]Profile, 0, len(b.cfg.Profiles)),
	}
	for _, p := range b.cfg.Profiles {
		out.Profiles = append(out.Profiles, Profile{
			Name:      p.Name,
			APIKey:    p.APIKey,
			AccountID: p.AccountID,
		})
	}
	return out
}

// GetConfig returns the config view.
func (b *Backend) GetConfig() Config {
	return b.configView()
}

// ListZones returns all zones for the active profile.
func (b *Backend) ListZones(ctx context.Context) ([]Zone, error) {
	client, err := b.clientOrErr()
	if err != nil {
		return nil, err
	}
	list, err := client.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Zone, 0, len(list))
	for _, z := range list {
		out = append(out, Zone{ID: z.ID, Name: z.Name, Status: string(z.Status)})
	}
	return out, nil
}

// ListRecords returns all DNS records for a zone.
func (b *Backend) ListRecords(ctx context.Context, zoneID string) ([]Record, error) {
	client, err := b.clientOrErr()
	if err != nil {
		return nil, err
	}
	list, err := client.ListRecords(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	return toRecords(list), nil
}

// CreateRecord creates a DNS record.
func (b *Backend) CreateRecord(ctx context.Context, zoneID, name, rtype, content string, ttl int, proxied bool, comment string) error {
	client, err := b.clientOrErr()
	if err != nil {
		return err
	}
	_, err = client.CreateRecord(ctx, zoneID, name, rtype, content, ttl, proxied, comment)
	return err
}

// UpdateRecord updates an existing DNS record.
func (b *Backend) UpdateRecord(ctx context.Context, zoneID, recordID, name, rtype, content string, ttl int, proxied bool, comment string) error {
	client, err := b.clientOrErr()
	if err != nil {
		return err
	}
	_, err = client.UpdateRecord(ctx, zoneID, recordID, name, rtype, content, ttl, proxied, comment)
	return err
}

// DeleteRecord removes a DNS record.
func (b *Backend) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	client, err := b.clientOrErr()
	if err != nil {
		return err
	}
	return client.DeleteRecord(ctx, zoneID, recordID)
}

// ListWorkers returns all Workers scripts for an account.
func (b *Backend) ListWorkers(ctx context.Context, accountID string) ([]WorkerScript, error) {
	client, err := b.clientOrErr()
	if err != nil {
		return nil, err
	}
	if accountID == "" {
		return nil, errors.New("no account ID configured; set it in the Config pane")
	}
	list, err := client.ListWorkers(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkerScript, 0, len(list))
	for _, s := range list {
		out = append(out, WorkerScript{ID: s.ID, CreatedOn: shortTime(s.CreatedOn)})
	}
	return out, nil
}

// ListDeployments returns the deployment history for a Workers script.
func (b *Backend) ListDeployments(ctx context.Context, accountID, script string) ([]Deployment, error) {
	client, err := b.clientOrErr()
	if err != nil {
		return nil, err
	}
	if accountID == "" {
		return nil, errors.New("no account ID configured; set it in the Config pane")
	}
	list, err := client.ListDeployments(ctx, accountID, script)
	if err != nil {
		return nil, err
	}
	out := make([]Deployment, 0, len(list))
	for _, d := range list {
		out = append(out, Deployment{CreatedOn: shortTime(d.CreatedOn), Strategy: string(d.Strategy)})
	}
	return out, nil
}

// SaveProfile upserts a profile and persists the config.
func (b *Backend) SaveProfile(name, apiKey, accountID string) (Config, error) {
	if name == "" {
		return b.configView(), errors.New("profile name is required")
	}
	found := false
	for i := range b.cfg.Profiles {
		if b.cfg.Profiles[i].Name == name {
			b.cfg.Profiles[i].APIKey = apiKey
			b.cfg.Profiles[i].AccountID = accountID
			found = true
			break
		}
	}
	if !found {
		b.cfg.Profiles = append(b.cfg.Profiles, config.Profile{
			Name:      name,
			APIKey:    apiKey,
			AccountID: accountID,
		})
	}
	if b.cfg.DefaultProfile == "" {
		b.cfg.DefaultProfile = name
	}
	if err := b.cfg.Save(); err != nil {
		return b.configView(), err
	}
	b.rebuildClient()
	return b.configView(), nil
}

// DeleteProfile removes a profile and persists the config.
func (b *Backend) DeleteProfile(name string) (Config, error) {
	for i := range b.cfg.Profiles {
		if b.cfg.Profiles[i].Name == name {
			b.cfg.Profiles = append(b.cfg.Profiles[:i], b.cfg.Profiles[i+1:]...)
			break
		}
	}
	if b.cfg.DefaultProfile == name {
		b.cfg.DefaultProfile = ""
	}
	if b.activeProfile == name {
		b.activeProfile = ""
	}
	if err := b.cfg.Save(); err != nil {
		return b.configView(), err
	}
	b.rebuildClient()
	return b.configView(), nil
}

// ActivateProfile switches the session-active profile.
func (b *Backend) ActivateProfile(name string) (Config, error) {
	if b.cfg.Profile(name) == nil {
		return b.configView(), fmt.Errorf("unknown profile: %s", name)
	}
	b.activeProfile = name
	b.rebuildClient()
	return b.configView(), nil
}

// SetDefaultProfile persists a profile as the default and activates it.
func (b *Backend) SetDefaultProfile(name string) (Config, error) {
	if b.cfg.Profile(name) == nil {
		return b.configView(), fmt.Errorf("unknown profile: %s", name)
	}
	b.activeProfile = name
	b.cfg.DefaultProfile = name
	if err := b.cfg.Save(); err != nil {
		return b.configView(), err
	}
	b.rebuildClient()
	return b.configView(), nil
}

// toRecords maps SDK record responses to the JSON DTO.
func toRecords(list []dns.RecordResponse) []Record {
	out := make([]Record, 0, len(list))
	for _, r := range list {
		out = append(out, Record{
			ID:      r.ID,
			Type:    string(r.Type),
			Name:    r.Name,
			Content: r.Content,
			TTL:     int(r.TTL),
			Proxied: r.Proxied,
			Comment: r.Comment,
		})
	}
	return out
}

// shortTime formats a timestamp as a date, matching the old TUI's list rows.
func shortTime(ts time.Time) string {
	return ts.Format("2006-01-02")
}
