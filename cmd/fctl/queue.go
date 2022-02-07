package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
	"github.com/pkg/errors"
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
		&cli.StringFlag{
			Name:    "priority",
			Aliases: []string{"p"},
			Usage:   "set render priority (urgent, normal, low)",
			Value:   "normal",
		},
		&cli.StringFlag{
			Name:  "render-engine",
			Usage: "set render engine",
			Value: "CYCLES",
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

	ctx, err := getContext()
	if err != nil {
		return err
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

	client, err := getClient(clix)
	if err != nil {
		return err
	}
	defer client.Close()

	stream, err := client.QueueJob(ctx)
	if err != nil {
		return err
	}

	renderEngine, err := getRenderEngine(clix.String("render-engine"))
	if err != nil {
		return err
	}

	jobPriority, err := getJobPriority(clix.String("priority"))
	if err != nil {
		return err
	}

	if err := stream.Send(&api.QueueJobRequest{
		Data: &api.QueueJobRequest_Request{
			Request: &api.JobRequest{
				Name:             name,
				ResolutionX:      int64(clix.Int("resolution-x")),
				ResolutionY:      int64(clix.Int("resolution-y")),
				ResolutionScale:  int64(clix.Int("resolution-scale")),
				RenderStartFrame: int64(clix.Int("render-start-frame")),
				RenderEndFrame:   int64(clix.Int("render-end-frame")),
				RenderEngine:     renderEngine,
				RenderSamples:    int64(clix.Int("render-samples")),
				RenderUseGPU:     clix.Bool("render-use-gpu"),
				RenderPriority:   int64(priority),
				CPU:              int64(clix.Int("cpu")),
				Memory:           int64(clix.Int("memory")),
				RenderSlices:     int64(clix.Int("render-slices")),
				ContentType:      contentType,
				Priority:         jobPriority,
			},
		},
	}); err != nil {
		return err
	}
	logrus.Debugf("sending job request for %s", name)

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

func getRenderEngine(v string) (api.RenderEngine, error) {
	switch strings.ToLower(v) {
	case "cycles":
		return api.RenderEngine_CYCLES, nil
	case "eevee":
		return api.RenderEngine_BLENDER_EEVEE, nil
	}
	return api.RenderEngine_UNKNOWN, fmt.Errorf("unknown render engine %s", v)
}

func getJobPriority(v string) (api.JobPriority, error) {
	switch strings.ToLower(v) {
	case "urgent":
		return api.JobPriority_URGENT, nil
	case "normal":
		return api.JobPriority_NORMAL, nil
	case "low":
		return api.JobPriority_LOW, nil
	}
	return api.JobPriority_NORMAL, fmt.Errorf("unknown priority %s", v)
}
