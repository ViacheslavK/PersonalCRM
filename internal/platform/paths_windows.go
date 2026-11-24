//go:build windows

package platform

import (
	"os"
	"path/filepath"
)

func defaultOSDirs() (dataDir string, configDir string, err error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	appDir := filepath.Join(base, AppName)
	return appDir, appDir, nil
}
