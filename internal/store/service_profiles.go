package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidServiceProfileSnapshot = errors.New("invalid service profile snapshot")

type ImportedServiceProfile struct {
	SourceID   string
	Name       string
	Properties map[string]string
}

type HotSpotProfileSyncInput struct {
	Address         string
	Port            int
	UseTLS          bool
	RouterIdentity  string
	RouterOSVersion string
	Model           string
	Profiles        []ImportedServiceProfile
}

type ServiceProfileSyncResult struct {
	RouterID        int64  `json:"routerId"`
	RouterKey       string `json:"routerKey"`
	RouterIdentity  string `json:"routerIdentity"`
	RouterOSVersion string `json:"routerOsVersion"`
	Model           string `json:"model"`
	SourceSystem    string `json:"sourceSystem"`
	ProfileCount    int    `json:"profileCount"`
	SyncedAt        string `json:"syncedAt"`
}

type ServiceProfileRecord struct {
	ID           int64             `json:"id"`
	RouterID     int64             `json:"routerId"`
	SourceSystem string            `json:"sourceSystem"`
	SourceID     string            `json:"sourceId"`
	Name         string            `json:"name"`
	Properties   map[string]string `json:"properties"`
	Present      bool              `json:"present"`
	LastSeenAt   string            `json:"lastSeenAt"`
}

func (db *DB) SyncHotSpotProfilesSnapshot(ctx context.Context, input HotSpotProfileSyncInput) (ServiceProfileSyncResult, error) {
	input.Address = strings.TrimSpace(input.Address)
	input.RouterIdentity = strings.TrimSpace(input.RouterIdentity)
	input.RouterOSVersion = strings.TrimSpace(input.RouterOSVersion)
	input.Model = strings.TrimSpace(input.Model)
	if input.Address == "" || input.Port < 1 || input.Port > 65535 || input.RouterOSVersion == "" || len(input.Profiles) > 100000 {
		return ServiceProfileSyncResult{}, ErrInvalidServiceProfileSnapshot
	}

	routerKey := strings.ToLower(input.Address) + "|" + fmt.Sprint(input.Port) + "|" + fmt.Sprint(input.UseTLS)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ServiceProfileSyncResult{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `INSERT INTO routers
		(router_key,display_name,address,api_port,use_tls,router_identity,routeros_version,model,last_seen_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		ON CONFLICT(router_key) DO UPDATE SET
		display_name=excluded.display_name,address=excluded.address,api_port=excluded.api_port,
		use_tls=excluded.use_tls,router_identity=excluded.router_identity,
		routeros_version=excluded.routeros_version,model=excluded.model,
		last_seen_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		routerKey, input.RouterIdentity, input.Address, input.Port, boolInt(input.UseTLS),
		input.RouterIdentity, input.RouterOSVersion, input.Model)
	if err != nil {
		return ServiceProfileSyncResult{}, fmt.Errorf("upsert HotSpot router: %w", err)
	}

	var routerID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM routers WHERE router_key=?`, routerKey).Scan(&routerID); err != nil {
		return ServiceProfileSyncResult{}, err
	}
	nowExpr := "strftime('%Y-%m-%dT%H:%M:%fZ','now')"
	if _, err := tx.ExecContext(ctx, `UPDATE service_profiles SET is_present=0,updated_at=`+nowExpr+
		` WHERE router_id=? AND source_system='hotspot'`, routerID); err != nil {
		return ServiceProfileSyncResult{}, err
	}

	for _, profile := range input.Profiles {
		profile.SourceID = strings.TrimSpace(profile.SourceID)
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.SourceID == "" {
			profile.SourceID = profile.Name
		}
		if profile.SourceID == "" || profile.Name == "" || len([]rune(profile.Name)) > 255 {
			return ServiceProfileSyncResult{}, ErrInvalidServiceProfileSnapshot
		}
		propertiesJSON, err := marshalProperties(profile.Properties)
		if err != nil {
			return ServiceProfileSyncResult{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO service_profiles
			(router_id,source_system,source_id,name,properties_json,is_present,last_seen_at,updated_at)
			VALUES(?,'hotspot',?,?,?,1,`+nowExpr+`,`+nowExpr+`)
			ON CONFLICT(router_id,source_system,source_id) DO UPDATE SET
			name=excluded.name,properties_json=excluded.properties_json,is_present=1,
			last_seen_at=`+nowExpr+`,updated_at=`+nowExpr,
			routerID, profile.SourceID, profile.Name, propertiesJSON)
		if err != nil {
			return ServiceProfileSyncResult{}, fmt.Errorf("upsert HotSpot service profile: %w", err)
		}
	}

	var syncedAt string
	if err := tx.QueryRowContext(ctx, "SELECT "+nowExpr).Scan(&syncedAt); err != nil {
		return ServiceProfileSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ServiceProfileSyncResult{}, err
	}
	return ServiceProfileSyncResult{
		RouterID: routerID, RouterKey: routerKey, RouterIdentity: input.RouterIdentity,
		RouterOSVersion: input.RouterOSVersion, Model: input.Model,
		SourceSystem: "hotspot", ProfileCount: len(input.Profiles), SyncedAt: syncedAt,
	}, nil
}

func (db *DB) ListServiceProfiles(ctx context.Context, routerID int64, sourceSystem string, includeMissing bool) ([]ServiceProfileRecord, error) {
	sourceSystem = strings.ToLower(strings.TrimSpace(sourceSystem))
	if routerID < 1 {
		return nil, ErrImportedRouterNotFound
	}
	if sourceSystem != "hotspot" && sourceSystem != "usermanager" {
		return nil, ErrInvalidServiceProfileSnapshot
	}
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM routers WHERE id=?`, routerID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrImportedRouterNotFound
	}

	query := `SELECT id,router_id,source_system,source_id,name,properties_json,is_present,last_seen_at
		FROM service_profiles WHERE router_id=? AND source_system=?`
	if !includeMissing {
		query += " AND is_present=1"
	}
	query += " ORDER BY is_present DESC,name COLLATE NOCASE"
	rows, err := db.QueryContext(ctx, query, routerID, sourceSystem)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := make([]ServiceProfileRecord, 0)
	for rows.Next() {
		var profile ServiceProfileRecord
		var propertiesJSON string
		var present int
		if err := rows.Scan(&profile.ID, &profile.RouterID, &profile.SourceSystem, &profile.SourceID,
			&profile.Name, &propertiesJSON, &present, &profile.LastSeenAt); err != nil {
			return nil, err
		}
		profile.Properties = parseProperties(propertiesJSON)
		profile.Present = present == 1
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return profiles, nil
}
