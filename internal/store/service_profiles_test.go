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


func TestHotSpotAndUserManagerProfilesShareRouterCatalogWithoutNameCollisions(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "shared-profile-catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	hotspot, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.13", Port: 8728, RouterIdentity: "shared-router", RouterOSVersion: "7.20.1", Model: "CHR",
		Profiles: []ImportedServiceProfile{{SourceID: "*1", Name: "default", Properties: map[string]string{"rate-limit": "2M/2M"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	userManager, err := db.SyncUserManagerSnapshot(ctx, UserManagerSyncInput{
		Address: "192.0.2.13", Port: 8728, RouterIdentity: "shared-router", RouterOSVersion: "7.20.1", Model: "CHR",
		Profiles: []ImportedUserManagerProfile{{SourceID: "*1", Name: "default", Properties: map[string]string{"validity": "30d"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hotspot.RouterID != userManager.RouterID {
		t.Fatalf("same router has two local IDs: HotSpot=%d UserManager=%d", hotspot.RouterID, userManager.RouterID)
	}

	hotspotProfiles, err := db.ListServiceProfiles(ctx, hotspot.RouterID, "hotspot", true)
	if err != nil {
		t.Fatal(err)
	}
	umProfiles, err := db.ListServiceProfiles(ctx, hotspot.RouterID, "usermanager", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(hotspotProfiles) != 1 || len(umProfiles) != 1 || hotspotProfiles[0].ID == umProfiles[0].ID {
		t.Fatalf("source-specific profiles collided: hotspot=%+v um=%+v", hotspotProfiles, umProfiles)
	}
	packageResult, err := db.ExecContext(ctx, `INSERT INTO card_packages
		(name,service_system,service_profile_id,user_price_minor,dealer_price_minor)
		VALUES('HotSpot Starter','hotspot',?,20000,16000)`, hotspotProfiles[0].ID)
	if err != nil {
		t.Fatalf("HotSpot card package could not reference its imported service profile: %v", err)
	}
	if _, err := packageResult.LastInsertId(); err != nil {
		t.Fatal(err)
	}
}
