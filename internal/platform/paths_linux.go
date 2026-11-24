//go:build linux

package platform

import (
	"os"
	"path/filepath"
)

func defaultOSDirs() (dataDir string, configDir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		xdgData = filepath.Join(home, ".local", "share")
	}
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	return filepath.Join(xdgData, "pcrm"), filepath.Join(xdgConfig, "pcrm"), nil
}
