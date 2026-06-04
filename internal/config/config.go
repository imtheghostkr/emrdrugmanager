package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Paths struct {
	ConfigPath    string
	AppDataDir    string
	LocalDataDir  string
	CredentialDir string
	LogDir        string
}

type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	EMR      EMRConfig      `yaml:"emr" json:"emr"`
	Database DatabaseConfig `yaml:"database" json:"database"`
}

type ServerConfig struct {
	Host                string   `yaml:"host" json:"host"`
	Port                int      `yaml:"port" json:"port"`
	AccessTokenRequired bool     `yaml:"access_token_required" json:"access_token_required"`
	AccessTokenHash     string   `yaml:"access_token_hash" json:"-"`
	AllowedCIDRs        []string `yaml:"allowed_cidrs" json:"allowed_cidrs"`
}

type EMRConfig struct {
	Adapter string `yaml:"adapter" json:"adapter"`
}

type DatabaseConfig struct {
	Host        string `yaml:"host" json:"host"`
	Port        int    `yaml:"port" json:"port"`
	Name        string `yaml:"name" json:"name"`
	User        string `yaml:"user" json:"user"`
	PasswordRef string `yaml:"password_ref" json:"-"`
	SSLMode     string `yaml:"sslmode" json:"sslmode"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 3987,
		},
		EMR: EMRConfig{Adapter: "eghis"},
		Database: DatabaseConfig{
			Host:    "127.0.0.1",
			Port:    5432,
			Name:    "postgres",
			SSLMode: "disable",
		},
	}
}

func ResolvePaths(configPath string) (Paths, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	localData, err := os.UserCacheDir()
	if err != nil {
		localData = appData
	}
	appDir := filepath.Join(appData, "OpenDrugBridge")
	localDir := filepath.Join(localData, "OpenDrugBridge")
	if configPath == "" {
		configPath = filepath.Join(appDir, "config.yaml")
	}
	return Paths{
		ConfigPath:    configPath,
		AppDataDir:    appDir,
		LocalDataDir:  localDir,
		CredentialDir: filepath.Join(appDir, "credentials"),
		LogDir:        filepath.Join(localDir, "logs"),
	}, nil
}

func LoadOrDefault(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	ApplyDefaults(&cfg)
	return cfg, nil
}

func ApplyDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 3987
	}
	if cfg.EMR.Adapter == "" {
		cfg.EMR.Adapter = "eghis"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
}

func Save(path string, cfg Config) error {
	ApplyDefaults(&cfg)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (d DatabaseConfig) DSN(password string) string {
	sslmode := d.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s", d.Host, d.Port, d.Name, d.User, password, sslmode)
}

func (d DatabaseConfig) RedactedSummary() string {
	if d.Host == "" || d.Name == "" || d.User == "" {
		return "not configured"
	}
	return fmt.Sprintf("%s:%d/%s as %s", d.Host, d.Port, d.Name, d.User)
}
