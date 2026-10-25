package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestCardPackageCRUDAndFiltering(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "card-packages.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	syncResult, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.21", Port: 8728, RouterIdentity: "package-router", RouterOSVersion: "7.20.1",
		Profiles: []ImportedServiceProfile{{SourceID: "*1", Name: "Premium", Properties: map[string]string{"rate-limit": "8M/8M"}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := db.CreateCardPackage(ctx, CardPackageInput{
		Name: "Premium 200", ServiceSystem: "hotspot", ServiceProfileID: 1,
		Description: "Ten-hour card", UserPriceMinor: 20000, DealerPriceMinor: 15000,
		DefaultPrefix: "P200-", Active: true, SortOrder: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID < 1 || created.RouterID != syncResult.RouterID || created.ServiceProfileName != "Premium" ||
		created.ServiceSystem != "hotspot" || !created.ServiceProfilePresent ||
		created.UserPriceMinor != 20000 || created.DealerPriceMinor != 15000 ||
		created.DefaultPrefix != "P200-" || !created.Active {
		t.Fatalf("unexpected created package: %+v", created)
	}

	active, err := db.ListCardPackages(ctx, false, "hotspot")
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != created.ID {
		t.Fatalf("active package list=%+v", active)
	}
	if _, err := db.ListCardPackages(ctx, true, "unknown"); !errors.Is(err, ErrInvalidCardPackage) {
		t.Fatalf("invalid system error=%v", err)
	}

	updated, err := db.UpdateCardPackage(ctx, created.ID, CardPackageInput{
		Name: "Premium 200", ServiceSystem: "hotspot", ServiceProfileID: created.ServiceProfileID,
		Description: "Updated description", UserPriceMinor: 25000, DealerPriceMinor: 18000,
		DefaultPrefix: "P250", Active: false, SortOrder: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Active || updated.UserPriceMinor != 25000 || updated.Description != "Updated description" || updated.SortOrder != 2 {
		t.Fatalf("unexpected updated package: %+v", updated)
	}
	active, err = db.ListCardPackages(ctx, false, "")
	if err != nil || len(active) != 0 {
		t.Fatalf("inactive package appeared in active list: packages=%+v err=%v", active, err)
	}
	all, err := db.ListCardPackages(ctx, true, "")
	if err != nil || len(all) != 1 || all[0].ID != created.ID {
		t.Fatalf("all package list=%+v err=%v", all, err)
	}
}

func TestCardPackageRejectsWrongSystemUnknownProfileAndInvalidAmounts(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "card-packages-validation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	syncResult, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.22", Port: 8728, RouterIdentity: "package-router", RouterOSVersion: "7.20.1",
		Profiles: []ImportedServiceProfile{{SourceID: "*1", Name: "default"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	base := CardPackageInput{Name: "Starter", ServiceSystem: "hotspot", ServiceProfileID: 1, UserPriceMinor: 10000, DealerPriceMinor: 0, Active: true}
	wrongSystem := base
	wrongSystem.ServiceSystem = "usermanager"
	if _, err := db.CreateCardPackage(ctx, wrongSystem); !errors.Is(err, ErrInvalidCardPackage) {
		t.Fatalf("wrong system error=%v", err)
	}
	unknown := base
	unknown.ServiceProfileID = 999
	if _, err := db.CreateCardPackage(ctx, unknown); !errors.Is(err, ErrCardPackageServiceProfileNotFound) {
		t.Fatalf("unknown profile error=%v", err)
	}
	invalidAmounts := base
	invalidAmounts.UserPriceMinor = 0
	if _, err := db.CreateCardPackage(ctx, invalidAmounts); !errors.Is(err, ErrInvalidCardPackage) {
		t.Fatalf("invalid amount error=%v", err)
	}
	invalidDealer := base
	invalidDealer.DealerPriceMinor = 11000
	if _, err := db.CreateCardPackage(ctx, invalidDealer); !errors.Is(err, ErrInvalidCardPackage) {
		t.Fatalf("invalid dealer amount error=%v", err)
	}
	if syncResult.RouterID < 1 {
		t.Fatal("profile router was not created")
	}
}

func TestCardPackageNameIsUniqueIgnoringCase(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "card-packages-unique.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	_, err = db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.23", Port: 8728, RouterIdentity: "package-router", RouterOSVersion: "7.20.1",
		Profiles: []ImportedServiceProfile{{SourceID: "*1", Name: "default"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := CardPackageInput{Name: "Gold", ServiceSystem: "hotspot", ServiceProfileID: 1, UserPriceMinor: 10000, Active: true}
	if _, err := db.CreateCardPackage(ctx, input); err != nil {
		t.Fatal(err)
	}
	input.Name = "gold"
	if _, err := db.CreateCardPackage(ctx, input); !errors.Is(err, ErrCardPackageNameExists) {
		t.Fatalf("duplicate name error=%v", err)
	}
}

func TestCardPackageWarnsWhenProfileDisappearsAndCannotBeReactivated(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "card-packages-missing-profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	first, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.24", Port: 8728, RouterIdentity: "package-router", RouterOSVersion: "7.20.1",
		Profiles: []ImportedServiceProfile{{SourceID: "*1", Name: "default"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := db.CreateCardPackage(ctx, CardPackageInput{
		Name: "Default", ServiceSystem: "hotspot", ServiceProfileID: 1,
		UserPriceMinor: 10000, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.24", Port: 8728, RouterIdentity: "package-router", RouterOSVersion: "7.20.1",
		Profiles: []ImportedServiceProfile{},
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := db.ListCardPackages(ctx, true, "")
	if err != nil || len(all) != 1 || all[0].ServiceProfilePresent {
		t.Fatalf("missing-profile state not surfaced: packages=%+v err=%v", all, err)
	}
	if _, err := db.UpdateCardPackage(ctx, pkg.ID, CardPackageInput{
		Name: "Default", ServiceSystem: "hotspot", ServiceProfileID: pkg.ServiceProfileID,
		UserPriceMinor: 10000, Active: true,
	}); !errors.Is(err, ErrCardPackageServiceProfileUnavailable) {
		t.Fatalf("reactivation error=%v", err)
	}
	_, err = db.UpdateCardPackage(ctx, pkg.ID, CardPackageInput{
		Name: "Default", ServiceSystem: "hotspot", ServiceProfileID: pkg.ServiceProfileID,
		UserPriceMinor: 10000, Active: false,
	})
	if err != nil {
		t.Fatalf("deactivating a package with a missing profile should remain possible: %v", err)
	}
	if first.RouterID != pkg.RouterID {
		t.Fatal("package router relationship changed unexpectedly")
	}
}

func TestArchiveCardPackagePreservesRecord(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "card-packages-archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	_, err = db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.25", Port: 8728, RouterIdentity: "package-router", RouterOSVersion: "7.20.1",
		Profiles: []ImportedServiceProfile{{SourceID: "*1", Name: "default"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := db.CreateCardPackage(ctx, CardPackageInput{
		Name: "Archive test", ServiceSystem: "hotspot", ServiceProfileID: 1,
		UserPriceMinor: 10000, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	archived, err := db.ArchiveCardPackage(ctx, pkg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Active {
		t.Fatalf("archived package remains active: %+v", archived)
	}
	list, err := db.ListCardPackages(ctx, true, "")
	if err != nil || len(list) != 1 || list[0].ID != pkg.ID || list[0].Active {
		t.Fatalf("archive removed package history: list=%+v err=%v", list, err)
	}
	if _, err := db.ArchiveCardPackage(ctx, 999); !errors.Is(err, ErrCardPackageNotFound) {
		t.Fatalf("missing package archive error=%v", err)
	}
}
