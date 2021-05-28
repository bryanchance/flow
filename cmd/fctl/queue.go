package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

var queueJobCommand = &cli.Command{
	Name:  "queue",
	Usage: "queue render job",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Usage: "job name",
		},
		&cli.StringFlag{
			Name:    "project-file",
			Aliases: []string{"f"},
			Usage:   "path to project file",
		},
		&cli.IntFlag{
			Name:    "resolution-x",
			Aliases: []string{"x"},
			Usage:   "render resolution (x)",
			Value:   1920,
		},
		&cli.IntFlag{
			Name:    "resolution-y",
			Aliases: []string{"y"},
			Usage:   "render resolution (y)",
			Value:   1080,
		},
		&cli.IntFlag{
			Name:  "resolution-scale",
			Usage: "render resolution scale",
			Value: 100,
		},
		&cli.IntFlag{
			Name:    "render-start-frame",
			Aliases: []string{"s"},
			Usage:   "render start frame",
			Value:   1,
		},
		&cli.IntFlag{
			Name:    "render-end-frame",
			Aliases: []string{"e"},
			Usage:   "render end frame",
		},
		&cli.IntFlag{
			Name:  "render-samples",
			Usage: "render samples",
			Value: 100,
		},
		&cli.BoolFlag{
			Name:    "render-use-gpu",
			Aliases: []string{"g"},
			Usage:   "use gpu for rendering",
		},
		&cli.IntFlag{
			Name:    "render-priority",
			Aliases: []string{"p"},
			Usage:   "set render priority (0-100)",
			Value:   50,
		},
		&cli.IntFlag{
			Name:  "cpu",
			Usage: "specify cpu (in Mhz) for each task in the render job",
		},
		&cli.IntFlag{
			Name:  "memory",
			Usage: "specify memory (in MB) for each task in the render job",
		},
		&cli.IntFlag{
			Name:  "render-slices",
			Usage: "number of slices to subdivide a frame for rendering",
		},
	},
	Action: queueAction,
}

func queueAction(clix *cli.Context) error {
	name := clix.String("name")
	if name == "" {
		return fmt.Errorf("job name must be specified")
	}

	priority := clix.Int("render-priority")
	if priority < 0 || priority > 100 {
		return fmt.Errorf("priority must be greater than 0 and less than 100")
	}

	jobFile := clix.String("project-file")
	logrus.Debugf("using project %s", jobFile)

	f, err := os.Open(jobFile)
	if err != nil {
		return errors.Wrapf(err, "error opening project file %s", jobFile)
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

	ctx := context.Background()
	client, err := getClient(clix)
	if err != nil {
		return err
	}

	stream, err := client.QueueJob(ctx)
	if err != nil {
		return err
	}

	id := uuid.NewV4()
	if err := stream.Send(&api.QueueJobRequest{
		Data: &api.QueueJobRequest_Request{
			Request: &api.JobRequest{
				UUID:             id.String(),
				Name:             name,
				ResolutionX:      int64(clix.Int("resolution-x")),
				ResolutionY:      int64(clix.Int("resolution-y")),
				ResolutionScale:  int64(clix.Int("resolution-scale")),
				RenderStartFrame: int64(clix.Int("render-start-frame")),
				RenderEndFrame:   int64(clix.Int("render-end-frame")),
				RenderSamples:    int64(clix.Int("render-samples")),
				RenderUseGPU:     clix.Bool("render-use-gpu"),
				RenderPriority:   int64(priority),
				CPU:              int64(clix.Int("cpu")),
				Memory:           int64(clix.Int("memory")),
				RenderSlices:     int64(clix.Int("render-slices")),
				ContentType:      contentType,
			},
		},
	}); err != nil {
		return err
	}
	logrus.Debugf("sending job request for %s", id.String())

	rdr := bufio.NewReader(f)
	buf := make([]byte, 4096)

	chunk := 0
	for {
		n, err := rdr.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "error reading file chunk")
		}

		req := &api.QueueJobRequest{
			Data: &api.QueueJobRequest_ChunkData{
				ChunkData: buf[:n],
			},
		}

		if err := stream.Send(req); err != nil {
			return errors.Wrap(err, "error sending file chunk")
		}

		chunk += 1
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		return errors.Wrap(err, "error receiving response from server")
	}

	logrus.Printf("queued job %s", res.GetUUID())
	return nil
}
