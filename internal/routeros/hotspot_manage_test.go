package routerosadapter

import (
	"errors"
	"strings"
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

func TestNormalizeHotSpotUserRejectsMissingNameOrBadID(t *testing.T) {
	tests := []HotSpotUserInput{
		{Password: "secret"},
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

func TestNormalizeHotSpotUserAllowsBlankPassword(t *testing.T) {
	user, err := normalizeUser(HotSpotUserInput{Name: "  voucher-100  "}, true)
	if err != nil {
		t.Fatalf("blank password should be allowed: %v", err)
	}
	if user.Name != "voucher-100" || user.Password != "" {
		t.Fatalf("unexpected normalized user: %#v", user)
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

func TestHotSpotUserCreateArgsOmitBlankPassword(t *testing.T) {
	args := hotSpotUserCreateArgs(HotSpotUserInput{Name: "voucher-100", Profile: "default", Server: "all"})
	for _, arg := range args {
		if strings.HasPrefix(arg, "=password=") {
			t.Fatalf("blank password should be omitted from RouterOS arguments, got %q", arg)
		}
	}
	if len(args) == 0 || args[0] != "=name=voucher-100" {
		t.Fatalf("expected username argument, got %#v", args)
	}
}

func TestHotSpotUserCreateArgsIncludeProvidedPassword(t *testing.T) {
	args := hotSpotUserCreateArgs(HotSpotUserInput{Name: "voucher-100", Password: "secret"})
	found := false
	for _, arg := range args {
		if arg == "=password=secret" {
			found = true
		}
	}
	if !found {
		t.Fatalf("provided password was not added to RouterOS arguments: %#v", args)
	}
}
