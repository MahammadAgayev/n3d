package runtimes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

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

func (d *DockerRuntime) CreateNetwork(ctx context.Context, name string, labels map[string]string) error {
	networks, err := d.cli.NetworkList(ctx, types.NetworkListOptions{})

	if err != nil {
		return err
	}

	for _, v := range networks {
		if v.Name == name {
			return nil
		}
	}

	res, err := d.cli.NetworkCreate(ctx, name, types.NetworkCreate{
		CheckDuplicate: true,
		Labels:         labels,
	})

	if err != nil {
		return err
	}

	log.WithContext(ctx).WithFields(log.Fields{
		"resultId": res.ID,
		"name":     name,
	}).Info("network created")

	return nil
}

func (d *DockerRuntime) DeleteNetwork(ctx context.Context, id string, labels map[string]string) error {
	err := d.cli.NetworkRemove(ctx, id)

	if err != nil {
		return err
	}

	log.WithContext(ctx).Info("network deleted")

	return nil
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
	}

	hostConfig.Mounts = make([]mount.Mount, 0)
	for _, v := range inputConfig.Volumes {
		typ := mount.TypeVolume
		if v.IsBind {
			typ = mount.TypeBind
		}

		mount := mount.Mount{
			Source: v.Name,
			Target: v.Dest,
			Type:   typ,
		}

		hostConfig.Mounts = append(hostConfig.Mounts, mount)
	}

	for _, v := range inputConfig.TmpFs {
		mount := mount.Mount{
			Target: v,
			Type:   mount.TypeTmpfs,
		}

		hostConfig.Mounts = append(hostConfig.Mounts, mount)
	}

	_, _, err := d.cli.ImageInspectWithRaw(ctx, inputConfig.Image)

	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{
			"image": config.Image,
		}).Info("image do not exists, pulling....")

		d.pullImage(ctx, inputConfig.Image)
	}

	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, inputConfig.Name)
	if err != nil {
		return nil, err
	}

	if len(inputConfig.ExtraCerts) > 0 {
		for _, v := range inputConfig.ExtraCerts {
			err = d.copyToContainer(ctx, resp.ID, v, "/etc/ssl/certs/")

			if err != nil {
				return nil, err
			}
		}
	}

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
	filters := filters.NewArgs()

	for k, v := range labels {
		filters.Add("label", fmt.Sprintf("%s=%s", k, v))
	}

	containers, err := d.cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters,
		All:     true,
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

func (d *DockerRuntime) GetNetworksByLabel(ctx context.Context, labels map[string]string) ([]*Network, error) {
	filters := filters.NewArgs()

	for k, v := range labels {
		filters.Add("label", fmt.Sprintf("%s=%s", k, v))
	}

	networks, err := d.cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters,
	})

	if err != nil {
		return nil, err
	}

	if len(networks) == 0 {
		return nil, nil
	}

	n3dNetworks := make([]*Network, 0)

	for _, n := range networks {
		net := &Network{
			Id:     n.ID,
			Name:   n.Name,
			Labels: n.Labels,
		}

		n3dNetworks = append(n3dNetworks, net)
	}

	return n3dNetworks, nil
}

func (d *DockerRuntime) CreateVolume(ctx context.Context, name string, labels map[string]string) error {
	_, err := d.cli.VolumeCreate(ctx, volume.CreateOptions{
		Labels: labels,
	})

	if err != nil {
		return err
	}

	return nil
}

func (d *DockerRuntime) Exec(ctx context.Context, node *Node, cmd []string) (*string, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStderr: false,
		AttachStdout: true,
		Tty:          false,
	}

	execResp, err := d.cli.ContainerExecCreate(ctx, node.Id, execConfig)

	if err != nil {
		return nil, err
	}

	resp, err := d.cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{
		Tty: execConfig.Tty,
	})

	if err != nil {
		return nil, errors.Join(errors.New("unable to read exec response"), err)
	}

	defer resp.Close()

	err = waitForExecutionUntilTimeout(ctx, func() (bool, error) {
		execStatus, err := d.cli.ContainerExecInspect(ctx, execResp.ID)

		if err != nil {
			return false, err
		}

		if execStatus.Running {
			return false, nil
		}

		return true, nil
	}, time.Second*30)

	if err != nil {
		return nil, err
	}

	buff := bytes.NewBuffer([]byte{})
	stdcopy.StdCopy(buff, nil, resp.Reader)
	text := buff.String()

	return &text, nil
}

func waitForExecutionUntilTimeout(ctx context.Context, f func() (bool, error), duration time.Duration) error {
	context, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	for {
		deadline, ok := context.Deadline()

		if ok && time.Since(deadline) >= 0 {
			break
		}

		complete, err := f()

		if err != nil {
			return err
		}

		if complete {
			break
		}
	}

	return nil
}

func (d DockerRuntime) copyToContainer(ctx context.Context, containerID, sourcePath, destPath string) error {
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}

	tarball, err := archive.TarWithOptions(sourcePath, &archive.TarOptions{})
	if err != nil {
		return err
	}

	err = d.cli.CopyToContainer(ctx, containerID, destPath, tarball, types.CopyToContainerOptions{CopyUIDGID: false})
	if err != nil {
		return err
	}

	return nil
}
