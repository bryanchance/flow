package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"github.com/dustin/go-humanize"
	cli "github.com/urfave/cli/v2"
)

var workersCommand = &cli.Command{
	Name:  "workers",
	Usage: "render worker commands",
	Subcommands: []*cli.Command{
		workersListCommand,
		workerStopCommand,
	},
}

var workersListCommand = &cli.Command{
	Name:    "list",
	Usage:   "list available workers",
	Aliases: []string{"ls"},
	Action: func(clix *cli.Context) error {
		ctx := context.Background()
		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.ListWorkers(ctx, &api.ListWorkersRequest{})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
		fmt.Fprintf(w, "NAME\tVERSION\tCPUS\tMEMORY\tGPUS\n")
		for _, wrk := range resp.Workers {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				wrk.Name,
				wrk.Version,
				wrk.CPUs,
				humanize.Bytes(uint64(wrk.MemoryTotal)),
				strings.Join(wrk.GPUs, ", "),
			)
		}
		w.Flush()
		return nil
	},
}

var workerStopCommand = &cli.Command{
	Name:      "stop",
	Usage:     "inform server to stop worker",
	ArgsUsage: "[NAME]",
	Action: func(clix *cli.Context) error {
		name := clix.Args().Get(0)
		if name == "" {
			return fmt.Errorf("worker name must be specified")
		}

		ctx := context.Background()
		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		if _, err := client.ControlWorker(ctx, &api.ControlWorkerRequest{
			WorkerID: name,
			Message: &api.ControlWorkerRequest_Stop{
				Stop: &api.WorkerStop{},
			},
		}); err != nil {
			return err
		}

		return nil
	},
}
