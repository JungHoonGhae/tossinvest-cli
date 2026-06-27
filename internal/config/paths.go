package config

import (
	"os"
	"path/filepath"
)

const AppName = "tossctl"

type Paths struct {
	ConfigDir       string
	CacheDir        string
	ConfigFile      string
	SessionFile     string
	LineageFile     string
	CredentialsFile string
	TokenFile       string
}

func DefaultPaths() (Paths, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}

	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return Paths{}, err
	}

	configDir := filepath.Join(configRoot, AppName)

	return Paths{
		ConfigDir:       configDir,
		CacheDir:        filepath.Join(cacheRoot, AppName),
		ConfigFile:      filepath.Join(configDir, "config.json"),
		SessionFile:     filepath.Join(configDir, "session.json"),
		LineageFile:     filepath.Join(configDir, "trading-lineage.json"),
		CredentialsFile: filepath.Join(configDir, "openapi-credentials.json"),
		TokenFile:       filepath.Join(cacheRoot, AppName, "openapi-token.json"),
	}, nil
}
