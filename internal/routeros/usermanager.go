package routerosadapter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrUnsupportedUserManagerVersion = errors.New("unsupported RouterOS version for User Manager")
	ErrInvalidUserManagerSnapshot    = errors.New("invalid User Manager response")
)

type UserManagerProfile struct {
	SourceID   string            `json:"sourceId"`
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties"`
}

type UserManagerLimitation struct {
	SourceID   string            `json:"sourceId"`
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties"`
}

type UserManagerProfileLimitation struct {
	ProfileSourceID    string            `json:"profileSourceId"`
	ProfileName        string            `json:"profileName"`
	LimitationSourceID string            `json:"limitationSourceId"`
	LimitationName     string            `json:"limitationName"`
	Position           int               `json:"position"`
	Properties         map[string]string `json:"properties"`
}

type UserManagerSnapshot struct {
	Router             RouterInfo                     `json:"router"`
	MajorVersion       int                            `json:"majorVersion"`
	Profiles           []UserManagerProfile           `json:"profiles"`
	Limitations        []UserManagerLimitation        `json:"limitations"`
	ProfileLimitations []UserManagerProfileLimitation `json:"profileLimitations"`
}

type userManagerPaths struct {
	profiles            string
	limitations         string
	profileLimitations  string
	profileProperties   string
	limitationProperties string
	relationProperties  string
}

func pathsForUserManagerVersion(version string) (userManagerPaths, int, error) {
	major, ok := routerOSMajorVersion(version)
	if !ok {
		return userManagerPaths{}, 0, ErrUnsupportedUserManagerVersion
	}
	switch major {
	case 6:
		return userManagerPaths{
			profiles: "/tool/user-manager/profile/print",
			limitations: "/tool/user-manager/profile/limitation/print",
			profileLimitations: "/tool/user-manager/profile/profile-limitation/print",
			profileProperties: ".id,name,owner,name-for-users,starts-at,override-shared-users,price,validity",
			limitationProperties: ".id,name,owner,group-name,rate-limit,transfer-limit,uptime-limit,download-limit,upload-limit,address-list,ip-pool",
			relationProperties: ".id,owner,profile,limitation,comment,from-time,till-time,weekdays",
		}, major, nil
	case 7:
		return userManagerPaths{
			profiles: "/user-manager/profile/print",
			limitations: "/user-manager/limitation/print",
			profileLimitations: "/user-manager/profile-limitation/print",
			profileProperties: ".id,name,name-for-users,validity,starts-when,price,override-shared-users",
			limitationProperties: ".id,name,transfer-limit,uptime-limit,download-limit,upload-limit,rate-limit-rx,rate-limit-tx,reset-counters-interval,reset-counters-start-time,rate-limit-priority,rate-limit-min-rx,rate-limit-min-tx,rate-limit-burst-rx,rate-limit-burst-tx,rate-limit-burst-threshold-rx,rate-limit-burst-threshold-tx,rate-limit-burst-time-rx,rate-limit-burst-time-tx",
			relationProperties: ".id,profile,limitation,comment,from-time,till-time,weekdays",
		}, major, nil
	default:
		return userManagerPaths{}, major, ErrUnsupportedUserManagerVersion
	}
}

