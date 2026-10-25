package routerosadapter

import (
	"strings"
	"testing"
)

func TestValidateVoucherBatchInput(t *testing.T) {
	valid, err := ValidateVoucherBatchInput(VoucherBatchInput{
		Reference: "VCH-TEST", Prefix: "BT-", Count: 60,
		Profile: "default", Server: "",
	})
	if err != nil {
		t.Fatalf("valid voucher batch rejected: %v", err)
	}
	if valid.Server != "all" {
		t.Fatalf("default server = %q, want all", valid.Server)
	}

	tests := []VoucherBatchInput{
		{Prefix: "OK", Count: 0, Profile: "default"},
		{Prefix: "OK", Count: 201, Profile: "default"},
		{Prefix: "BAD PREFIX", Count: 5, Profile: "default"},
		{Prefix: "OK", Count: 2, Profile: ""},
		{Prefix: "TOO-LONG-PREFIX-THAT-WOULD-EXCEED-USERNAME-LENGTH", Count: 1, Profile: "default"},
	}
	for i, input := range tests {
		if _, err := ValidateVoucherBatchInput(input); err == nil {
			t.Errorf("case %d unexpectedly passed validation", i)
		}
	}
}

func TestRandomVoucherCodeUsesExpectedLengthAndAlphabet(t *testing.T) {
	code, err := randomVoucherCode(12)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 12 {
		t.Fatalf("code length = %d, want 12", len(code))
	}
	for _, r := range code {
		found := false
		for _, allowed := range voucherAlphabet {
			if r == allowed {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("generated unexpected character %q", r)
		}
	}
}

func TestHotSpotVoucherCreateArgsOmitPassword(t *testing.T) {
	args := voucherCreateArgs(VoucherBatchInput{
		Reference: "VCH-TEST", Profile: "default", Server: "all",
	}, "BT-AAAA2222")
	for _, arg := range args {
		if len(arg) >= len("=password=") && arg[:len("=password=")] == "=password=" {
			t.Fatalf("password must not be sent for username-only voucher: %q", arg)
		}
	}
	if len(args) < 3 || args[0] != "=name=BT-AAAA2222" {
		t.Fatalf("unexpected voucher command arguments: %#v", args)
	}
}

func TestHotSpotVoucherCreateArgsIncludeOptionalLimit(t *testing.T) {
	args := voucherCreateArgs(VoucherBatchInput{
		Reference: "VCH-TEST", Profile: "default", Server: "all", LimitUptime: "1d00:00:00",
	}, "BT-AAAA2222")
	found := false
	for _, arg := range args {
		if arg == "=limit-uptime=1d00:00:00" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected time limit to be included: %#v", args)
	}
}

func TestVoucherBatchDefaultsAndCredentialOptions(t *testing.T) {
	defaults, err := ValidateVoucherBatchInput(VoucherBatchInput{Count: 1, Profile: "default"})
	if err != nil {
		t.Fatalf("defaults rejected: %v", err)
	}
	if defaults.UsernameLength != 8 || defaults.UsernameMode != "alphanumeric" || defaults.PasswordMode != "none" {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}

	tests := []VoucherBatchInput{
		{Count: 1, Profile: "default", UsernameLength: 3},
		{Count: 1, Profile: "default", UsernameLength: 33},
		{Count: 1, Profile: "default", UsernameMode: "symbols"},
		{Count: 1, Profile: "default", PasswordMode: "unknown"},
		{Count: 1, Profile: "default", PasswordMode: "random", PasswordLength: 33},
		{Count: 1, Profile: "default", PasswordMode: "random", PasswordCharset: "unicode"},
		{Count: 1, Profile: "default", Prefix: strings.Repeat("A", 29), UsernameLength: 36},
	}
	for i, input := range tests {
		if _, err := ValidateVoucherBatchInput(input); err == nil {
			t.Errorf("case %d unexpectedly passed validation", i)
		}
	}
}

func TestRandomVoucherCodeUsesRequestedCharacterSet(t *testing.T) {
	for _, tc := range []struct {
		mode string
		allowed string
	}{
		{"letters", voucherLetters},
		{"numbers", voucherDigits},
		{"alphanumeric", voucherAlphabet},
	} {
		code, err := randomVoucherCodeForMode(16, tc.mode)
		if err != nil {
			t.Fatalf("%s: %v", tc.mode, err)
		}
		if len(code) != 16 {
			t.Fatalf("%s generated %d chars", tc.mode, len(code))
		}
		for _, ch := range code {
			if !strings.ContainsRune(tc.allowed, ch) {
				t.Errorf("%s generated unexpected character %q", tc.mode, ch)
			}
		}
	}
}

func TestVoucherCreateArgsCanIncludePassword(t *testing.T) {
	args := voucherCreateArgs(VoucherBatchInput{Profile: "default", Server: "all"}, "AB12", "P455")
	found := false
	for _, arg := range args {
		if arg == "=password=P455" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected explicit password to be included: %#v", args)
	}
}
