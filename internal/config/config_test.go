package config

import (
	"path/filepath"
	"testing"
)

func TestProfileLookup(t *testing.T) {
	cfg := &Config{
		DefaultProfile: "main",
		Profiles: []Profile{
			{Name: "main", APIKey: "k1", AccountID: "a1"},
			{Name: "backup", APIKey: "k2", AccountID: "a2"},
		},
	}

	if prof := cfg.Profile("backup"); prof == nil || prof.APIKey != "k2" {
		t.Fatalf("Profile(backup) = %+v, want k2", prof)
	}
	if prof := cfg.Profile("missing"); prof != nil {
		t.Fatalf("Profile(missing) = %+v, want nil", prof)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	SetPath(filepath.Join(t.TempDir(), "config.toml"))
	defer SetPath("")

	cfg := &Config{
		DefaultProfile: "main",
		Profiles: []Profile{
			{Name: "main", APIKey: "k1", AccountID: "a1"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultProfile != "main" {
		t.Fatalf("DefaultProfile = %q, want main", loaded.DefaultProfile)
	}
	if len(loaded.Profiles) != 1 || loaded.Profiles[0].APIKey != "k1" {
		t.Fatalf("profiles = %+v, want one profile with k1", loaded.Profiles)
	}
}
