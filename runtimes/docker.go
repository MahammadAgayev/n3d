package runtimes

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func (d *DockerRuntime) RunNode(ctx context.Context, node NodeConfig) (*Node, error) {
	// Define container configuration
	config := &container.Config{
		Image:        node.Image,
		Cmd:          node.Cmd,
		Env:          node.Env,
		User:         node.User,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Labels:       node.Labels,
	}

	// Define host configuration
	hostConfig := &container.HostConfig{
		NetworkMode:  container.NetworkMode(node.NetworkName),
		Privileged:   node.Privileged,
		PortBindings: node.Ports,
	}

	exposedPorts := nat.PortSet{}
	for ep := range node.Ports {
		if _, exists := exposedPorts[ep]; !exists {
			exposedPorts[ep] = struct{}{}
		}
	}

	config.ExposedPorts = exposedPorts

	hostConfig.Mounts = make([]mount.Mount, 0)
	for _, v := range node.Volumes {
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

	for _, v := range node.TmpFs {
		mount := mount.Mount{
			Target: v,
			Type:   mount.TypeTmpfs,
		}

		hostConfig.Mounts = append(hostConfig.Mounts, mount)
	}

	_, _, err := d.cli.ImageInspectWithRaw(ctx, node.Image)

	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{
			"image": config.Image,
		}).Info("image do not exists, pulling....")

		d.pullImage(ctx, node.Image)
	}

	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, node.Name)
	if err != nil {
		return nil, err
	}

	if len(node.ExtraCerts) > 0 {
		for _, v := range node.ExtraCerts {
			err = d.copyToNode(ctx, resp.ID, v, "/etc/ssl/certs/")

			if err != nil {
				return nil, err
			}
		}
	}

	if len(node.Files) > 0 {
		for _, f := range node.Files {
			err = d.writeToNode(ctx, f.Content, f.Path, f.FileMode, resp.ID)

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
		"name":        node.Name,
	}).Debug("container started")

	ipAddr, err := GetContainerIp(ctx, *d.cli, resp.ID, node.NetworkName)

	if err != nil {
		log.WithError(err).WithField("id", resp.ID).Error("unable to get ip address for the container")

		return nil, err
	}

	return &Node{Id: resp.ID, Name: node.Name, Ip: ipAddr}, nil
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

func GetContainerIp(ctx context.Context, cli client.Client, id string, networkName string) (string, error) {
	containerInfo, err := cli.ContainerInspect(context.Background(), id)
	if err != nil {
		return "", err
	}

	networkSettings := containerInfo.NetworkSettings
	ipAddress := networkSettings.Networks[networkName].IPAddress

	return ipAddress, nil
}

func (d *DockerRuntime) StartNode(ctx context.Context, node *Node) error {
	if err := d.cli.ContainerStart(ctx, node.Id, types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

func (d *DockerRuntime) StopNode(ctx context.Context, node *Node) error {
	if err := d.cli.ContainerStop(ctx, node.Id, container.StopOptions{}); err != nil {
		return err
	}

	return nil
}

func (d *DockerRuntime) RemoveNode(ctx context.Context, node *Node) error {
	if err := d.cli.ContainerRemove(ctx, node.Id, types.ContainerRemoveOptions{}); err != nil {
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
		Name:   name,
	})

	if err != nil {
		return err
	}

	return nil
}

func (d *DockerRuntime) GetVolumesByLabel(ctx context.Context, labels map[string]string) ([]*Volume, error) {
	filters := filters.NewArgs()

	for k, v := range labels {
		filters.Add("label", fmt.Sprintf("%s=%s", k, v))
	}

	volumeResp, err := d.cli.VolumeList(ctx, volume.ListOptions{
		Filters: filters,
	})

	if err != nil {
		return nil, err
	}

	if len(volumeResp.Volumes) == 0 {
		return nil, nil
	}

	n3dVolumes := make([]*Volume, 0)

	for _, n := range volumeResp.Volumes {
		vol := &Volume{
			Name: n.Name,
			Dest: n.Mountpoint,
		}

		n3dVolumes = append(n3dVolumes, vol)
	}

	return n3dVolumes, nil
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

func (d *DockerRuntime) RemoveVolume(ctx context.Context, name string) error {
	err := d.cli.VolumeRemove(ctx, name, false)

	return err
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

func (d DockerRuntime) copyToNode(ctx context.Context, containerID, sourcePath, destPath string) error {
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

func (d *DockerRuntime) writeToNode(ctx context.Context, content []byte, dest string, mode os.FileMode, containerId string) error {
	buf := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buf)
	defer tarWriter.Close()
	tarHeader := &tar.Header{
		Name: dest,
		Mode: int64(mode),
		Size: int64(len(content)),
	}

	if err := tarWriter.WriteHeader(tarHeader); err != nil {
		return fmt.Errorf("failed to write tar header: %+v", err)
	}

	if _, err := tarWriter.Write(content); err != nil {
		return fmt.Errorf("failed to write tar content: %+v", err)
	}

	if err := tarWriter.Close(); err != nil {
		log.Debugf("failed to close tar writer: %+v", err)
	}

	tarBytes := bytes.NewReader(buf.Bytes())
	if err := d.cli.CopyToContainer(ctx, containerId, "/", tarBytes, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true}); err != nil {
		return fmt.Errorf("failed to copy content to container '%s': %+v", containerId, err)
	}

	return nil
}
