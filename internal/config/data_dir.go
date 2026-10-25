package config

import (
	"errors"
	"os"
	"path/filepath"
)

func DefaultDataDir() (string, error) {
	if value := os.Getenv("BT_MANAGER_DATA_DIR"); value != "" { return filepath.Abs(value) }
	base, err := os.UserConfigDir()
	if err != nil { return "", err }
	if base == "" { return "", errors.New("OS did not provide a user config directory") }
	return filepath.Join(base, "bt-manager"), nil
}
