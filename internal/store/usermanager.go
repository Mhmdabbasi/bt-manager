package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidUserManagerSnapshot = errors.New("invalid User Manager snapshot")
	ErrImportedRouterNotFound = errors.New("imported router not found")
)

type ImportedUserManagerProfile struct {
	SourceID   string
	Name       string
	Properties map[string]string
}

type ImportedUserManagerLimitation struct {
	SourceID   string
	Name       string
	Properties map[string]string
}

type ImportedUserManagerProfileLimitation struct {
	ProfileSourceID    string
	ProfileName        string
	LimitationSourceID string
	LimitationName     string
	Position           int
	Properties         map[string]string
}

type UserManagerSyncInput struct {
	Address            string
	Port               int
	UseTLS             bool
	RouterIdentity     string
	RouterOSVersion    string
	Model              string
	Profiles           []ImportedUserManagerProfile
	Limitations        []ImportedUserManagerLimitation
	ProfileLimitations []ImportedUserManagerProfileLimitation
}

type UserManagerSyncResult struct {
	RouterID          int64  `json:"routerId"`
	RouterKey          string `json:"routerKey"`
	RouterIdentity    string `json:"routerIdentity"`
	RouterOSVersion   string `json:"routerOsVersion"`
	Model             string `json:"model"`
	ProfileCount      int    `json:"profileCount"`
	LimitationCount   int    `json:"limitationCount"`
	RelationshipCount int    `json:"relationshipCount"`
	SyncedAt          string `json:"syncedAt"`
}

type UserManagerLimitationRecord struct {
	ID         int64             `json:"id"`
	SourceID   string            `json:"sourceId"`
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties"`
	Present    bool              `json:"present"`
	LastSeenAt string            `json:"lastSeenAt"`
}

type UserManagerProfileLimitationRecord struct {
	Position   int                         `json:"position"`
	Properties map[string]string           `json:"properties"`
	Limitation UserManagerLimitationRecord `json:"limitation"`
}

type UserManagerProfileRecord struct {
	ID          int64                                `json:"id"`
	RouterID    int64                                `json:"routerId"`
	SourceID    string                               `json:"sourceId"`
	Name        string                               `json:"name"`
	Properties  map[string]string                    `json:"properties"`
	Present     bool                                 `json:"present"`
	LastSeenAt  string                               `json:"lastSeenAt"`
	Limitations []UserManagerProfileLimitationRecord `json:"limitations"`
}

