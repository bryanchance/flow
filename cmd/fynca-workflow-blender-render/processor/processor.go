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
package processor

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/workflows/v1"
	"github.com/fynca/fynca/client"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Config struct {
	ID                    string
	Type                  string
	Address               string
	TLSCertificate        string
	TLSKey                string
	TLSInsecureSkipVerify bool
	MaxWorkflows          uint64
	ServiceToken          string
}

type Processor struct {
	blenderPath string
	config      *fynca.Config
	cfg         *Config
}

func NewProcessor(blenderPath string, cfg *fynca.Config, pCfg *Config) (*Processor, error) {
	if blenderPath == "" {
		// resolve in path
		bp, err := exec.LookPath(blenderExecutableName)
		if err != nil {
			return nil, err
		}
		blenderPath = bp
	}
	if _, err := os.Stat(blenderPath); err != nil {
		return nil, errors.Wrapf(err, "cannot find blender at path %q", blenderPath)
	}

	return &Processor{
		blenderPath: blenderPath,
		config:      cfg,
		cfg:         pCfg,
	}, nil
}

func (p *Processor) Start(ctx context.Context) error {
	c, err := p.getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	stream, err := c.SubscribeWorkflowEvents(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&api.SubscribeWorkflowEventsRequest{
		Request: &api.SubscribeWorkflowEventsRequest_Info{
			Info: &api.SubscriberInfo{
				ID:           p.cfg.ID,
				Type:         p.cfg.Type,
				MaxWorkflows: p.cfg.MaxWorkflows,
			},
		},
	}); err != nil {
		return err
	}

	for {
		evt, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		logrus.Debugf("workflow event: %+v", evt)
		switch v := evt.Event.(type) {
		case *api.WorkflowEvent_Workflow:
			output, err := p.renderWorkflow(ctx, v.Workflow)
			if err != nil {
				return err
			}

			if err := stream.Send(&api.SubscribeWorkflowEventsRequest{
				Request: &api.SubscribeWorkflowEventsRequest_Output{
					Output: output,
				},
			}); err != nil {
				return err
			}
		case *api.WorkflowEvent_Close:
			logrus.Debug("close event received from server")
			break
		}
	}

	return nil
}

func (p *Processor) Process(ctx context.Context, w *api.Workflow) (*api.WorkflowOutput, error) {
	logrus.Debugf("processing workflow %s (%s)", w.Name, w.ID)
	output, err := p.renderWorkflow(ctx, w)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (p *Processor) getClient() (*client.Client, error) {
	cfg := &fynca.Config{
		GRPCAddress:           p.cfg.Address,
		TLSClientCertificate:  p.cfg.TLSCertificate,
		TLSClientKey:          p.cfg.TLSKey,
		TLSInsecureSkipVerify: p.cfg.TLSInsecureSkipVerify,
	}

	return client.NewClient(cfg, client.WithServiceToken(p.cfg.ServiceToken))
}
