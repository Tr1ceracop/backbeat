package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Tempo    TempoConfig    `toml:"tempo"`
	Jira     JiraConfig     `toml:"jira"`
	Tracking TrackingConfig `toml:"tracking"`
}

type TempoConfig struct {
	APIToken  string `toml:"api_token"`
	BaseURL   string `toml:"base_url"`
	AccountID string `toml:"account_id"`
}

type JiraConfig struct {
	BaseURL  string `toml:"base_url"`
	APIToken string `toml:"api_token"`
	Email    string `toml:"email"`
}

type TrackingConfig struct {
	IdleThreshold Duration `toml:"idle_threshold"`
	PollInterval  Duration `toml:"poll_interval"`
	MinSession    Duration `toml:"min_session"`
	IssueKey      string   `toml:"issue_key"`
}

// Duration wraps time.Duration for TOML parsing.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

func DefaultConfig() *Config {
	return &Config{
		Tempo: TempoConfig{
			BaseURL: "https://api.tempo.io",
		},
		Tracking: TrackingConfig{
			IdleThreshold: Duration{5 * time.Minute},
			PollInterval:  Duration{30 * time.Second},
			MinSession:    Duration{1 * time.Minute},
		},
	}
}

func DefaultPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "backbeat", "config.toml")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}

	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

func (c *Config) SetIssueKey(key string) error {
	c.Tracking.IssueKey = key
	return c.Save("")
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultPath()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(c)
}