func (db *DB) SyncUserManagerSnapshot(ctx context.Context, input UserManagerSyncInput) (UserManagerSyncResult, error) {
	input.Address = strings.TrimSpace(input.Address)
	input.RouterIdentity = strings.TrimSpace(input.RouterIdentity)
	input.RouterOSVersion = strings.TrimSpace(input.RouterOSVersion)
	input.Model = strings.TrimSpace(input.Model)
	if input.Address == "" || input.Port < 1 || input.Port > 65535 || input.RouterOSVersion == "" ||
		len(input.Profiles) > 100000 || len(input.Limitations) > 100000 || len(input.ProfileLimitations) > 200000 {
		return UserManagerSyncResult{}, ErrInvalidUserManagerSnapshot
	}

	routerKey := strings.ToLower(input.Address) + "|" + fmt.Sprint(input.Port) + "|" + fmt.Sprint(input.UseTLS)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return UserManagerSyncResult{}, err
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
		return UserManagerSyncResult{}, fmt.Errorf("upsert User Manager router: %w", err)
	}
	var routerID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM routers WHERE router_key=?`, routerKey).Scan(&routerID); err != nil {
		return UserManagerSyncResult{}, err
	}

	nowExpr := "strftime('%Y-%m-%dT%H:%M:%fZ','now')"
	_, err = tx.ExecContext(ctx, `UPDATE service_profiles SET is_present=0,updated_at=`+nowExpr+
		` WHERE router_id=? AND source_system='usermanager'`, routerID)
	if err != nil {
		return UserManagerSyncResult{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE um_limitations SET is_present=0,updated_at=`+nowExpr+` WHERE router_id=?`, routerID)
	if err != nil {
		return UserManagerSyncResult{}, err
	}

	profileIDs := make(map[string]int64, len(input.Profiles))
	profileNames := make(map[string]int64, len(input.Profiles))
	for _, profile := range input.Profiles {
		profile.SourceID = strings.TrimSpace(profile.SourceID)
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.SourceID == "" {
			profile.SourceID = profile.Name
		}
		if profile.SourceID == "" || profile.Name == "" || len([]rune(profile.Name)) > 255 {
			return UserManagerSyncResult{}, ErrInvalidUserManagerSnapshot
		}
		propertiesJSON, err := marshalProperties(profile.Properties)
		if err != nil {
			return UserManagerSyncResult{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO service_profiles
			(router_id,source_system,source_id,name,properties_json,is_present,last_seen_at,updated_at)
			VALUES(?,'usermanager',?,?,?,1,`+nowExpr+`,`+nowExpr+`)
			ON CONFLICT(router_id,source_system,source_id) DO UPDATE SET
			name=excluded.name,properties_json=excluded.properties_json,is_present=1,
			last_seen_at=`+nowExpr+`,updated_at=`+nowExpr,
			routerID, profile.SourceID, profile.Name, propertiesJSON)
		if err != nil {
			return UserManagerSyncResult{}, fmt.Errorf("upsert User Manager profile: %w", err)
		}
		var id int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM service_profiles WHERE router_id=? AND source_system='usermanager' AND source_id=?`,
			routerID, profile.SourceID).Scan(&id); err != nil {
			return UserManagerSyncResult{}, err
		}
		profileIDs[profile.SourceID] = id
		profileNames[profile.Name] = id
	}

	limitationIDs := make(map[string]int64, len(input.Limitations))
	limitationNames := make(map[string]int64, len(input.Limitations))
	for _, limitation := range input.Limitations {
		limitation.SourceID = strings.TrimSpace(limitation.SourceID)
		limitation.Name = strings.TrimSpace(limitation.Name)
		if limitation.SourceID == "" {
			limitation.SourceID = limitation.Name
		}
		if limitation.SourceID == "" || limitation.Name == "" || len([]rune(limitation.Name)) > 255 {
			return UserManagerSyncResult{}, ErrInvalidUserManagerSnapshot
		}
		propertiesJSON, err := marshalProperties(limitation.Properties)
		if err != nil {
			return UserManagerSyncResult{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO um_limitations
			(router_id,source_id,name,properties_json,is_present,last_seen_at,updated_at)
			VALUES(?,?,?,?,1,`+nowExpr+`,`+nowExpr+`)
			ON CONFLICT(router_id,source_id) DO UPDATE SET
			name=excluded.name,properties_json=excluded.properties_json,is_present=1,
			last_seen_at=`+nowExpr+`,updated_at=`+nowExpr,
			routerID, limitation.SourceID, limitation.Name, propertiesJSON)
		if err != nil {
			return UserManagerSyncResult{}, fmt.Errorf("upsert User Manager limitation: %w", err)
		}
		var id int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM um_limitations WHERE router_id=? AND source_id=?`,
			routerID, limitation.SourceID).Scan(&id); err != nil {
			return UserManagerSyncResult{}, err
		}
		limitationIDs[limitation.SourceID] = id
		limitationNames[limitation.Name] = id
	}

	// Refresh relationships only after all three RouterOS reads succeeded.
	_, err = tx.ExecContext(ctx, `DELETE FROM um_profile_limitations
		WHERE profile_id IN (SELECT id FROM service_profiles WHERE router_id=? AND source_system='usermanager')`, routerID)
	if err != nil {
		return UserManagerSyncResult{}, err
	}

	relationshipCount := 0
	for _, relation := range input.ProfileLimitations {
		profileSourceID := strings.TrimSpace(relation.ProfileSourceID)
		profileName := strings.TrimSpace(relation.ProfileName)
		limitationSourceID := strings.TrimSpace(relation.LimitationSourceID)
		limitationName := strings.TrimSpace(relation.LimitationName)
		profileID := profileIDs[profileSourceID]
		if profileID == 0 {
			profileID = profileNames[profileName]
		}
		limitationID := limitationIDs[limitationSourceID]
		if limitationID == 0 {
			limitationID = limitationNames[limitationName]
		}
		if profileID == 0 || limitationID == 0 {
			return UserManagerSyncResult{}, fmt.Errorf("%w: profile-limitation references an unknown profile or limitation", ErrInvalidUserManagerSnapshot)
		}
		propertiesJSON, err := marshalProperties(relation.Properties)
		if err != nil {
			return UserManagerSyncResult{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO um_profile_limitations(profile_id,limitation_id,position,properties_json)
			VALUES(?,?,?,?)`, profileID, limitationID, relation.Position, propertiesJSON)
		if err != nil {
			return UserManagerSyncResult{}, fmt.Errorf("save User Manager profile-limitation relationship: %w", err)
		}
		relationshipCount++
	}

	var syncedAt string
	if err := tx.QueryRowContext(ctx, "SELECT "+nowExpr).Scan(&syncedAt); err != nil {
		return UserManagerSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return UserManagerSyncResult{}, err
	}
	return UserManagerSyncResult{
		RouterID: routerID, RouterKey: routerKey, RouterIdentity: input.RouterIdentity,
		RouterOSVersion: input.RouterOSVersion, Model: input.Model,
		ProfileCount: len(input.Profiles), LimitationCount: len(input.Limitations),
		RelationshipCount: relationshipCount, SyncedAt: syncedAt,
	}, nil
}

func (db *DB) ListUserManagerProfiles(ctx context.Context, routerID int64) ([]UserManagerProfileRecord, error) {
	if routerID < 1 {
		return nil, ErrImportedRouterNotFound
	}
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM routers WHERE id=?`, routerID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrImportedRouterNotFound
	}
	rows, err := db.QueryContext(ctx, `SELECT id,router_id,source_id,name,properties_json,is_present,last_seen_at
		FROM service_profiles WHERE router_id=? AND source_system='usermanager'
		ORDER BY is_present DESC,name COLLATE NOCASE`, routerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := make([]UserManagerProfileRecord, 0)
	byID := make(map[int64]int, 64)
	for rows.Next() {
		var profile UserManagerProfileRecord
		var propertiesJSON string
		var present int
		if err := rows.Scan(&profile.ID, &profile.RouterID, &profile.SourceID, &profile.Name,
			&propertiesJSON, &present, &profile.LastSeenAt); err != nil {
			return nil, err
		}
		profile.Properties = parseProperties(propertiesJSON)
		profile.Present = present == 1
		profile.Limitations = make([]UserManagerProfileLimitationRecord, 0)
		byID[profile.ID] = len(profiles)
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	relRows, err := db.QueryContext(ctx, `SELECT r.profile_id,r.position,r.properties_json,
		l.id,l.source_id,l.name,l.properties_json,l.is_present,l.last_seen_at
		FROM um_profile_limitations r
		JOIN service_profiles p ON p.id=r.profile_id
		JOIN um_limitations l ON l.id=r.limitation_id
		WHERE p.router_id=? AND p.source_system='usermanager'
		ORDER BY r.profile_id,r.position,l.name COLLATE NOCASE`, routerID)
	if err != nil {
		return nil, err
	}
	defer relRows.Close()
	for relRows.Next() {
		var profileID int64
		var position int
		var relationJSON, limitationJSON string
		var limitation UserManagerLimitationRecord
		var present int
		if err := relRows.Scan(&profileID, &position, &relationJSON,
			&limitation.ID, &limitation.SourceID, &limitation.Name, &limitationJSON,
			&present, &limitation.LastSeenAt); err != nil {
			return nil, err
		}
		profileIndex, ok := byID[profileID]
		if !ok {
			continue
		}
		limitation.Properties = parseProperties(limitationJSON)
		limitation.Present = present == 1
		profiles[profileIndex].Limitations = append(profiles[profileIndex].Limitations, UserManagerProfileLimitationRecord{
			Position: position, Properties: parseProperties(relationJSON), Limitation: limitation,
		})
	}
	if err := relRows.Err(); err != nil {
		return nil, err
	}
	return profiles, nil
}

func marshalProperties(properties map[string]string) (string, error) {
	if properties == nil {
		properties = map[string]string{}
	}
	data, err := json.Marshal(properties)
	if err != nil {
		return "", fmt.Errorf("encode imported properties: %w", err)
	}
	return string(data), nil
}

func parseProperties(value string) map[string]string {
	var properties map[string]string
	if err := json.Unmarshal([]byte(value), &properties); err != nil || properties == nil {
		return map[string]string{}
	}
	return properties
}
