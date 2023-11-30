package containers

import (
	"context"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

type Node struct {
	Id     string
	Name   string
	Ip     string
	Labels map[string]string
}

type NodeNetwork struct {
	Id string
}

type NodeConfig struct {
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
	Labels      map[string]string
}

type Volume struct {
	Src     string
	Dest    string
	IsTmpFs bool
}

type Runtime interface {
	CreateNetwork(ctx context.Context, name string) (NodeNetwork, error)
	RunContainer(ctx context.Context, config NodeConfig) (*Node, error)
	Logs(ctx context.Context, containerName string, wait bool) (io.ReadCloser, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string) error
	GetNodesByLabel(ctx context.Context, labels map[string]string) ([]*Node, error)
	AddLabels(ctx context.Context, node *Node, labels map[string]string) error
}

type DockerRuntime struct {
	cli *client.Client
}

func NewDockerRuntime() (Runtime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return nil, err
	}

	return &DockerRuntime{
		cli: cli,
	}, nil
}

func (d *DockerRuntime) CreateNetwork(ctx context.Context, name string) (NodeNetwork, error) {
	networks, err := d.cli.NetworkList(ctx, types.NetworkListOptions{})

	if err != nil {
		return NodeNetwork{}, err
	}

	for _, v := range networks {
		if v.Name == name {
			return NodeNetwork{
				Id: v.ID,
			}, nil
		}
	}

	res, err := d.cli.NetworkCreate(ctx, name, types.NetworkCreate{
		CheckDuplicate: true,
	})

	if err != nil {
		return NodeNetwork{}, err
	}

	log.WithContext(ctx).WithFields(log.Fields{
		"resultId": res.ID,
		"name":     name,
	}).Info("network created")

	return NodeNetwork{
		Id: res.ID,
	}, nil
}

func (d *DockerRuntime) RunContainer(ctx context.Context, inputConfig NodeConfig) (*Node, error) {
	// Define container configuration
	config := &container.Config{
		Image:        inputConfig.Image,
		Cmd:          inputConfig.Cmd,
		Env:          inputConfig.Env,
		User:         inputConfig.User,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Labels:       inputConfig.Labels,
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

	return &Node{Id: resp.ID, Name: inputConfig.Name, Ip: ipAddr}, nil
}

func (d *DockerRuntime) Logs(ctx context.Context, containerName string, wait bool) (io.ReadCloser, error) {
	reader, err := d.cli.ContainerLogs(ctx, containerName, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     wait,
	})

	return reader, err
}

func (d *DockerRuntime) pullImage(ctx context.Context, imageName string) error {

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

func (d *DockerRuntime) StartContainer(ctx context.Context, id string) error {
	if err := d.cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

func (d *DockerRuntime) StopContainer(ctx context.Context, id string) error {
	if err := d.cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return err
	}

	return nil
}

func (d *DockerRuntime) RemoveContainer(ctx context.Context, id string) error {
	if err := d.cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{}); err != nil {
		return err
	}

	return nil
}

func (d *DockerRuntime) GetNodesByLabel(ctx context.Context, labels map[string]string) ([]*Node, error) {
	filters := filters.Args{}

	for k, v := range labels {
		filters.Add(k, v)
	}

	containers, err := d.cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters,
	})

	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0)

	for _, v := range containers {

		node := &Node{
			Name:   v.Names[0],
			Id:     v.ID,
			Labels: v.Labels,
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (d *DockerRuntime) AddLabels(ctx context.Context, node *Node, labels map[string]string) error {
	return nil
}
