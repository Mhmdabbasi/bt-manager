package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestSyncUserManagerSnapshotPersistsProfilesAndMultipleLimitations(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "usermanager.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	input := UserManagerSyncInput{
		Address: "192.0.2.10", Port: 8728, RouterIdentity: "test-router",
		RouterOSVersion: "7.20.1", Model: "CHR",
		Profiles: []ImportedUserManagerProfile{
			{SourceID: "*1", Name: "Starter", Properties: map[string]string{"validity": "30d", "price": "200"}},
			{SourceID: "*2", Name: "Monthly", Properties: map[string]string{"validity": "30d"}},
		},
		Limitations: []ImportedUserManagerLimitation{
			{SourceID: "*11", Name: "Daily", Properties: map[string]string{"transfer-limit": "1G"}},
			{SourceID: "*12", Name: "MonthlyCap", Properties: map[string]string{"transfer-limit": "20G"}},
		},
		ProfileLimitations: []ImportedUserManagerProfileLimitation{
			{ProfileSourceID: "Starter", ProfileName: "Starter", LimitationSourceID: "Daily", LimitationName: "Daily", Position: 0, Properties: map[string]string{"weekdays": "sunday,monday"}},
			{ProfileSourceID: "Starter", ProfileName: "Starter", LimitationSourceID: "MonthlyCap", LimitationName: "MonthlyCap", Position: 1, Properties: map[string]string{"from-time": "0s"}},
			{ProfileSourceID: "Monthly", ProfileName: "Monthly", LimitationSourceID: "MonthlyCap", LimitationName: "MonthlyCap", Position: 0, Properties: map[string]string{}},
		},
	}
	first, err := db.SyncUserManagerSnapshot(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ProfileCount != 2 || first.LimitationCount != 2 || first.RelationshipCount != 3 {
		t.Fatalf("unexpected sync counts: %+v", first)
	}
	allLimitations, err := db.ListUserManagerLimitations(ctx, first.RouterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(allLimitations) != 2 || !allLimitations[0].Present || !allLimitations[1].Present {
		t.Fatalf("unexpected standalone limitation inventory: %+v", allLimitations)
	}

	profiles, err := db.ListUserManagerProfiles(ctx, first.RouterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(profiles))
	}
	if profiles[0].Name != "Monthly" || !profiles[0].Present {
		t.Fatalf("unexpected sorted profile: %+v", profiles[0])
	}
	var starter *UserManagerProfileRecord
	for i := range profiles {
		if profiles[i].Name == "Starter" {
			starter = &profiles[i]
			break
		}
	}
	if starter == nil || len(starter.Limitations) != 2 {
		t.Fatalf("Starter should have two limitations: %+v", starter)
	}
	if starter.Limitations[0].Limitation.Name != "Daily" || starter.Limitations[1].Limitation.Name != "MonthlyCap" {
		t.Fatalf("limitation relationship order was not preserved: %+v", starter.Limitations)
	}
	if starter.Properties["validity"] != "30d" || starter.Limitations[0].Limitation.Properties["transfer-limit"] != "1G" {
		t.Fatalf("imported profile/limitation properties missing: %+v", starter)
	}

	// A second successful sync preserves local IDs, updates properties, and soft-marks records
	// that disappeared from the RouterOS snapshot instead of deleting their history.
	input.Profiles = []ImportedUserManagerProfile{
		{SourceID: "*1", Name: "Starter", Properties: map[string]string{"validity": "14d"}},
	}
	input.Limitations = []ImportedUserManagerLimitation{
		{SourceID: "*11", Name: "Daily", Properties: map[string]string{"transfer-limit": "2G"}},
	}
	input.ProfileLimitations = []ImportedUserManagerProfileLimitation{
		{ProfileSourceID: "Starter", ProfileName: "Starter", LimitationSourceID: "Daily", LimitationName: "Daily", Position: 0},
	}
	second, err := db.SyncUserManagerSnapshot(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if second.RouterID != first.RouterID {
		t.Fatalf("router identity changed across syncs: %d -> %d", first.RouterID, second.RouterID)
	}
	profiles, err = db.ListUserManagerProfiles(ctx, second.RouterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("soft-missing profile was deleted; found %d, want 2", len(profiles))
	}
	presentCount := 0
	for _, profile := range profiles {
		if profile.Present {
			presentCount++
			if profile.Name != "Starter" || profile.Properties["validity"] != "14d" {
				t.Fatalf("present profile wasn't updated: %+v", profile)
			}
			if len(profile.Limitations) != 1 || profile.Limitations[0].Limitation.Properties["transfer-limit"] != "2G" {
				t.Fatalf("current relationship/limitation not updated: %+v", profile)
			}
		} else if profile.Name != "Monthly" {
			t.Fatalf("unexpected soft-missing profile: %+v", profile)
		} else if len(profile.Limitations) != 1 || profile.Limitations[0].Limitation.Present {
			t.Fatalf("last-known relationship should remain visible while the profile/limitation is absent: %+v", profile)
		}
	}
	limitations, err := db.ListUserManagerLimitations(ctx, second.RouterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(limitations) != 2 || limitations[0].Name != "Daily" || !limitations[0].Present || limitations[1].Name != "MonthlyCap" || limitations[1].Present {
		t.Fatalf("missing limitation should be soft-marked, not deleted: %+v", limitations)
	}
	if presentCount != 1 {
		t.Fatalf("present profile count=%d, want 1", presentCount)
	}
}

func TestSyncUserManagerSnapshotRollsBackInvalidRelationships(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "usermanager-invalid.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	valid := UserManagerSyncInput{
		Address: "192.0.2.11", Port: 8728, RouterOSVersion: "7.20.1",
		Profiles: []ImportedUserManagerProfile{{SourceID: "*1", Name: "Starter"}},
		Limitations: []ImportedUserManagerLimitation{{SourceID: "*2", Name: "Daily"}},
		ProfileLimitations: []ImportedUserManagerProfileLimitation{
			{ProfileName: "Starter", LimitationName: "Daily", Position: 0},
		},
	}
	original, err := db.SyncUserManagerSnapshot(ctx, valid)
	if err != nil {
		t.Fatal(err)
	}
	bad := valid
	bad.Profiles = []ImportedUserManagerProfile{{SourceID: "*1", Name: "Starter-new"}}
	bad.ProfileLimitations = []ImportedUserManagerProfileLimitation{
		{ProfileName: "Missing", LimitationName: "Daily", Position: 0},
	}
	if _, err := db.SyncUserManagerSnapshot(ctx, bad); !errors.Is(err, ErrInvalidUserManagerSnapshot) {
		t.Fatalf("error=%v, want invalid-snapshot error", err)
	}

	profiles, err := db.ListUserManagerProfiles(ctx, original.RouterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Name != "Starter" || !profiles[0].Present {
		t.Fatalf("failed sync partially changed saved state: %+v", profiles)
	}
	if len(profiles[0].Limitations) != 1 || profiles[0].Limitations[0].Limitation.Name != "Daily" {
		t.Fatalf("failed sync changed relationships: %+v", profiles[0])
	}
}

func TestListUserManagerProfilesRejectsUnknownRouter(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "usermanager-missing.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ListUserManagerProfiles(context.Background(), 999); !errors.Is(err, ErrImportedRouterNotFound) {
		t.Fatalf("error=%v, want ErrImportedRouterNotFound", err)
	}
}
