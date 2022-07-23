package processor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/sirupsen/logrus"
)

type webhookConfig struct {
	Target  string
	Headers map[string]string
}

func (p *Processor) processWorkflow(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	logrus.Debugf("processing workflow %s", cfg.Workflow.ID)
	w := cfg.Workflow
	output := &workflows.ProcessorOutput{
		Parameters: map[string]string{},
	}

	startedAt := time.Now()

	// get webhook params
	webhookConfig, err := parseParameters(w)
	if err != nil {
		return nil, err
	}

	if webhookConfig.Target == "" {
		return nil, fmt.Errorf("target must be specified")
	}

	logs := []string{}

	wg := &sync.WaitGroup{}
	for _, iw := range cfg.InputWorkflows {
		if iw.Output != nil {
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()

				logrus.Debugf("sending webhook for %+v", iw)
				buf := &bytes.Buffer{}
				m := &jsonpb.Marshaler{}
				if err := m.Marshal(buf, iw); err != nil {
					logrus.WithError(err).Errorf("error sending webhook for %s to %s", iw.ID, webhookConfig.Target)
					logs = append(logs, fmt.Sprintf("error marshaling request for %s to %s: %s", iw.ID, webhookConfig.Target, err))
					return
				}
				req, err := http.NewRequest("POST", webhookConfig.Target, buf)
				if err != nil {
					logrus.WithError(err).Errorf("error making webhook request for %s to %s", iw.ID, webhookConfig.Target)
					logs = append(logs, fmt.Sprintf("error making request for %s to %s: %s", iw.ID, webhookConfig.Target, err))
					return
				}

				c := &http.Client{}
				for k, v := range webhookConfig.Headers {
					req.Header.Set(k, v)
				}
				logrus.Debugf("request: %+v", req)
				resp, err := c.Do(req)
				if err != nil {
					logrus.WithError(err).Errorf("error sending request for %s to %s", iw.ID, webhookConfig.Target)
					logs = append(logs, fmt.Sprintf("error sending request for %s to %s: %s", iw.ID, webhookConfig.Target, err))
					if resp != nil {
						defer resp.Body.Close()
						o, err := ioutil.ReadAll(resp.Body)
						if err != nil {
							logrus.WithError(err).Errorf("error reading response from %s", webhookConfig.Target)
						}
						logs = append(logs, string(o))
					}
					return
				}
				logs = append(logs, fmt.Sprintf("send webhook for %s to %s successfully", iw.ID, webhookConfig.Target))
			}(wg)
		}
	}

	wg.Wait()

	// workflow output info
	output.Parameters["target"] = webhookConfig.Target

	output.FinishedAt = time.Now()
	output.Duration = output.FinishedAt.Sub(startedAt)
	output.Log = strings.Join(logs, "\n")

	logrus.Infof("workflow complete: %s", w.ID)

	return output, nil
}

func parseParameters(w *api.Workflow) (*webhookConfig, error) {
	cfg := &webhookConfig{
		Headers: map[string]string{},
	}

	for k, v := range w.Parameters {
		switch strings.ToLower(k) {
		case "target":
			cfg.Target = v
		case "headers":
			for _, h := range strings.Split(v, ",") {
				hparts := strings.SplitN(h, ":", 2)
				if len(hparts) != 2 {
					logrus.Warnf("invalid format for header: %s; expected HEADER:VALUE", h)
					continue
				}

				hk, hv := hparts[0], hparts[1]

				cfg.Headers[hk] = hv
			}
		default:
			return nil, fmt.Errorf("unknown parameter specified: %s", k)
		}
	}

	return cfg, nil
}
