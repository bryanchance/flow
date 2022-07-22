package workflows

import (
	"context"
	"sort"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
)

type ByProcessorType []*api.ProcessorInfo

func (s ByProcessorType) Len() int           { return len(s) }
func (s ByProcessorType) Less(i, j int) bool { return s[i].Type < s[j].Type }
func (s ByProcessorType) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *service) ListWorkflowProcessors(ctx context.Context, r *api.ListWorkflowProcessorsRequest) (*api.ListWorkflowProcessorsResponse, error) {
	processors := []*api.ProcessorInfo{}
	for _, p := range s.processors {
		processors = append(processors, p)
	}
	sort.Sort(ByProcessorType(processors))
	return &api.ListWorkflowProcessorsResponse{
		Processors: processors,
	}, nil
}
