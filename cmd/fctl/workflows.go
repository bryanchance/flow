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
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

const (
	bufSize = 4096
)

var workflowsCommand = &cli.Command{
	Name:  "workflows",
	Usage: "manage workflows",
	Subcommands: []*cli.Command{
		workflowsQueueCommand,
		workflowsProcessorsCommand,
		workflowsListCommand,
		workflowsInfoCommand,
		workflowsDeleteCommand,
	},
}

var workflowsListCommand = &cli.Command{
	Name:    "list",
	Usage:   "list available workflows",
	Aliases: []string{"ls"},
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

		resp, err := client.ListWorkflows(ctx, &api.ListWorkflowsRequest{})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
		fmt.Fprintf(w, "ID\tNAME\tTYPE\tSTATUS\tCREATED\tPRIORITY\tDURATION\n")
		for _, wf := range resp.Workflows {
			duration := ""
			if wf.Output != nil {
				duration = wf.Output.Duration.String()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				wf.ID,
				wf.Name,
				wf.Type,
				wf.Status.String(),
				humanize.Time(wf.CreatedAt),
				wf.Priority,
				duration,
			)
		}
		w.Flush()
		return nil
	},
}

var workflowsProcessorsCommand = &cli.Command{
	Name:  "processors",
	Usage: "list available workflow processors",
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

		resp, err := client.ListWorkflowProcessors(ctx, &api.ListWorkflowProcessorsRequest{})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
		fmt.Fprintf(w, "TYPE\tID\tSTARTED\tSCOPE\tCPUS\tMEMORY\tGPUS\n")
		for _, p := range resp.Processors {
			scope := "global"
			if v, ok := p.Scope.Scope.(*api.ProcessorScope_Namespace); ok {
				scope = v.Namespace
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
				p.Type,
				p.ID,
				humanize.Time(p.StartedAt),
				scope,
				p.CPUs,
				humanize.Bytes(uint64(p.MemoryTotal)),
				strings.Join(p.GPUs, ","),
			)
		}
		w.Flush()
		return nil
	},
}

