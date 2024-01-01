package consul

import (
	"context"
	"fmt"
	"n3d/constants"
	"n3d/runtimes"
)

type ConsulConfiguration struct {
	ClusterName string
	NetworkName string
	Id          int
}

const (
	imageName = "consul:1.15.4"
)

func NewConsulServer(ctx context.Context, runtime runtimes.Runtime, config ConsulConfiguration) (*runtimes.Node, error) {
	volName := fmt.Sprintf("%s-consul-vol", config.ClusterName)
	runtime.CreateVolume(ctx, volName, map[string]string{
		constants.ClusterName: config.ClusterName,
		constants.VolumeType:  constants.Consul,
	})

	ctn, err := runtime.RunContainer(ctx, runtimes.NodeConfig{
		Image:       imageName,
		Name:        fmt.Sprintf("%s-consul-server-%d", config.ClusterName, config.Id),
		NetworkName: config.NetworkName,
		Cmd: []string{"agent", "-server", "-ui", "-bootstrap-expect=1",
			"-client=0.0.0.0", "-hcl=connect { enabled = true }", "-hcl=ports { grpc = 8502 serf_lan = 28301 }"},
		Ports: []string{"8500/tcp:8500"},
		Volumes: []*runtimes.Volume{
			{
				Name:   volName,
				Dest:   "/consul/data",
				IsBind: false,
			},
		},
		Labels: map[string]string{
			constants.NodeType:    constants.Consul,
			constants.ClusterName: config.ClusterName,
		},
	})

	if err != nil {
		return nil, err
	}

	return ctn, nil
}
