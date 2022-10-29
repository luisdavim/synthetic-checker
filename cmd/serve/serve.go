package serve

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

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

func New(cfg *config.Config) *cobra.Command {
	var (
		failStatus     int
		degradedStatus int
		haMode         bool
		leID           string
		leNs           string
	)
	// cmd represents the base command when called without any subcommands
	cmd := &cobra.Command{
		Use:          "serve",
		Aliases:      []string{"run", "start"},
		Short:        "Run as a service",
		Long:         `Run as a service.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			chkr, err := checker.NewFromConfig(*cfg)
			if err != nil {
				return err
			}

			var srvCfg server.Config
			if err := server.ReadConfig(&srvCfg); err != nil {
				return fmt.Errorf("error reading server config: %v", err)
			}
			srv := server.New(srvCfg)

			if haMode {
				le, err := leaderelection.NewLeaderElector(leID, leNs)
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

	server.Init(cmd)

	cmd.Flags().IntVarP(&failStatus, "failed-status-code", "F", http.StatusOK, "HTTP status code to return when all checks are failed")
	cmd.Flags().IntVarP(&degradedStatus, "degraded-status-code", "D", http.StatusOK, "HTTP status code to return when check check is failed")
	cmd.Flags().BoolVarP(&haMode, "k8s-leader-election", "", false, "Enable leader election, only works when running in k8s")
	cmd.Flags().StringVarP(&leID, "leader-election-id", "", "", "set the leader election ID, defaults to POD_NAME or hostname")
	cmd.Flags().StringVarP(&leNs, "leader-election-ns", "", "", "set the leader election namespace, defaults to the current namespace")

	return cmd
}