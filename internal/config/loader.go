package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const (
	defaultConfigName = "gateway"
	defaultConfigDir  = "configs"
	envPrefix         = "GATEWAY"
)

// Load reads configuration from a YAML file and merges environment variable
// overrides. Environment variables are matched to config keys using the
// GATEWAY_ prefix with dots and hyphens replaced by underscores.
//
// Priority (highest to lowest): env vars > config file > defaults.
//
// Examples:
//
//	GATEWAY_SERVER_PORT=9090            overrides server.port
//	GATEWAY_SECURITY_JWT_HMAC_SECRET=x  overrides security.jwt.hmac_secret
//	GATEWAY_CONFIG=/etc/gw/custom.yaml  points to an alternate config file
func Load(path string) (Config, error) {
	v := viper.New()

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName(defaultConfigName)
		v.SetConfigType("yaml")
		v.AddConfigPath(defaultConfigDir)
		v.AddConfigPath(".")
	}

	// 12-factor: env vars override config file values.
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
