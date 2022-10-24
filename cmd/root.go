/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/luisdavim/synthetic-checker/pkg/checker"
	"github.com/luisdavim/synthetic-checker/pkg/config"
	"github.com/luisdavim/synthetic-checker/pkg/leaderelection"
	"github.com/luisdavim/synthetic-checker/pkg/server"
)

func statusHandler(chkr *checker.CheckRunner, srv *server.Server, failStatus, degradedStatus int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusCode := http.StatusOK
		checkStatus := chkr.GetStatus()
		if failStatus != http.StatusOK || degradedStatus != http.StatusOK {
			allFailed := true
			for _, res := range checkStatus {
				if !res.OK {
					statusCode = degradedStatus
				} else {
					allFailed = false
				}
			}
			if allFailed {
				statusCode = failStatus
			}
		}
		srv.JSONResponse(w, r, checkStatus, statusCode)
	}
}

func newCmd(cfg *config.Config, srvCfg *server.Config) *cobra.Command {
	var (
		failStatus     int
		degradedStatus int
		haMode         bool
	)
	// cmd represents the base command when called without any subcommands
	cmd := &cobra.Command{
		Use:          "synthetic-checker",
		Short:        "A service to run synthetic checks and report their results",
		Long:         `A service to run synthetic checks and report their results.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			chkr, err := checker.NewFromConfig(*cfg)
			if err != nil {
				return err
			}
			srv := server.New(*srvCfg)

			if haMode {
				le, err := leaderelection.NewLeaderElector("", "")
				if err != nil {
					return err
				}
				go le.RunLeaderElection(context.Background(), func(ctx context.Context) {
					chkr.RunWithContext(ctx)
					<-ctx.Done() // hold the routine, RunWithContext goes into the background
				}, chkr.Sync)
			} else {
				stop := chkr.Run() // Start the checker
				srv.WithShutdownFunc(func() error {
					// ensure the checker routines are stopped
					stop()
					return nil
				})
			}

			routes := server.Routes{
				"/": {
					Func:    statusHandler(chkr, srv, failStatus, degradedStatus),
					Methods: []string{"GET"},
					Name:    "status",
				},
			}
			srv.WithRoutes(routes) // Register Routes
			srv.Run()              // Start Server
			return nil
		},
	}

	cmd.Flags().IntVarP(&failStatus, "failed-status-code", "F", http.StatusOK, "HTTP status code to return when all checks are failed")
	cmd.Flags().IntVarP(&degradedStatus, "degraded-status-code", "D", http.StatusOK, "HTTP status code to return when check check is failed")
	cmd.Flags().BoolVarP(&haMode, "k8s-leader-election", "", false, "Enable leader election, only works when running in k8s")

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	var (
		cfgFile string
		cfg     config.Config
		srvCfg  server.Config
	)

	cobra.OnInitialize(func() {
		// Get the configuration for the checks
		var err error
		cfg, err = initConfig(cfgFile)
		if err != nil {
			log.Fatalf("error reading checks config:  %v", err)
		}
		if err := server.ReadConfig(&srvCfg); err != nil {
			log.Fatalf("error reading server config: %v", err)
		}
	})

	cmd := newCmd(&cfg, &srvCfg)
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
