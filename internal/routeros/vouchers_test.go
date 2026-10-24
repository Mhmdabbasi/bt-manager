package routerosadapter

import (
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
	args := []string{"=name=BT-AAAA2222", "=profile=default", "=server=all"}
	for _, arg := range args {
		if len(arg) >= len("=password=") && arg[:len("=password=")] == "=password=" {
			t.Fatalf("password must not be sent for username-only voucher: %q", arg)
		}
	}
}