var workflowsQueueCommand = &cli.Command{
	Name:  "queue",
	Usage: "queue workflow",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "name",
			Aliases: []string{"n"},
			Usage:   "workflow instance name",
		},
		&cli.StringFlag{
			Name:    "type",
			Aliases: []string{"t"},
			Usage:   "workflow type",
		},
		&cli.StringFlag{
			Name:    "input-file",
			Aliases: []string{"f"},
			Usage:   "file path to use as workflow input",
		},
		&cli.StringSliceFlag{
			Name:    "input-workflow-id",
			Aliases: []string{"w"},
			Usage:   "output from another workflow to use as input (can be multiple)",
		},
		&cli.StringFlag{
			Name:  "priority",
			Usage: "workflow priority (low, normal, urgent)",
			Value: "normal",
		},
		&cli.StringSliceFlag{
			Name:    "parameter",
			Aliases: []string{"p"},
			Usage:   "specify workflow parameters (KEY=VAL)",
		},
		&cli.StringSliceFlag{
			Name:    "label",
			Aliases: []string{"l"},
			Usage:   "workflow labels (KEY=VAL)",
		},
	},
	Action: func(clix *cli.Context) error {
		workflowName := clix.String("name")
		if workflowName == "" {
			return fmt.Errorf("workflow name must be specified")
		}
		workflowType := clix.String("type")
		if workflowType == "" {
			return fmt.Errorf("workflow type must be specified")
		}
		inputFilePath := clix.String("input-file")
		inputWorkflowIDs := clix.StringSlice("input-workflow-id")
		// validate input type; only file or workflow allowed -- not both
		if inputFilePath != "" && len(inputWorkflowIDs) > 0 {
			return fmt.Errorf("only one input file or workflow IDs can be specified")
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		params, err := parseKeyValues(clix.StringSlice("parameter"))
		if err != nil {
			return err
		}

		labels, err := parseKeyValues(clix.StringSlice("label"))
		if err != nil {
			return err
		}

		stream, err := client.QueueWorkflow(ctx)
		if err != nil {
			return err
		}

		priority, err := getWorkflowPriority(clix.String("priority"))
		if err != nil {
			return err
		}

		workflowRequest := &api.WorkflowRequest{
			Name:       workflowName,
			Type:       workflowType,
			Parameters: params,
			Labels:     labels,
			Priority:   priority,
		}

		var inputFile *os.File

		if inputFilePath != "" {
			logrus.Debugf("using workflow input %s", inputFilePath)

			f, err := os.Open(inputFilePath)
			if err != nil {
				return errors.Wrapf(err, "error opening workflow input file %s", inputFilePath)
			}
			defer f.Close()

			cBuf := make([]byte, 512)
			if _, err := f.Read(cBuf); err != nil {
				return err
			}
			contentType := http.DetectContentType(cBuf)
			if _, err := f.Seek(0, 0); err != nil {
				return err
			}
			logrus.Debugf("detected content type: %s", contentType)
			inputFile = f
			workflowRequest.Input = &api.WorkflowRequest_File{
				File: &api.WorkflowInputFile{
					Filename:    filepath.Base(inputFilePath),
					ContentType: contentType,
				},
			}
		}

		if len(inputWorkflowIDs) > 0 {
			workflowInputs := []*api.WorkflowInputWorkflow{}
			for _, inputID := range inputWorkflowIDs {
				workflowInputs = append(workflowInputs, &api.WorkflowInputWorkflow{
					ID: inputID,
				})
			}
			workflowRequest.Input = &api.WorkflowRequest_Workflows{
				Workflows: &api.WorkflowInputWorkflows{
					WorkflowInputs: workflowInputs,
				},
			}
		}

		if err := stream.Send(&api.QueueWorkflowRequest{
			Data: &api.QueueWorkflowRequest_Request{
				Request: workflowRequest,
			},
		}); err != nil {
			return err
		}

		// TODO: stream local data
		if inputFile != nil {
			rdr := bufio.NewReader(inputFile)
			buf := make([]byte, bufSize)

			chunk := 0
			for {
				n, err := rdr.Read(buf)
				if err == io.EOF {
					break
				}
				if err != nil {
					return errors.Wrap(err, "error reading file chunk")
				}

				req := &api.QueueWorkflowRequest{
					Data: &api.QueueWorkflowRequest_ChunkData{
						ChunkData: buf[:n],
					},
				}

				if err := stream.Send(req); err != nil {
					return errors.Wrap(err, "error sending file chunk")
				}

				chunk += 1
			}

			logrus.Debugf("uploaded %d bytes", chunk*bufSize)
		}

		resp, err := stream.CloseAndRecv()
		if err != nil {
			return errors.Wrap(err, "error receiving response from server")
		}

		fmt.Printf("%+v\n", resp.ID)

		return nil
	},
}

var workflowsInfoCommand = &cli.Command{
	Name:      "info",
	Usage:     "get workflow info",
	ArgsUsage: "[ID]",
	Flags:     []cli.Flag{},
	Action: func(clix *cli.Context) error {
		id := clix.Args().Get(0)
		if id == "" {
			cli.ShowSubcommandHelp(clix)
			return fmt.Errorf("workflow ID must be specified")
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.GetWorkflow(ctx, &api.GetWorkflowRequest{
			ID: id,
		})
		if err != nil {
			return err
		}

		if err := marshaler().Marshal(os.Stdout, resp.Workflow); err != nil {
			return err
		}

		return nil
	},
}

var workflowsDeleteCommand = &cli.Command{
	Name:      "delete",
	Usage:     "delete workflow",
	ArgsUsage: "[ID]",
	Flags:     []cli.Flag{},
	Action: func(clix *cli.Context) error {
		id := clix.Args().Get(0)
		if id == "" {
			cli.ShowSubcommandHelp(clix)
			return fmt.Errorf("workflow ID must be specified")
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		if _, err := client.DeleteWorkflow(ctx, &api.DeleteWorkflowRequest{
			ID: id,
		}); err != nil {
			return err
		}

		return nil
	},
}

func getWorkflowPriority(p string) (api.WorkflowPriority, error) {
	switch strings.ToLower(p) {
	case "low":
		return api.WorkflowPriority_LOW, nil
	case "normal":
		return api.WorkflowPriority_NORMAL, nil
	case "urgent":
		return api.WorkflowPriority_URGENT, nil
	}

	return api.WorkflowPriority_UNKNOWN, fmt.Errorf("unknown priority specified: %s (use \"low\", \"normal\", or \"urgent\")", p)
}

func parseKeyValues(values []string) (map[string]string, error) {
	vals := map[string]string{}
	for _, p := range values {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format for parameter %s: expect KEY=VAL", p)
		}
		k, v := parts[0], parts[1]
		vals[k] = v
	}

	return vals, nil
}