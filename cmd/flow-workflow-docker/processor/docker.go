package processor

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

type docker struct {
	client   *dockerclient.Client
	username string
	password string
}

func newDocker(address, username, password string) (*docker, error) {
	clientOpts := []dockerclient.Opt{
		dockerclient.WithHost(address),
	}
	c, err := dockerclient.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("attempting to connect to docker at %s", address)
	ping, err := c.Ping(context.Background())
	if err != nil {
		return nil, err
	}

	logrus.Debugf("docker container runtime ready: apiVersion=%s experimental=%v", ping.APIVersion, ping.Experimental)
	return &docker{
		client:   c,
		username: username,
		password: password,
	}, nil
}

func (c *docker) pullImage(ctx context.Context, name string) error {
	opts := dockertypes.ImagePullOptions{}
	if c.username != "" {
		creds := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.username, c.password)))
		opts.RegistryAuth = creds
	}
	r, err := c.client.ImagePull(ctx, name, dockertypes.ImagePullOptions{})
	if err != nil {
		return err
	}

	if _, err := c.client.ImageLoad(ctx, r, true); err != nil {
		return err
	}

	return nil
}

func (c *docker) createContainer(ctx context.Context, image string, args []string, workingDir string) (string, error) {
	if err := c.pullImage(ctx, image); err != nil {
		return "", err
	}

	containerConfig := &containertypes.Config{
		Image:      image,
		Cmd:        args,
		WorkingDir: workingDir,
	}

	hostConfig := &containertypes.HostConfig{
		NetworkMode: "host",
	}

	var netConfig *networktypes.NetworkingConfig

	logrus.Debugf("container config: %+v", containerConfig)
	resp, err := c.client.ContainerCreate(ctx, containerConfig, hostConfig, netConfig, nil, "")
	if err != nil {
		return "", err
	}
	for _, w := range resp.Warnings {
		logrus.Warnf("warning during container creation for %s: %s", image, w)
	}

	if err := c.client.ContainerStart(ctx, resp.ID, dockertypes.ContainerStartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (c *docker) waitContainer(ctx context.Context, id string) error {
	statusCh, errCh := c.client.ContainerWait(ctx, id, containertypes.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-statusCh:
	}

	return nil
}

func (c *docker) containerLogs(ctx context.Context, id string) (io.Reader, error) {
	r, err := c.client.ContainerLogs(ctx, id, dockertypes.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (c *docker) deleteContainer(ctx context.Context, id string) error {
	if err := c.client.ContainerRemove(ctx, id, dockertypes.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}); err != nil {
		return err
	}

	return nil
}
