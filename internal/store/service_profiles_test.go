package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestSyncHotSpotProfilesSnapshotAndSoftMissing(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "hotspot-profiles.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	first, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.12", Port: 8728, RouterIdentity: "test-hotspot", RouterOSVersion: "7.20.1", Model: "CHR",
		Profiles: []ImportedServiceProfile{
			{SourceID: "*1", Name: "default", Properties: map[string]string{"shared-users": "1"}},
			{SourceID: "*2", Name: "Premium", Properties: map[string]string{"rate-limit": "10M/10M"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceSystem != "hotspot" || first.ProfileCount != 2 {
		t.Fatalf("unexpected sync summary: %+v", first)
	}
	profiles, err := db.ListServiceProfiles(ctx, first.RouterID, "hotspot", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || !profiles[0].Present {
		t.Fatalf("unexpected initial profiles: %+v", profiles)
	}

	second, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.12", Port: 8728, RouterIdentity: "test-hotspot", RouterOSVersion: "7.20.1", Model: "CHR",
		Profiles: []ImportedServiceProfile{
			{SourceID: "*1", Name: "default", Properties: map[string]string{"shared-users": "2"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.RouterID != first.RouterID {
		t.Fatalf("router ID changed across syncs: %d -> %d", first.RouterID, second.RouterID)
	}
	all, err := db.ListServiceProfiles(ctx, second.RouterID, "hotspot", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("missing profile was deleted: %+v", all)
	}
	foundDefault, foundPremium := false, false
	for _, profile := range all {
		switch profile.Name {
		case "default":
			foundDefault = profile.Present && profile.Properties["shared-users"] == "2"
		case "Premium":
			foundPremium = !profile.Present
		}
	}
	if !foundDefault || !foundPremium {
		t.Fatalf("current and missing profiles aren't marked as expected: %+v", all)
	}
	active, err := db.ListServiceProfiles(ctx, second.RouterID, "hotspot", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Name != "default" {
		t.Fatalf("includeMissing=false returned %+v", active)
	}
}

func TestListServiceProfilesRejectsInvalidSystemAndRouter(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "hotspot-profiles-errors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ListServiceProfiles(ctx, 1, "invalid", true); !errors.Is(err, ErrInvalidServiceProfileSnapshot) {
		t.Fatalf("system error=%v, want ErrInvalidServiceProfileSnapshot", err)
	}
	if _, err := db.ListServiceProfiles(ctx, 999, "hotspot", true); !errors.Is(err, ErrImportedRouterNotFound) {
		t.Fatalf("router error=%v, want ErrImportedRouterNotFound", err)
	}
}
