package serve

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/luisdavim/synthetic-checker/pkg/checker"
	"github.com/luisdavim/synthetic-checker/pkg/config"
	"github.com/luisdavim/synthetic-checker/pkg/ingresswatcher"
	"github.com/luisdavim/synthetic-checker/pkg/leaderelection"
	"github.com/luisdavim/synthetic-checker/pkg/server"
)

type options struct {
	failStatus     int
	degradedStatus int
	haMode         bool
	watchIngresses bool
	leID           string
	leNs           string
}

func New(cfg *config.Config) *cobra.Command {
	var opts options
	// cmd represents the base command when called without any subcommands
	cmd := &cobra.Command{
		Use:          "serve",
		Aliases:      []string{"run", "start"},
		Short:        "Run as a service",
		Long:         `Run as a service.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			chkr, err := checker.NewFromConfig(*cfg, !opts.haMode)
			if err != nil {
				return err
			}

			var srvCfg server.Config
			if err := server.ReadConfig(&srvCfg); err != nil {
				return fmt.Errorf("error reading server config: %v", err)
			}

			if opts.haMode {
				le, err := leaderelection.NewLeaderElector(opts.leID, opts.leNs)
				if err != nil {
					return err
				}
				go le.RunLeaderElection(context.Background(), func(ctx context.Context) {
					chkr.Run(ctx)
					if opts.watchIngresses {
						ingresswatcher.StartBackground(chkr, fmt.Sprintf(":%d", srvCfg.HTTP.Port+1), fmt.Sprintf(":%d", srvCfg.HTTP.Port+2), false)
					}
					<-ctx.Done() // hold the routine, Run goes into the background
				}, chkr.Syncer(false, srvCfg.HTTP.Port))
			} else {
				chkr.Run(context.Background())
				if opts.watchIngresses {
					ingresswatcher.StartBackground(chkr, fmt.Sprintf(":%d", srvCfg.HTTP.Port+1), fmt.Sprintf(":%d", srvCfg.HTTP.Port+2), false)
				}
			}

			srv := server.New(srvCfg)
			srv.WithShutdownFunc(func() error {
				// ensure the checker routines are stopped
				chkr.Stop()
				time.Sleep(2 * time.Second)
				return nil
			})
			setRoutes(chkr, srv, opts.failStatus, opts.degradedStatus) // Register Routes
			srv.Run()                                                  // Start Server
			return nil
		},
	}

	server.Init(cmd)

	cmd.Flags().IntVarP(&opts.failStatus, "failed-status-code", "F", http.StatusOK, "HTTP status code to return when all checks are failed")
	cmd.Flags().IntVarP(&opts.degradedStatus, "degraded-status-code", "D", http.StatusOK, "HTTP status code to return when check check is failed")
	cmd.Flags().BoolVarP(&opts.haMode, "k8s-leader-election", "", false, "Enable leader election, only works when running in k8s")
	cmd.Flags().StringVarP(&opts.leID, "leader-election-id", "", "", "set the leader election ID, defaults to POD_NAME or hostname")
	cmd.Flags().StringVarP(&opts.leNs, "leader-election-ns", "", "", "set the leader election namespace, defaults to the current namespace")
	cmd.Flags().BoolVarP(&opts.watchIngresses, "watch-ingresses", "w", false, "Automatically setup checks for k8s ingresses, only works when running in k8s")

	return cmd
}
