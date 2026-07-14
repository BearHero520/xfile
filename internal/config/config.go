package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Addr          string
	DataDir       string
	FilesDir      string
	DatabasePath  string
	StaticDir     string
	SiteName      string
	SessionSecret string
}

func Load() Config {
	dataDir := env("XFILE_DATA_DIR", "data")
	filesDir := env("XFILE_FILES_DIR", filepath.Join(dataDir, "files"))

	return Config{
		Addr:          env("XFILE_ADDR", ":3008"),
		DataDir:       dataDir,
		FilesDir:      filesDir,
		DatabasePath:  env("XFILE_DB", filepath.Join(dataDir, "xfile.db")),
		StaticDir:     env("XFILE_STATIC_DIR", filepath.Join("web", "dist")),
		SiteName:      env("XFILE_SITE_NAME", "XFile"),
		SessionSecret: os.Getenv("XFILE_SESSION_SECRET"),
	}
}

func env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
