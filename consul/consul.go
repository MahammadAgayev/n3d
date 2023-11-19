package consul

import (
	"context"
	"fmt"
	"n3d/containers"
)

type ConsulConfiguration struct {
	ClusterName string
	NetworkName string
	Id          int
}

const (
	imageName = "consul:1.15.4"
)

func NewConsulServer(ctx context.Context, cli containers.ContainerClient, config ConsulConfiguration) (*containers.Container, error) {
	ctn, err := cli.RunContainer(ctx, containers.ContainerConfig{
		Image:       imageName,
		Name:        fmt.Sprintf("%s-consul-server-%d", config.ClusterName, config.Id),
		NetworkName: config.NetworkName,
		Cmd: []string{"agent", "-server", "-ui", "-bootstrap-expect=1",
			"-client=0.0.0.0", "-hcl=connect { enabled = true }", "-hcl=ports { grpc = 8502 serf_lan = 28301 }"},
		Ports: []string{"8500/tcp:8500"},
		TmpFs: []string{"/opt/consul"},
	})

	if err != nil {
		return nil, err
	}

	return ctn, nil
}
