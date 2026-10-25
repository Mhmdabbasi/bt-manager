package routerosadapter

import (
	"context"
	"fmt"
	"strings"

	ros "github.com/Mhmdabbasi/routeros-api-Go"
)

const hotspotPreviewLimit = 50

// HotSpotOverview contains read-only snapshots from the common RouterOS HotSpot menus.
// Passwords and other credential fields are deliberately never requested or returned.
type HotSpotOverview struct {
	Counts         HotSpotCounts          `json:"counts"`
	Servers        []HotSpotServer        `json:"servers"`
	Profiles       []HotSpotProfile       `json:"profiles"`
	Users          []HotSpotUser          `json:"users"`
	ActiveSessions []HotSpotActiveSession `json:"activeSessions"`
	PreviewLimit   int                    `json:"previewLimit"`
	Truncated      bool                   `json:"truncated"`
}

type HotSpotCounts struct {
	Servers        int `json:"servers"`
	Profiles       int `json:"profiles"`
	Users          int `json:"users"`
	ActiveSessions int `json:"activeSessions"`
}

type HotSpotServer struct {
	Name       string `json:"name"`
	Interface  string `json:"interface"`
	AddressPool string `json:"addressPool"`
	Profile    string `json:"profile"`
	Disabled   bool   `json:"disabled"`
}

type HotSpotProfile struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RateLimit      string `json:"rateLimit"`
	SharedUsers    string `json:"sharedUsers"`
	SessionTimeout string `json:"sessionTimeout"`
	IdleTimeout    string `json:"idleTimeout"`
}

type HotSpotUser struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Profile    string `json:"profile"`
	Server     string `json:"server"`
	LimitUptime string `json:"limitUptime"`
	Disabled   bool   `json:"disabled"`
	Comment    string `json:"comment"`
}

type HotSpotActiveSession struct {
	ID        string `json:"id"`
	User      string `json:"user"`
	Address   string `json:"address"`
	MACAddress string `json:"macAddress"`
	Server    string `json:"server"`
	Uptime    string `json:"uptime"`
	BytesIn   string `json:"bytesIn"`
	BytesOut  string `json:"bytesOut"`
}

func HotSpotSnapshot(ctx context.Context, request TestRequest) (HotSpotOverview, error) {
	api, err := connectRouter(ctx, request)
	if err != nil {
		return HotSpotOverview{}, err
	}
	defer api.Close()

	servers, err := readRows(ctx, api, "/ip/hotspot/print", "name,interface,address-pool,profile,disabled")
	if err != nil {
		return HotSpotOverview{}, fmt.Errorf("read HotSpot servers: %w", err)
	}
	profiles, err := readRows(ctx, api, "/ip/hotspot/user/profile/print", ".id,name,rate-limit,shared-users,session-timeout,idle-timeout")
	if err != nil {
		return HotSpotOverview{}, fmt.Errorf("read HotSpot profiles: %w", err)
	}
	users, err := readRows(ctx, api, "/ip/hotspot/user/print", ".id,name,profile,server,limit-uptime,disabled,comment")
	if err != nil {
		return HotSpotOverview{}, fmt.Errorf("read HotSpot users: %w", err)
	}
	active, err := readRows(ctx, api, "/ip/hotspot/active/print", ".id,user,address,mac-address,server,uptime,bytes-in,bytes-out")
	if err != nil {
		return HotSpotOverview{}, fmt.Errorf("read active HotSpot sessions: %w", err)
	}

	result := HotSpotOverview{
		Counts: HotSpotCounts{
			Servers: len(servers), Profiles: len(profiles), Users: len(users), ActiveSessions: len(active),
		},
		Servers: make([]HotSpotServer, 0, min(len(servers), hotspotPreviewLimit)),
		Profiles: make([]HotSpotProfile, 0, min(len(profiles), hotspotPreviewLimit)),
		Users: make([]HotSpotUser, 0, min(len(users), hotspotPreviewLimit)),
		ActiveSessions: make([]HotSpotActiveSession, 0, min(len(active), hotspotPreviewLimit)),
		PreviewLimit: hotspotPreviewLimit,
		Truncated: len(servers) > hotspotPreviewLimit || len(profiles) > hotspotPreviewLimit ||
			len(users) > hotspotPreviewLimit || len(active) > hotspotPreviewLimit,
	}

	for _, row := range previewRows(servers) {
		result.Servers = append(result.Servers, HotSpotServer{
			Name: row["name"], Interface: row["interface"], AddressPool: row["address-pool"],
			Profile: row["profile"], Disabled: routerOSBool(row["disabled"]),
		})
	}
	for _, row := range previewRows(profiles) {
		result.Profiles = append(result.Profiles, HotSpotProfile{
			ID: row[".id"], Name: row["name"], RateLimit: row["rate-limit"], SharedUsers: row["shared-users"],
			SessionTimeout: row["session-timeout"], IdleTimeout: row["idle-timeout"],
		})
	}
	for _, row := range previewRows(users) {
		result.Users = append(result.Users, HotSpotUser{
			ID: row[".id"], Name: row["name"], Profile: row["profile"], Server: row["server"],
			LimitUptime: row["limit-uptime"], Disabled: routerOSBool(row["disabled"]), Comment: row["comment"],
		})
	}
	for _, row := range previewRows(active) {
		result.ActiveSessions = append(result.ActiveSessions, HotSpotActiveSession{
			ID: row[".id"], User: row["user"], Address: row["address"], MACAddress: row["mac-address"],
			Server: row["server"], Uptime: row["uptime"], BytesIn: row["bytes-in"], BytesOut: row["bytes-out"],
		})
	}
	return result, nil
}

