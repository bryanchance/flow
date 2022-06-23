// Copyright 2022 Evan Hazlett
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
		ctx, err := getContext()
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
