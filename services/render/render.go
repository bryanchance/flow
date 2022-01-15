package render

import (
	"bytes"
	"context"
	"io"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"github.com/pkg/errors"
)

func (s *service) GetLatestRender(req *api.GetLatestRenderRequest, stream api.Render_GetLatestRenderServer) error {
	ctx := context.Background()
	data, err := s.ds.GetLatestRender(ctx, req.ID)
	if err != nil {
		return err
	}

	rdr := bytes.NewBuffer(data)
	buf := make([]byte, 4096)

	for {
		n, err := rdr.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "error reading file chunk")
		}

		resp := &api.GetLatestRenderResponse{
			Data: buf[:n],
		}

		if err := stream.Send(resp); err != nil {
			return errors.Wrap(err, "error sending file chunk")
		}
	}

	return nil
}
