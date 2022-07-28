package main

import (
	"fmt"
	"os"

	infoapi "github.com/ehazlett/flow/api/services/info/v1"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

var infoCommand = &cli.Command{
	Name:  "info",
	Usage: "flow system info",
	Subcommands: []*cli.Command{
		infoWorkflowsCommand,
		infoVersionCommand,
	},
}

var infoVersionCommand = &cli.Command{
	Name:  "version",
	Usage: "show flow server version",
	Flags: []cli.Flag{},
	Action: func(clix *cli.Context) error {
		ctx, err := getContext(clix)
		if err != nil {
			return err
		}

		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.Version(ctx, &infoapi.VersionRequest{})
		if err != nil {
			return err
		}

		logrus.Debugf("%+v", resp)

		fmt.Printf(`Name: %s
Version: %s
Build: %s
Commit: %s
`,
			resp.Name, resp.Version, resp.Build, resp.Commit)
		return nil
	},
}

var infoWorkflowsCommand = &cli.Command{
	Name:  "workflows",
	Usage: "show number of workflows",
	Flags: []cli.Flag{},
	Action: func(clix *cli.Context) error {
		ctx, err := getContext(clix)
		if err != nil {
			return err
		}

		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.WorkflowInfo(ctx, &infoapi.WorkflowInfoRequest{})
		if err != nil {
			return err
		}

		fmt.Printf("")

		if err := marshaler().Marshal(os.Stdout, resp); err != nil {
			return err
		}
		return nil
	},
}
