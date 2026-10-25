package routerosadapter

import (
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

const (
	MaxVoucherBatchSize = 200
	voucherCodeLength   = 8
	voucherAlphabet     = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	voucherLetters      = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	voucherDigits       = "0123456789"
)

var voucherPrefixPattern = regexp.MustCompile("^[A-Za-z0-9_-]{0,32}$")

type VoucherBatchInput struct {
	Reference      string `json:"reference"`
	Prefix         string `json:"prefix"`
	Count          int    `json:"count"`
	Profile        string `json:"profile"`
	Server         string `json:"server"`
	LimitUptime    string `json:"limitUptime"`
	UsernameLength int    `json:"usernameLength"`
	UsernameMode   string `json:"usernameMode"`
	PasswordMode   string `json:"passwordMode"`
	PasswordLength int    `json:"passwordLength"`
	PasswordCharset string `json:"passwordCharset"`
}

type IssuedVoucher struct {
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	ProfileName string `json:"profileName"`
	ServerName  string `json:"serverName"`
	LimitUptime string `json:"limitUptime"`
}

type VoucherBatchResult struct {
	Created      []IssuedVoucher `json:"created"`
	FailedCount  int             `json:"failedCount"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
}

func ValidateVoucherBatchInput(input VoucherBatchInput) (VoucherBatchInput, error) {
	input.Reference = strings.TrimSpace(input.Reference)
	input.Prefix = strings.TrimSpace(input.Prefix)
	input.Profile = strings.TrimSpace(input.Profile)
	input.Server = strings.TrimSpace(input.Server)
	input.LimitUptime = strings.TrimSpace(input.LimitUptime)
	if input.UsernameLength == 0 {
		input.UsernameLength = voucherCodeLength
	}
	if input.UsernameMode == "" {
		input.UsernameMode = "alphanumeric"
	}
	if input.PasswordMode == "" {
		input.PasswordMode = "none"
	}
	if input.PasswordLength == 0 {
		input.PasswordLength = 4
	}
	if input.PasswordCharset == "" {
		input.PasswordCharset = "alphanumeric"
	}
	if input.Count < 1 || input.Count > MaxVoucherBatchSize ||
		input.Profile == "" || !voucherPrefixPattern.MatchString(input.Prefix) ||
		!validText(input.Profile, 64) || !validText(input.Server, 64) ||
		!validText(input.LimitUptime, 64) ||
		input.UsernameLength < 4 || input.UsernameLength > 32 ||
		!validVoucherCharset(input.UsernameMode) ||
		!validPasswordMode(input.PasswordMode) ||
		!validVoucherCharset(input.PasswordCharset) ||
		(input.PasswordMode == "random" && (input.PasswordLength < 4 || input.PasswordLength > 32)) {
		return VoucherBatchInput{}, ErrInvalidRequest
	}
	if input.Server == "" {
		input.Server = "all"
	}
	if len(input.Prefix)+input.UsernameLength > 64 {
		return VoucherBatchInput{}, ErrInvalidRequest
	}
	return input, nil
}

func validVoucherCharset(mode string) bool {
	switch mode {
	case "alphanumeric", "letters", "numbers":
		return true
	default:
		return false
	}
}

func validPasswordMode(mode string) bool {
	switch mode {
	case "none", "same", "random":
		return true
	default:
		return false
	}
}

// GenerateHotSpotVouchers creates HotSpot accounts using the selected credential policy.
// It omits the password attribute only when PasswordMode is none.
func GenerateHotSpotVouchers(
	ctx context.Context,
	request TestRequest,
	input VoucherBatchInput,
	onCreated func(IssuedVoucher) error,
) (VoucherBatchResult, error) {
	input, err := ValidateVoucherBatchInput(input)
	if err != nil {
		return VoucherBatchResult{}, err
	}
	api, err := connectRouter(ctx, request)
	if err != nil {
		return VoucherBatchResult{}, err
	}
	defer api.Close()

	result := VoucherBatchResult{Created: make([]IssuedVoucher, 0, input.Count)}
	for i := 0; i < input.Count; i++ {
		code, err := randomVoucherCodeForMode(input.UsernameLength, input.UsernameMode)
		if err != nil {
			result.FailedCount = input.Count - len(result.Created)
			result.ErrorMessage = "تعذر توليد رمز آمن للقسيمة."
			return result, err
		}
		voucher := IssuedVoucher{
			Username: input.Prefix + code, ProfileName: input.Profile,
			ServerName: input.Server, LimitUptime: input.LimitUptime,
		}
		switch input.PasswordMode {
		case "same":
			voucher.Password = voucher.Username
		case "random":
			voucher.Password, err = randomVoucherCodeForMode(input.PasswordLength, input.PasswordCharset)
			if err != nil {
				result.FailedCount = input.Count - len(result.Created)
				result.ErrorMessage = "تعذر توليد كلمة مرور آمنة للقسيمة."
				return result, err
			}
		}
		args := voucherCreateArgs(input, voucher.Username, voucher.Password)
		if _, err := api.SendContext(ctx, "/ip/hotspot/user/add", args...); err != nil {
			result.FailedCount = input.Count - len(result.Created)
			result.ErrorMessage = fmt.Sprintf("توقف التوليد بعد إنشاء %d من %d قسيمة: %s", len(result.Created), input.Count, safeRouterError(err))
			return result, err
		}

		// Include the username immediately after router-side creation, so partial
		// results remain visible even if writing the local inventory then fails.
		result.Created = append(result.Created, voucher)
		if onCreated != nil {
			if err := onCreated(voucher); err != nil {
				result.FailedCount = input.Count - len(result.Created)
				result.ErrorMessage = "أُنشئ حساب على الراوتر ولكن تعذر حفظ سجل القسيمة محليًا؛ احتفظ بالرمز الظاهر في النتيجة."
				return result, fmt.Errorf("save voucher inventory after router creation: %w", err)
			}
		}
	}
	return result, nil
}

func voucherCreateArgs(input VoucherBatchInput, username string, passwords ...string) []string {
	args := []string{
		"=name=" + username,
		"=profile=" + input.Profile,
		"=server=" + input.Server,
		"=comment=bt-manager:" + input.Reference,
	}
	if len(passwords) > 0 && passwords[0] != "" {
		args = append(args, "=password="+passwords[0])
	}
	if input.LimitUptime != "" {
		args = append(args, "=limit-uptime="+input.LimitUptime)
	}
	return args
}

func randomVoucherCode(length int) (string, error) {
	return randomVoucherCodeForMode(length, "alphanumeric")
}

func randomVoucherCodeForMode(length int, mode string) (string, error) {
	if length < 1 || length > 32 {
		return "", errors.New("invalid voucher code length")
	}
	alphabet := voucherAlphabet
	switch mode {
	case "letters":
		alphabet = voucherLetters
	case "numbers":
		alphabet = voucherDigits
	case "alphanumeric", "":
		alphabet = voucherAlphabet
	default:
		return "", errors.New("invalid voucher character set")
	}
	var b strings.Builder
	b.Grow(length)
	limit := big.NewInt(int64(len(alphabet)))
	for i := 0; i < length; i++ {
		n, err := crand.Int(crand.Reader, limit)
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String(), nil
}

func safeRouterError(err error) string {
	if err == nil {
		return ""
	}
	// RouterOS may echo command values; never reflect any stored router password.
	message := strings.ReplaceAll(err.Error(), "\r", " ")
	message = strings.ReplaceAll(message, "\n", " ")
	if len(message) > 180 {
		message = message[:180]
	}
	return message
}
