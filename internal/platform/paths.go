package platform

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	AppName        = "PersonalCRM"
	EnvDataDir     = "PCRM_DATA_DIR"
	PortableMarker = "pcrm.portable"
	DBFileName     = "pcrm.db"
	ConfigFileName = "config.toml"
)

// Paths holds all resolved filesystem locations used by PersonalCRM.
type Paths struct {
	DataDir    string
	ConfigDir  string
	DBPath     string
	ConfigPath string
	BackupDir  string
	LogDir     string
	PluginsDir string
	Portable   bool
}

// ResolvePaths determines data/config directories using the precedence:
// 1. Explicit overrideDataDir (--data-dir flag)
// 2. PCRM_DATA_DIR environment variable
// 3. Portable mode (pcrm.portable or pcrm.db next to the running executable)
// 4. OS-standard user directories
func ResolvePaths(overrideDataDir string) (Paths, error) {
	if dir := strings.TrimSpace(overrideDataDir); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return Paths{}, err
		}
		return buildUnifiedPaths(abs, false), nil
	}

	if dir := strings.TrimSpace(os.Getenv(EnvDataDir)); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return Paths{}, err
		}
		return buildUnifiedPaths(abs, false), nil
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		if isPortableDir(exeDir) {
			return buildUnifiedPaths(filepath.Join(exeDir, "data"), true), nil
		}
	}

	dataDir, configDir, err := defaultOSDirs()
	if err != nil {
		return Paths{}, err
	}

	return Paths{
		DataDir:    dataDir,
		ConfigDir:  configDir,
		DBPath:     filepath.Join(dataDir, DBFileName),
		ConfigPath: filepath.Join(configDir, ConfigFileName),
		BackupDir:  filepath.Join(dataDir, "backups"),
		LogDir:     filepath.Join(dataDir, "logs"),
		PluginsDir: filepath.Join(dataDir, "plugins"),
		Portable:   false,
	}, nil
}

// EnsureDirs creates all necessary directories if they do not exist yet.
func EnsureDirs(p Paths) error {
	dirs := []string{
		p.DataDir,
		p.ConfigDir,
		p.BackupDir,
		p.LogDir,
		p.PluginsDir,
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func buildUnifiedPaths(baseDir string, portable bool) Paths {
	return Paths{
		DataDir:    baseDir,
		ConfigDir:  baseDir,
		DBPath:     filepath.Join(baseDir, DBFileName),
		ConfigPath: filepath.Join(baseDir, ConfigFileName),
		BackupDir:  filepath.Join(baseDir, "backups"),
		LogDir:     filepath.Join(baseDir, "logs"),
		PluginsDir: filepath.Join(baseDir, "plugins"),
		Portable:   portable,
	}
}

func isPortableDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, PortableMarker)); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, DBFileName)); err == nil {
		return true
	}
	return false
}
