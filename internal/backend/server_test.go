package backend

import (
	"path/filepath"
	"testing"

	"cloudflui/internal/config"
)

// TestDispatchBasic exercises the RPC dispatch without network access: ping,
// an unknown method, and the config round-trip through save_profile.
func TestDispatchBasic(t *testing.T) {
	config.SetPath(filepath.Join(t.TempDir(), "config.toml"))
	defer config.SetPath("")

	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(b)

	// ping
	resp := srv.handle([]byte(`{"id":1,"method":"ping"}`))
	if resp.Error != nil {
		t.Fatalf("ping error: %v", resp.Error)
	}

	// unknown method
	resp = srv.handle([]byte(`{"id":2,"method":"bogus"}`))
	if resp.Error == nil || resp.Error.Code != codeMethodNotFound {
		t.Fatalf("bogus = %+v, want method-not-found error", resp)
	}

	// save a profile
	resp = srv.handle([]byte(`{"id":3,"method":"save_profile","params":{"name":"main","api_key":"k","account_id":"a"}}`))
	if resp.Error != nil {
		t.Fatalf("save_profile error: %v", resp.Error)
	}

	// get_config reflects it
	resp = srv.handle([]byte(`{"id":4,"method":"get_config"}`))
	if resp.Error != nil {
		t.Fatalf("get_config error: %v", resp.Error)
	}
	cfg, ok := resp.Result.(Config)
	if !ok {
		t.Fatalf("get_config result = %T, want Config", resp.Result)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "main" {
		t.Fatalf("config = %+v, want one profile named main", cfg)
	}
	if cfg.DefaultProfile != "main" {
		t.Fatalf("DefaultProfile = %q, want main", cfg.DefaultProfile)
	}
}

// TestListZonesWithoutClient verifies operations fail cleanly when no profile
// is configured rather than panicking.
func TestListZonesWithoutClient(t *testing.T) {
	config.SetPath(filepath.Join(t.TempDir(), "config.toml"))
	defer config.SetPath("")

	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(b)

	resp := srv.handle([]byte(`{"id":1,"method":"list_zones"}`))
	if resp.Error == nil {
		t.Fatal("expected error listing zones with no profile")
	}
}
