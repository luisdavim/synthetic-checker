/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/luisdavim/synthetic-checker/cmd/check"
	"github.com/luisdavim/synthetic-checker/cmd/serve"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	// cmd represents the base command when called without any subcommands
	cmd := &cobra.Command{
		Use:   "synthetic-checker",
		Short: "A tool to run synthetic checks and report their results",
		Long:  `A tool to run synthetic checks and report their results.`,
	}

	cmd.AddCommand(serve.New(cfg), check.New(cfg))

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	var (
		cfgFile string
		cfg     config.Config
	)

	cobra.OnInitialize(func() {
		// Get the configuration for the checks
		var err error
		cfg, err = initConfig(cfgFile)
		if err != nil {
			log.Fatalf("error reading checks config:  %v", err)
		}
	})

	cmd := NewCmd(&cfg)
	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.checks.yaml)")

	if err := cmd.Execute(); err != nil {
		log.Fatalf("error:  %v", err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cfgFile string) (config.Config, error) {
	cfg := config.Config{}
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg, err
		}

		// Search config in home directory with name "checks.yaml".
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/config")
		viper.SetConfigName("checks")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv() // read in environment variables that match the config paths

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	// TODO: should be errors.Is
	// see: https://github.com/spf13/viper/issues/1139
	if errors.As(err, new(viper.ConfigFileNotFoundError)) {
		err = nil
	}
	if err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
		err = viper.Unmarshal(&cfg)
	}

	return cfg, err
}
