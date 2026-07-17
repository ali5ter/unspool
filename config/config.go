// Package config loads unspool settings from ~/.config/unspool/config.toml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Queue holds Queue-mirroring settings (PRD §5.4).
type Queue struct {
	Mirror           bool   `mapstructure:"mirror"`
	MirrorPlaylistID string `mapstructure:"mirror_playlist_id"`
}

// Recommendations holds synthesised-recommendations settings (PRD §5.8).
type Recommendations struct {
	Enabled bool `mapstructure:"enabled"`
}

// Filters holds feed-filtering settings (PRD §5.1, §5.2).
type Filters struct {
	HideShorts        bool    `mapstructure:"hide_shorts"`
	AIScoreThreshold  float64 `mapstructure:"ai_score_threshold"`
	AIAutohide        bool    `mapstructure:"ai_autohide"`
	ShowSyntheticFlag bool    `mapstructure:"show_synthetic_flag"`
}

// Classifier holds the model-agnostic AI-slop inspection shell-out hooks (PRD §5.2 tiers 1-2).
type Classifier struct {
	TranscriptCommand      string `mapstructure:"transcript_command"`
	InspectCommand         string `mapstructure:"inspect_command"`
	AutoInspectNewChannels bool   `mapstructure:"auto_inspect_new_channels"`
	CacheVerdicts          bool   `mapstructure:"cache_verdicts"`
}

// Config holds user-configurable settings.
type Config struct {
	StoreDir              string          `mapstructure:"store_dir"`
	OAuthClientSecretFile string          `mapstructure:"oauth_client_secret_file"`
	MaxResolution         int             `mapstructure:"max_resolution"`
	AudioOnlyDefault      bool            `mapstructure:"audio_only_default"`
	PlaybackDetached      bool            `mapstructure:"playback_detached"`
	Thumbnails            string          `mapstructure:"thumbnails"`
	Theme                 string          `mapstructure:"theme"`
	ViewMode              string          `mapstructure:"view_mode"`
	DeArrow               bool            `mapstructure:"dearrow"`
	CookiesFromBrowser    string          `mapstructure:"cookies_from_browser"`
	SponsorBlock          []string        `mapstructure:"sponsorblock"`
	Queue                 Queue           `mapstructure:"queue"`
	Recommendations       Recommendations `mapstructure:"recommendations"`
	Filters               Filters         `mapstructure:"filters"`
	Classifier            Classifier      `mapstructure:"classifier"`
}

// DefaultStoreDir returns the platform default location for the local store:
// alongside the config directory under a "store" subdirectory.
// On macOS: ~/Library/Application Support/unspool/store
// On Linux: ~/.config/unspool/store
func DefaultStoreDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".config", "unspool", "store")
		}
		return ""
	}
	return filepath.Join(dir, "unspool", "store")
}

// DefaultClientSecretFile returns the platform default location for the
// downloaded Google Cloud OAuth client secret JSON (see docs/SETUP.md).
func DefaultClientSecretFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".config", "unspool", "client_secret.json")
		}
		return ""
	}
	return filepath.Join(dir, "unspool", "client_secret.json")
}

// Load reads ~/.config/unspool/config.toml, applying defaults where absent.
func Load() (*Config, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return defaults(), fmt.Errorf("locate config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(filepath.Join(dir, "unspool"))
	applyDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return defaults(), fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return defaults(), fmt.Errorf("parse config: %w", err)
	}
	if cfg.StoreDir == "" {
		cfg.StoreDir = DefaultStoreDir()
	}
	if cfg.OAuthClientSecretFile == "" {
		cfg.OAuthClientSecretFile = DefaultClientSecretFile()
	}
	if home, herr := os.UserHomeDir(); herr == nil {
		cfg.StoreDir = expandHome(cfg.StoreDir, home)
		cfg.OAuthClientSecretFile = expandHome(cfg.OAuthClientSecretFile, home)
	}
	return &cfg, nil
}

func applyDefaults(v *viper.Viper) {
	v.SetDefault("max_resolution", 1080)
	v.SetDefault("audio_only_default", false)
	v.SetDefault("playback_detached", true)
	v.SetDefault("thumbnails", "auto")
	v.SetDefault("theme", "warm-dark")
	v.SetDefault("view_mode", "rows")
	v.SetDefault("dearrow", true)
	v.SetDefault("cookies_from_browser", "")
	v.SetDefault("sponsorblock", []string{"sponsor", "selfpromo", "interaction"})
	v.SetDefault("queue.mirror", true)
	v.SetDefault("queue.mirror_playlist_id", "")
	v.SetDefault("recommendations.enabled", true)
	v.SetDefault("filters.hide_shorts", true)
	v.SetDefault("filters.ai_score_threshold", 0.7)
	v.SetDefault("filters.ai_autohide", false)
	v.SetDefault("filters.show_synthetic_flag", true)
	v.SetDefault("classifier.transcript_command", "")
	v.SetDefault("classifier.inspect_command", "")
	v.SetDefault("classifier.auto_inspect_new_channels", false)
	v.SetDefault("classifier.cache_verdicts", true)
}

// expandHome replaces a leading "~/" with the user's home directory.
func expandHome(path, home string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		return filepath.Join(home, path[2:])
	}
	return path
}

func defaults() *Config {
	v := viper.New()
	applyDefaults(v)
	var cfg Config
	_ = v.Unmarshal(&cfg)
	cfg.StoreDir = DefaultStoreDir()
	cfg.OAuthClientSecretFile = DefaultClientSecretFile()
	return &cfg
}
