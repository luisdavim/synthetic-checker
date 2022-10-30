package check

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/chroma/quick"
	"github.com/spf13/cobra"

	"github.com/luisdavim/synthetic-checker/pkg/checker"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

func New(cfg *config.Config) *cobra.Command {
	var (
		prettyJSON bool
		colour     bool
		plain      bool
		retries    int64
	)
	cmd := &cobra.Command{
		Use:          "check",
		Aliases:      []string{"exec"},
		Short:        "Run the checks once and get an exit code",
		Long:         `Run the checks once and get an exit code.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			chkr, err := checker.NewFromConfig(*cfg)
			if err != nil {
				return err
			}

			var anyFailed bool
			retries += 1
			for i := retries; i > 0; i-- {
				chkr.Check(context.Background())
				_, anyFailed = chkr.Summary()
				if !anyFailed {
					break
				}
				t := time.Duration(math.Pow(2, float64(retries-i)))
				if i > 1 {
					fmt.Printf("Error: some checks have failed, retrying in %ds\n", t)
				}
				time.Sleep(t * time.Second)
			}

			status := chkr.GetStatus()

			if plain {
				colour = false
				prettyJSON = false
			}
			var buf strings.Builder
			enc := json.NewEncoder(&buf)
			if prettyJSON {
				enc.SetIndent("", "    ")
			}
			if err := enc.Encode(status); err != nil {
				panic(err)
			}
			if colour {
				err := quick.Highlight(os.Stdout, buf.String(), "json", "terminal", "native")
				if err != nil {
					panic(err)
				}
			} else {
				fmt.Println(buf.String())
			}

			if anyFailed {
				return fmt.Errorf("some checks have failed")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&prettyJSON, "pretty-print", "p", true, "pretty print the check status")
	cmd.Flags().BoolVarP(&colour, "colour", "C", true, "print the check status in colour")
	cmd.Flags().BoolVarP(&plain, "plain", "P", false, "disable both pretty printing and colour")
	cmd.Flags().Int64VarP(&retries, "retries", "r", 0, "number of times to retry on failure")

	return cmd
}
