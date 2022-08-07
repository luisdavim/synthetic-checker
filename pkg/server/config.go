package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

// Auth holds the BasicAuth credentials for the HTTP server
type Auth struct {
	User string `mapstructure:"user,omitempty"`
	Pass string `mapstructure:"pass,omitempty"`
}

// HTTP holds the configuration for the HTTP server
type HTTP struct {
	Auth         Auth    `mapstructure:"auth,omitempty"`
	OperatorAuth string  `mapstructure:"operatorAuth,omitempty"`
	Port         int     `mapstructure:"port,omitempty"`
	SecurePort   int     `mapstructure:"securePort,omitempty"`
	CertFile     string  `mapstructure:"certFile,omitempty"`
	KeyFile      string  `mapstructure:"keyFile,omitempty"`
	RequestLimit float64 `mapstructure:"requestLimit,omitempty"`
	PrettyJSON   bool    `mapstructure:"prettyJSON,omitempty"`
}

// Config holds the full application configuration
type Config struct {
	HTTP  HTTP `mapstructure:"http,omitempty"`
	Debug bool `mapstructure:"debug,omitempty"`
	// If you read the documentation for ScrictSlashes
	// it lets you know that it generates a 301 redirect and converts all requests to GET requests.
	// So a POST request to /route will turn into a GET to /route/ and that will cause problems.
	// So instead you can set StripSlashes that will strip the trailing slashes before routing.
	StripSlashes bool `mapstructure:"stripSlashes,omitempty"`
}

func ReadConfig(config *Config) error {
	notfound := viper.ConfigFileNotFoundError{}
	// Read config from file into the Config struct
	if err := viper.ReadInConfig(); err != nil {
		if !errors.As(err, &notfound) {
			return err
		}
	}

	if viper.Get("debug").(bool) {
		viper.Debug()
	}

	if err := viper.Unmarshal(config); err != nil {
		return err
	}

	return nil
}

func LoadEnvConfig(rootDir string) error {
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".env") {
			err = gotenv.Load(path)
			if err != nil {
				return fmt.Errorf("error loading .env file %s: %w", path, err)
			}
		}
		return nil
	})
	return err
}
