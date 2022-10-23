/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/luisdavim/synthetic-checker/pkg/checker"
	"github.com/luisdavim/synthetic-checker/pkg/config"
	"github.com/luisdavim/synthetic-checker/pkg/server"
)

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
			log.Println(err)
			os.Exit(1)
		}
	})

	// cmd represents the base command when called without any subcommands
	cmd := &cobra.Command{
		Use:   "synthetic-checker",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			chkr, err := checker.NewFromConfig(cfg)
			if err != nil {
				return err
			}
			stop := chkr.Run() // Start the checker
			var srvCfg server.Config
			if err := server.ReadConfig(&srvCfg); err != nil {
				log.Fatalf("error reading config: %v", err)
			}
			httpServer := server.New(srvCfg)
			routes := server.Routes{
				"/": {
					Func: func(w http.ResponseWriter, r *http.Request) {
						httpServer.JSONResponse(w, r, chkr.GetStatus(), http.StatusOK)
					},
					Methods: []string{"GET"},
					Name:    "status",
				},
			}
			httpServer.WithShutdownFunc(func() error {
				// ensure the checker routines are stopped
				stop()
				return nil
			})
			httpServer.WithRoutes(routes) // Register Routes
			httpServer.Run()              // Start Server
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.checks.yaml)")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
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