func readRows(ctx context.Context, api *ros.API, command, properties string) ([]map[string]string, error) {
	responses, err := api.SendContext(ctx, command, "=.proplist="+properties)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]string, 0, len(responses))
	for _, response := range responses {
		if response != nil && response.Type == ros.ResponseReply && len(response.Data) > 0 {
			// Copy the data so this response no longer depends on the library's map.
			row := make(map[string]string, len(response.Data))
			for key, value := range response.Data {
				row[key] = value
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func previewRows(rows []map[string]string) []map[string]string {
	if len(rows) > hotspotPreviewLimit {
		return rows[:hotspotPreviewLimit]
	}
	return rows
}

func routerOSBool(value string) bool {
	switch value {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}


type ImportedHotSpotProfile struct {
	SourceID   string            `json:"sourceId"`
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties"`
}

type HotSpotProfilesSnapshot struct {
	Router   RouterInfo              `json:"router"`
	Profiles []ImportedHotSpotProfile `json:"profiles"`
}

// ReadHotSpotProfiles reads the full HotSpot user-profile list for durable local
// cataloging. It does not request users, passwords, or active sessions.
func ReadHotSpotProfiles(ctx context.Context, request TestRequest) (HotSpotProfilesSnapshot, error) {
	api, err := connectRouter(ctx, request)
	if err != nil {
		return HotSpotProfilesSnapshot{}, err
	}
	defer api.Close()

	system, err := api.System.GetInfoContext(ctx)
	if err != nil {
		return HotSpotProfilesSnapshot{}, fmt.Errorf("read RouterOS version for HotSpot profiles: %w", err)
	}
	const properties = ".id,name,address-pool,session-timeout,idle-timeout,keepalive-timeout,status-autorefresh,shared-users,rate-limit,add-mac-cookie,mac-cookie-timeout,address-list,transparent-proxy,use-radius"
	rows, err := readRows(ctx, api, "/ip/hotspot/user/profile/print", properties)
	if err != nil {
		return HotSpotProfilesSnapshot{}, fmt.Errorf("read HotSpot profiles: %w", err)
	}

	snapshot := HotSpotProfilesSnapshot{
		Router: RouterInfo{
			Identity: system.Identity, Version: system.Version, Model: system.Model,
			BoardName: system.BoardName, Uptime: system.Uptime, CPU: system.CPU,
			CPUCount: system.CPUCount, FreeMemory: system.FreeMemory, TotalMemory: system.TotalMemory,
		},
		Profiles: make([]ImportedHotSpotProfile, 0, len(rows)),
	}
	for _, row := range rows {
		name := strings.TrimSpace(row["name"])
		sourceID := sourceIdentity(row, name)
		if name == "" || sourceID == "" {
			return HotSpotProfilesSnapshot{}, fmt.Errorf("invalid HotSpot profile returned by RouterOS")
		}
		snapshot.Profiles = append(snapshot.Profiles, ImportedHotSpotProfile{
			SourceID: sourceID, Name: name,
			Properties: safeProperties(row, strings.Split(properties, ",")),
		})
	}
	return snapshot, nil
}
