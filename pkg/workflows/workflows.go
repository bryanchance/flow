package workflows

import (
	"context"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/client"
	"github.com/ehazlett/ttlcache"
	"github.com/sirupsen/logrus"
)

const (
	// time to wait after a failed job to retry the workflow again
	workflowFailedCacheDelay = time.Second * 60

	bufSize = 4096
)

type WorkflowHandler struct {
	cfg                 *Config
	failedWorkflowCache *ttlcache.TTLCache
	stopCh              chan bool
	processor           Processor
}

func NewWorkflowHandler(cfg *Config, p Processor) (*WorkflowHandler, error) {
	// TODO: validate cfg
	failedWorkflowCache, err := ttlcache.NewTTLCache(workflowFailedCacheDelay)
	if err != nil {
		return nil, err
	}

	return &WorkflowHandler{
		cfg:                 cfg,
		failedWorkflowCache: failedWorkflowCache,
		stopCh:              make(chan bool, 1),
		processor:           p,
	}, nil
}

func (h *WorkflowHandler) GetInputWorkflow(ctx context.Context, id, namespace string) (*api.Workflow, error) {
	c, err := h.getClient(client.WithNamespace(namespace))
	if err != nil {
		return nil, err
	}
	defer c.Close()

	logrus.Debugf("getting input workflow: %s ns=%s", id, namespace)

	resp, err := c.GetWorkflow(ctx, &api.GetWorkflowRequest{
		ID: id,
	})
	if err != nil {
		return nil, err
	}

	return resp.Workflow, nil
}

func (h *WorkflowHandler) getClient(clientOpts ...client.ClientOpt) (*client.Client, error) {
	cfg := &flow.Config{
		GRPCAddress:           h.cfg.Address,
		TLSClientCertificate:  h.cfg.TLSCertificate,
		TLSClientKey:          h.cfg.TLSKey,
		TLSInsecureSkipVerify: h.cfg.TLSInsecureSkipVerify,
	}

	opts := []client.ClientOpt{
		client.WithServiceToken(h.cfg.ServiceToken),
	}

	for _, o := range clientOpts {
		opts = append(opts, o)
	}

	return client.NewClient(cfg, opts...)
}
