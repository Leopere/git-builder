package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath   = "/etc/git-builder/config.yaml"
	DefaultPollInterval = 300
	DefaultWorkdir      = "/var/lib/git-builder/repos"
	DefaultSSHKey       = "id_ed25519"
)

type Config struct {
	PollIntervalSeconds int      `yaml:"poll_interval_seconds"`
	Workdir             string   `yaml:"workdir"`
	SSHKey              string   `yaml:"ssh_key"`
	Repos               []Repo   `yaml:"repos"`
}

type Repo struct {
	URL string `yaml:"url"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("GIT_BUILDER_CONFIG")
	}
	if path == "" {
		path = DefaultConfigPath
	}

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
	if c.Workdir == "" {
		c.Workdir = DefaultWorkdir
	}
	if c.SSHKey == "" {
		c.SSHKey = DefaultSSHKey
	}

	return &c, nil
}

func (c *Config) SSHKeyPath() string {
	base := "/etc/git-builder"
	if b := os.Getenv("GIT_BUILDER_KEY_DIR"); b != "" {
		base = b
	}
	return filepath.Join(base, c.SSHKey)
}
