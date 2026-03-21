package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath    = "/etc/git-builder/config.yaml"
	DefaultPollInterval  = 60
	MinPollInterval      = 60
	RecommendedPollInterval = 300
	DefaultWorkdir       = "/var/lib/git-builder/repos"
	DefaultSSHKey        = "id_ed25519"
)

type Config struct {
	PollIntervalSeconds int               `yaml:"poll_interval_seconds"`
	Workdir             string            `yaml:"workdir"`
	SSHKey              string            `yaml:"ssh_key"`
	TokenFromConfig     string            `yaml:"github_token"`
	GhcrToken           string            `yaml:"ghcr_token"`
	GhcrTokenAlt        string            `yaml:"GHCR_TOKEN"`
	GhcrUser            string            `yaml:"ghcr_user"` // optional; used for docker login ghcr.io (default Leopere)
	ScriptEnv           map[string]string `yaml:"script_env"`
	MaxConcurrent       int               `yaml:"max_concurrent"`
	Repos               []Repo            `yaml:"repos"`
	LocalOverrideDir    string            `yaml:"local_override_dir"`
	RunLogPath          string            `yaml:"run_log_path"`
}

type Repo struct {
	URL string `yaml:"url"`
}

// ResolvePath returns the config path when path is empty (default path).
func ResolvePath(path string) string {
	if path != "" {
		return path
	}
	return DefaultConfigPath
}

func Load(path string) (*Config, error) {
	path = ResolvePath(path)

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if c.PollIntervalSeconds <= 0 {
		c.PollIntervalSeconds = DefaultPollInterval
	}
	if c.PollIntervalSeconds < MinPollInterval {
		c.PollIntervalSeconds = MinPollInterval
	}
	if c.Workdir == "" {
		c.Workdir = DefaultWorkdir
	}
	if c.SSHKey == "" {
		c.SSHKey = DefaultSSHKey
	}
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = runtime.NumCPU()
	}
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = 1
	}

	return &c, nil
}

func (c *Config) SSHKeyPath() string {
	base := "/etc/git-builder"
	return filepath.Join(base, c.SSHKey)
}

func (c *Config) GitHubToken() string {
	return c.TokenFromConfig
}

// OverrideScriptDir returns the directory for OWNER-REPO.sh override scripts.
func (c *Config) OverrideScriptDir() string {
	return c.LocalOverrideDir
}
