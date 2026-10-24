package routerosadapter

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var routerOSIDPattern = regexp.MustCompile("^\\*[0-9A-Fa-f]+$")

type HotSpotUserInput struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Password    string `json:"password,omitempty"`
	Profile     string `json:"profile"`
	Server      string `json:"server,omitempty"`
	LimitUptime string `json:"limitUptime,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

type HotSpotProfileInput struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name"`
	RateLimit      string `json:"rateLimit,omitempty"`
	SharedUsers    string `json:"sharedUsers,omitempty"`
	SessionTimeout string `json:"sessionTimeout,omitempty"`
	IdleTimeout    string `json:"idleTimeout,omitempty"`
}

func CreateHotSpotUser(ctx context.Context, request TestRequest, user HotSpotUserInput) error {
	user, err := normalizeUser(user, true)
	if err != nil {
		return err
	}
	args := []string{"=name=" + user.Name, "=password=" + user.Password}
	appendIfSet := func(key, value string) {
		if value != "" {
			args = append(args, "="+key+"="+value)
		}
	}
	appendIfSet("profile", user.Profile)
	appendIfSet("server", user.Server)
	appendIfSet("limit-uptime", user.LimitUptime)
	appendIfSet("comment", user.Comment)
	return runHotSpotMutation(ctx, request, "/ip/hotspot/user/add", args...)
}

func UpdateHotSpotUser(ctx context.Context, request TestRequest, user HotSpotUserInput) error {
	user, err := normalizeUser(user, false)
	if err != nil {
		return err
	}
	args := []string{"=.id=" + user.ID, "=name=" + user.Name}
	if user.Password != "" {
		args = append(args, "=password="+user.Password)
	}
	args = append(args,
		"=profile="+user.Profile,
		"=server="+user.Server,
		"=limit-uptime="+user.LimitUptime,
		"=comment="+user.Comment,
	)
	return runHotSpotMutation(ctx, request, "/ip/hotspot/user/set", args...)
}

func SetHotSpotUserDisabled(ctx context.Context, request TestRequest, id string, disabled bool) error {
	if !validRouterOSID(id) {
		return ErrInvalidRequest
	}
	command := "/ip/hotspot/user/enable"
	if disabled {
		command = "/ip/hotspot/user/disable"
	}
	return runHotSpotMutation(ctx, request, command, "=.id="+id)
}

func DeleteHotSpotUser(ctx context.Context, request TestRequest, id string) error {
	if !validRouterOSID(id) {
		return ErrInvalidRequest
	}
	return runHotSpotMutation(ctx, request, "/ip/hotspot/user/remove", "=.id="+id)
}

func CreateHotSpotProfile(ctx context.Context, request TestRequest, profile HotSpotProfileInput) error {
	profile, err := normalizeProfile(profile, true)
	if err != nil {
		return err
	}
	return runHotSpotMutation(ctx, request, "/ip/hotspot/user/profile/add", profileArgs(profile, false)...)
}

func UpdateHotSpotProfile(ctx context.Context, request TestRequest, profile HotSpotProfileInput) error {
	profile, err := normalizeProfile(profile, false)
	if err != nil {
		return err
	}
	args := append([]string{"=.id=" + profile.ID}, profileArgs(profile, true)...)
	return runHotSpotMutation(ctx, request, "/ip/hotspot/user/profile/set", args...)
}

func DeleteHotSpotProfile(ctx context.Context, request TestRequest, id string) error {
	if !validRouterOSID(id) {
		return ErrInvalidRequest
	}
	return runHotSpotMutation(ctx, request, "/ip/hotspot/user/profile/remove", "=.id="+id)
}

func runHotSpotMutation(ctx context.Context, request TestRequest, command string, args ...string) error {
	api, err := connectRouter(ctx, request)
	if err != nil {
		return err
	}
	defer api.Close()
	_, err = api.SendContext(ctx, command, args...)
	if err != nil {
		return fmt.Errorf("RouterOS rejected HotSpot command: %w", err)
	}
	return nil
}

func normalizeUser(user HotSpotUserInput, creating bool) (HotSpotUserInput, error) {
	user.ID = strings.TrimSpace(user.ID)
	user.Name = strings.TrimSpace(user.Name)
	user.Profile = strings.TrimSpace(user.Profile)
	user.Server = strings.TrimSpace(user.Server)
	user.LimitUptime = strings.TrimSpace(user.LimitUptime)
	user.Comment = strings.TrimSpace(user.Comment)
	if creating {
		if user.Name == "" || user.Password == "" {
			return HotSpotUserInput{}, ErrInvalidRequest
		}
	} else if !validRouterOSID(user.ID) || user.Name == "" {
		return HotSpotUserInput{}, ErrInvalidRequest
	}
	if !validText(user.Name, 64) || !validText(user.Password, 128) ||
		!validText(user.Profile, 64) || !validText(user.Server, 64) ||
		!validText(user.LimitUptime, 64) || !validText(user.Comment, 255) {
		return HotSpotUserInput{}, ErrInvalidRequest
	}
	if user.Profile == "" {
		user.Profile = "default"
	}
	if user.Server == "" {
		user.Server = "all"
	}
	return user, nil
}

func normalizeProfile(profile HotSpotProfileInput, creating bool) (HotSpotProfileInput, error) {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.RateLimit = strings.TrimSpace(profile.RateLimit)
	profile.SharedUsers = strings.TrimSpace(profile.SharedUsers)
	profile.SessionTimeout = strings.TrimSpace(profile.SessionTimeout)
	profile.IdleTimeout = strings.TrimSpace(profile.IdleTimeout)
	if profile.Name == "" || !validText(profile.Name, 64) ||
		!validText(profile.RateLimit, 64) || !validText(profile.SharedUsers, 10) ||
		!validText(profile.SessionTimeout, 64) || !validText(profile.IdleTimeout, 64) {
		return HotSpotProfileInput{}, ErrInvalidRequest
	}
	if !creating && !validRouterOSID(profile.ID) {
		return HotSpotProfileInput{}, ErrInvalidRequest
	}
	if profile.SharedUsers != "" {
		n, err := strconv.Atoi(profile.SharedUsers)
		if err != nil || n < 1 || n > 1000000 {
			return HotSpotProfileInput{}, ErrInvalidRequest
		}
	}
	return profile, nil
}

func profileArgs(profile HotSpotProfileInput, includeEmpty bool) []string {
	values := []struct{ key, value string }{
		{"name", profile.Name},
		{"rate-limit", profile.RateLimit},
		{"shared-users", profile.SharedUsers},
		{"session-timeout", profile.SessionTimeout},
		{"idle-timeout", profile.IdleTimeout},
	}
	args := make([]string, 0, len(values))
	for _, field := range values {
		if includeEmpty || field.value != "" {
			args = append(args, "="+field.key+"="+field.value)
		}
	}
	return args
}

func validRouterOSID(id string) bool {
	return routerOSIDPattern.MatchString(strings.TrimSpace(id))
}

func validText(value string, maxRunes int) bool {
	if len([]rune(value)) > maxRunes {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
