package main

import (
	"encoding/json"
	"os"

	infoapi "github.com/ehazlett/flow/api/services/info/v1"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

var infoCommand = &cli.Command{
	Name:  "info",
	Usage: "flow server info",
	Subcommands: []*cli.Command{
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

		if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
			return err
		}
		return nil
	},
}
