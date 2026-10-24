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
)

var voucherPrefixPattern = regexp.MustCompile("^[A-Za-z0-9_-]{0,32}$")

type VoucherBatchInput struct {
	Reference   string `json:"reference"`
	Prefix      string `json:"prefix"`
	Count       int    `json:"count"`
	Profile     string `json:"profile"`
	Server      string `json:"server"`
	LimitUptime string `json:"limitUptime"`
}

type IssuedVoucher struct {
	Username    string `json:"username"`
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
	if input.Count < 1 || input.Count > MaxVoucherBatchSize ||
		input.Profile == "" || !voucherPrefixPattern.MatchString(input.Prefix) ||
		!validText(input.Profile, 64) || !validText(input.Server, 64) ||
		!validText(input.LimitUptime, 64) {
		return VoucherBatchInput{}, ErrInvalidRequest
	}
	if input.Server == "" {
		input.Server = "all"
	}
	if input.Prefix != "" && len(input.Prefix)+voucherCodeLength > 64 {
		return VoucherBatchInput{}, ErrInvalidRequest
	}
	return input, nil
}

// GenerateHotSpotVouchers creates username-only HotSpot accounts. It intentionally
// omits the password attribute so RouterOS uses the username-only login workflow.
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
		code, err := randomVoucherCode(voucherCodeLength)
		if err != nil {
			result.FailedCount = input.Count - len(result.Created)
			result.ErrorMessage = "تعذر توليد رمز آمن للقسيمة."
			return result, err
		}
		voucher := IssuedVoucher{
			Username: input.Prefix + code, ProfileName: input.Profile,
			ServerName: input.Server, LimitUptime: input.LimitUptime,
		}
		args := []string{
			"=name=" + voucher.Username,
			"=profile=" + input.Profile,
			"=server=" + input.Server,
			"=comment=bt-manager:" + input.Reference,
		}
		if input.LimitUptime != "" {
			args = append(args, "=limit-uptime="+input.LimitUptime)
		}
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

func randomVoucherCode(length int) (string, error) {
	if length < 1 || length > 32 {
		return "", errors.New("invalid voucher code length")
	}
	var b strings.Builder
	b.Grow(length)
	limit := big.NewInt(int64(len(voucherAlphabet)))
	for i := 0; i < length; i++ {
		n, err := crand.Int(crand.Reader, limit)
		if err != nil {
			return "", err
		}
		b.WriteByte(voucherAlphabet[n.Int64()])
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