func routerOSMajorVersion(version string) (int, bool) {
	version = strings.TrimSpace(version)
	start := -1
	for i, r := range version {
		if unicode.IsDigit(r) {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, false
	}
	version = version[start:]
	majorText, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(majorText)
	return major, err == nil
}

// ReadUserManagerSnapshot is intentionally read-only. It requests only profile, limitation,
// and relationship properties needed by the application; user/password records are never read.
func ReadUserManagerSnapshot(ctx context.Context, request TestRequest) (UserManagerSnapshot, error) {
	api, err := connectRouter(ctx, request)
	if err != nil {
		return UserManagerSnapshot{}, err
	}
	defer api.Close()

	system, err := api.System.GetInfoContext(ctx)
	if err != nil {
		return UserManagerSnapshot{}, fmt.Errorf("read RouterOS version for User Manager: %w", err)
	}
	paths, major, err := pathsForUserManagerVersion(system.Version)
	if err != nil {
		return UserManagerSnapshot{}, err
	}

	profileRows, err := readRows(ctx, api, paths.profiles, paths.profileProperties)
	if err != nil {
		return UserManagerSnapshot{}, fmt.Errorf("read User Manager profiles: %w", err)
	}
	limitationRows, err := readRows(ctx, api, paths.limitations, paths.limitationProperties)
	if err != nil {
		return UserManagerSnapshot{}, fmt.Errorf("read User Manager limitations: %w", err)
	}
	relationRows, err := readRows(ctx, api, paths.profileLimitations, paths.relationProperties)
	if err != nil {
		return UserManagerSnapshot{}, fmt.Errorf("read User Manager profile-limitation relationships: %w", err)
	}

	snapshot := UserManagerSnapshot{
		Router: RouterInfo{
			Identity: system.Identity, Version: system.Version, Model: system.Model,
			BoardName: system.BoardName, Uptime: system.Uptime, CPU: system.CPU,
			CPUCount: system.CPUCount, FreeMemory: system.FreeMemory, TotalMemory: system.TotalMemory,
		},
		MajorVersion: major,
		Profiles: make([]UserManagerProfile, 0, len(profileRows)),
		Limitations: make([]UserManagerLimitation, 0, len(limitationRows)),
		ProfileLimitations: make([]UserManagerProfileLimitation, 0, len(relationRows)),
	}
	profileNames := make(map[string]struct{}, len(profileRows))
	limitationNames := make(map[string]struct{}, len(limitationRows))
	for _, row := range profileRows {
		name := strings.TrimSpace(row["name"])
		sourceID := sourceIdentity(row, name)
		if name == "" || sourceID == "" {
			return UserManagerSnapshot{}, ErrInvalidUserManagerSnapshot
		}
		if _, duplicate := profileNames[name]; duplicate {
			return UserManagerSnapshot{}, fmt.Errorf("%w: duplicate profile name", ErrInvalidUserManagerSnapshot)
		}
		profileNames[name] = struct{}{}
		snapshot.Profiles = append(snapshot.Profiles, UserManagerProfile{
			SourceID: sourceID, Name: name,
			Properties: safeProperties(row, strings.Split(paths.profileProperties, ",")),
		})
	}
	for _, row := range limitationRows {
		name := strings.TrimSpace(row["name"])
		sourceID := sourceIdentity(row, name)
		if name == "" || sourceID == "" {
			return UserManagerSnapshot{}, ErrInvalidUserManagerSnapshot
		}
		if _, duplicate := limitationNames[name]; duplicate {
			return UserManagerSnapshot{}, fmt.Errorf("%w: duplicate limitation name", ErrInvalidUserManagerSnapshot)
		}
		limitationNames[name] = struct{}{}
		snapshot.Limitations = append(snapshot.Limitations, UserManagerLimitation{
			SourceID: sourceID, Name: name,
			Properties: safeProperties(row, strings.Split(paths.limitationProperties, ",")),
		})
	}
	for i, row := range relationRows {
		profileName := strings.TrimSpace(row["profile"])
		limitationName := strings.TrimSpace(row["limitation"])
		if profileName == "" || limitationName == "" {
			return UserManagerSnapshot{}, fmt.Errorf("%w: incomplete profile-limitation relationship", ErrInvalidUserManagerSnapshot)
		}
		if _, ok := profileNames[profileName]; !ok {
			return UserManagerSnapshot{}, fmt.Errorf("%w: relationship references an unavailable profile", ErrInvalidUserManagerSnapshot)
		}
		if _, ok := limitationNames[limitationName]; !ok {
			return UserManagerSnapshot{}, fmt.Errorf("%w: relationship references an unavailable limitation", ErrInvalidUserManagerSnapshot)
		}
		snapshot.ProfileLimitations = append(snapshot.ProfileLimitations, UserManagerProfileLimitation{
			ProfileSourceID: profileName, ProfileName: profileName,
			LimitationSourceID: limitationName, LimitationName: limitationName,
			Position: i,
			Properties: safeProperties(row, strings.Split(paths.relationProperties, ",")),
		})
	}
	return snapshot, nil
}

func sourceIdentity(row map[string]string, fallback string) string {
	if id := strings.TrimSpace(row[".id"]); id != "" {
		return id
	}
	return strings.TrimSpace(fallback)
}

func safeProperties(row map[string]string, allowed []string) map[string]string {
	properties := make(map[string]string, len(allowed))
	for _, key := range allowed {
		key = strings.TrimSpace(key)
		if key == "" || key == ".id" {
			continue
		}
		if value, ok := row[key]; ok {
			properties[key] = value
		}
	}
	return properties
}
