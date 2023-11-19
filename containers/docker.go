package containers

import (
	"context"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

type Container struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Ip   string `json:"ip"`
}

type ContainerNetwork struct {
	Id string `json:"id"`
}

type ContainerConfig struct {
	NetworkName string
	Name        string
	Image       string
	Cmd         []string
	Env         []string
	User        string
	VolumeBinds []string
	TmpFs       []string
	Privileged  bool
	Ports       []string
}

type Volume struct {
	Src     string
	Dest    string
	IsTmpFs bool
}

type ContainerClient interface {
	CreateNetwork(ctx context.Context, name string) (ContainerNetwork, error)
	RunContainer(ctx context.Context, config ContainerConfig) (*Container, error)
	Logs(ctx context.Context, containerName string, wait bool) (io.ReadCloser, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string) error
}

type DockerClient struct {
	cli *client.Client
}

func NewDockerClient() (ContainerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return nil, err
	}

	return &DockerClient{
		cli: cli,
	}, nil
}

func (d *DockerClient) CreateNetwork(ctx context.Context, name string) (ContainerNetwork, error) {
	res, err := d.cli.NetworkCreate(ctx, name, types.NetworkCreate{
		CheckDuplicate: true,
	})

	if err != nil {
		return ContainerNetwork{}, err
	}

	log.WithContext(ctx).WithFields(log.Fields{
		"resultId": res.ID,
		"name":     name,
	}).Info("network created")

	return ContainerNetwork{
		Id: res.ID,
	}, nil
}

func (d *DockerClient) RunContainer(ctx context.Context, inputConfig ContainerConfig) (*Container, error) {
	// Define container configuration
	config := &container.Config{
		Image:        inputConfig.Image,
		Cmd:          inputConfig.Cmd,
		Env:          inputConfig.Env,
		User:         inputConfig.User,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	// Define host configuration
	hostConfig := &container.HostConfig{
		NetworkMode:  container.NetworkMode(inputConfig.NetworkName),
		Privileged:   inputConfig.Privileged,
		PortBindings: convertToPortBinding(inputConfig.Ports),
		Binds:        inputConfig.VolumeBinds,
	}

	hostConfig.Tmpfs = make(map[string]string)
	for _, fs := range inputConfig.TmpFs {
		hostConfig.Tmpfs[fs] = ""
	}

	_, _, err := d.cli.ImageInspectWithRaw(ctx, inputConfig.Image)

	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{
			"image": config.Image,
		}).Info("image do not exists, pulling....")

		d.pullImage(ctx, inputConfig.Image)
	}

	// Create container
	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, inputConfig.Name)
	if err != nil {
		return nil, err
	}

	// Start container
	if err := d.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	log.WithContext(ctx).WithFields(log.Fields{
		"containerId": resp.ID,
		"name":        inputConfig.Name,
	}).Debug("container started")

	ipAddr, err := GetContainerIp(ctx, *d.cli, resp.ID, inputConfig.NetworkName)

	if err != nil {
		log.WithError(err).WithField("id", resp.ID).Error("unable to get ip address for the container")

		return nil, err
	}

	return &Container{Id: resp.ID, Name: inputConfig.Name, Ip: ipAddr}, nil
}

func (d *DockerClient) Logs(ctx context.Context, containerName string, wait bool) (io.ReadCloser, error) {
	reader, err := d.cli.ContainerLogs(ctx, containerName, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     wait,
	})

	return reader, err
}

func (d *DockerClient) pullImage(ctx context.Context, imageName string) error {

	out, err := d.cli.ImagePull(context.Background(), imageName, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()

	//enable in debug mode
	//io.Copy(os.Stdout, out)

	log.WithContext(ctx).WithFields(log.Fields{
		"name": imageName,
	}).Info("pulled.")

	return nil
}

func convertToPortBinding(ports []string) map[nat.Port][]nat.PortBinding {
	if len(ports) == 0 {
		return nil
	}

	m := make(map[nat.Port][]nat.PortBinding, 0)

	for _, v := range ports {
		splitted := strings.Split(v, ":")

		containerPort := splitted[0]
		hostPort := splitted[1]

		m[nat.Port(containerPort)] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}}
	}

	return m
}

func GetContainerIp(ctx context.Context, cli client.Client, id string, networkName string) (string, error) {
	containerInfo, err := cli.ContainerInspect(context.Background(), id)
	if err != nil {
		return "", err
	}

	networkSettings := containerInfo.NetworkSettings
	ipAddress := networkSettings.Networks[networkName].IPAddress

	return ipAddress, nil
}

func (d *DockerClient) StartContainer(ctx context.Context, id string) error {
	if err := d.cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

func (d *DockerClient) StopContainer(ctx context.Context, id string) error {
	if err := d.cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return err
	}

	return nil
}

func (d *DockerClient) RemoveContainer(ctx context.Context, id string) error {
	if err := d.cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{}); err != nil {
		return err
	}

	return nil
}
