package routerosadapter

import (
	"errors"
	"testing"
)

func TestNormalizeHotSpotUserDefaults(t *testing.T) {
	user, err := normalizeUser(HotSpotUserInput{
		Name: " user-1 ", Password: "p@ssword",
	}, true)
	if err != nil {
		t.Fatalf("normalizeUser() error = %v", err)
	}
	if user.Name != "user-1" || user.Profile != "default" || user.Server != "all" {
		t.Fatalf("unexpected normalized user: %#v", user)
	}
}

func TestNormalizeHotSpotUserRejectsMissingCredentialsOrBadID(t *testing.T) {
	tests := []HotSpotUserInput{
		{Name: "user-1"},
		{ID: "not-an-id", Name: "user-1"},
		{ID: "*2", Name: "user-1", Password: string(make([]byte, 130))},
	}
	for i, input := range tests {
		creating := i == 0
		if _, err := normalizeUser(input, creating); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("case %d error = %v, want ErrInvalidRequest", i, err)
		}
	}
}

func TestNormalizeHotSpotProfileValidatesIDAndSharedUsers(t *testing.T) {
	if _, err := normalizeProfile(HotSpotProfileInput{Name: "default", SharedUsers: "0"}, true); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("zero shared users error = %v, want ErrInvalidRequest", err)
	}
	if _, err := normalizeProfile(HotSpotProfileInput{ID: "bad", Name: "office"}, false); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid ID error = %v, want ErrInvalidRequest", err)
	}
	profile, err := normalizeProfile(HotSpotProfileInput{Name: "office", SharedUsers: "2", RateLimit: "4M/2M"}, true)
	if err != nil {
		t.Fatalf("valid profile error = %v", err)
	}
	if profile.Name != "office" || profile.SharedUsers != "2" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}

func TestValidRouterOSID(t *testing.T) {
	for _, id := range []string{"*1", "*AB12", "*0"} {
		if !validRouterOSID(id) {
			t.Errorf("validRouterOSID(%q) = false", id)
		}
	}
	for _, id := range []string{"", "1", "*", "*1; /system/reboot", " *1 "} {
		if validRouterOSID(id) {
			t.Errorf("validRouterOSID(%q) = true", id)
		}
	}
}
