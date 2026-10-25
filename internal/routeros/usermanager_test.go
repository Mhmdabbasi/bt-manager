package routerosadapter

import (
	"errors"
	"testing"
)

func TestPathsForUserManagerVersion(t *testing.T) {
	tests := []struct {
		name             string
		version          string
		wantMajor        int
		wantProfiles     string
		wantRelationships string
		wantErr          bool
	}{
		{name: "RouterOS v6", version: "6.49.17 (long-term)", wantMajor: 6, wantProfiles: "/tool/user-manager/profile/print", wantRelationships: "/tool/user-manager/profile/profile-limitation/print"},
		{name: "RouterOS v7", version: "7.20.1 (stable)", wantMajor: 7, wantProfiles: "/user-manager/profile/print", wantRelationships: "/user-manager/profile-limitation/print"},
		{name: "RouterOS v8 unknown", version: "8.0beta", wantMajor: 8, wantErr: true},
		{name: "missing version", version: "unknown", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, major, err := pathsForUserManagerVersion(test.version)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected unsupported-version error")
				}
				if test.wantMajor != 0 && major != test.wantMajor {
					t.Fatalf("major version=%d, want %d", major, test.wantMajor)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if major != test.wantMajor {
				t.Fatalf("major version=%d, want %d", major, test.wantMajor)
			}
			if paths.profiles != test.wantProfiles {
				t.Fatalf("profiles path=%q, want %q", paths.profiles, test.wantProfiles)
			}
			if paths.profileLimitations != test.wantRelationships {
				t.Fatalf("relationship path=%q, want %q", paths.profileLimitations, test.wantRelationships)
			}
			if paths.profileProperties == "" || paths.limitationProperties == "" || paths.relationProperties == "" {
				t.Fatal("version path set omitted properties")
			}
		})
	}
}

func TestUserManagerVersionRejectsUnsupportedRelease(t *testing.T) {
	_, _, err := pathsForUserManagerVersion("RouterOS 8.1")
	if !errors.Is(err, ErrUnsupportedUserManagerVersion) {
		t.Fatalf("error=%v, want ErrUnsupportedUserManagerVersion", err)
	}
}

func TestSafePropertiesOnlyReturnsRequestedNonIdentityFields(t *testing.T) {
	row := map[string]string{
		".id": "*1",
		"name": "Starter",
		"validity": "30d",
		"password": "must-not-be-returned",
		"unknown-property": "unexpected",
	}
	got := safeProperties(row, []string{".id", "name", "validity", "password"})
	if _, ok := got[".id"]; ok {
		t.Fatal("internal RouterOS row ID should not be copied into properties")
	}
	if _, ok := got["password"]; ok {
		t.Fatal("credential property should not be returned")
	}
	if got["name"] != "Starter" || got["validity"] != "30d" {
		t.Fatalf("expected allowed properties, got %#v", got)
	}
	if _, ok := got["unknown-property"]; ok {
		t.Fatal("unrequested property was returned")
	}
}
