package loadbalancer

import (
	"context"
	"fmt"
	"n3d/constants"
	"n3d/runtimes"

	"github.com/docker/go-connections/nat"
	"gopkg.in/yaml.v3"
)

const DefaultLBImage = "ghcr.io/k3d-io/k3d-proxy:latest"

type loadbalancerConfig struct {
	Ports    map[string][]string  `yaml:"ports"`
	Settings loadBalancerSettings `yaml:"settings"`
}

type loadBalancerSettings struct {
	WorkerConnections   int `yaml:"workerConnections"`
	DefaultProxyTimeout int `yaml:"defaultProxyTimeout,omitempty"`
}

const (
	defaultLoadbalancerConfigPath        = "/etc/confd/values.yaml"
	defaultLoadbalancerWorkerConnections = 1024
)

type LoadBalancerCreateOptions struct {
	NetworkName  string
	PortMappings []*PortMapping
	ClusterName  string
}

type PortMapping struct {
	Port    nat.Port
	Proto   string
	Servers []string
}

func NewLoadBalancer(ctx context.Context, runtime runtimes.Runtime, opts LoadBalancerCreateOptions) (*runtimes.Node, error) {
	nodeName := fmt.Sprintf("%s-default-lb", opts.ClusterName)
	lbConfig := convertToProxyConfig(&opts)

	configYaml, err := yaml.Marshal(lbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal loadbalancer config: %w", err)
	}

	portsToExpose := make(map[nat.Port][]nat.PortBinding, 0)
	//"4646/tcp:4646"
	for _, v := range opts.PortMappings {
		if _, exists := portsToExpose[v.Port]; !exists {
			portsToExpose[v.Port] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: string(v.Port),
				},
			}
		}
	}

	nodeConf := runtimes.NodeConfig{
		Name:        nodeName,
		Image:       DefaultLBImage,
		NetworkName: opts.NetworkName,
		Files: []*runtimes.FileInNode{
			{
				Content:  configYaml,
				Path:     defaultLoadbalancerConfigPath,
				FileMode: 0644,
			},
		},
		Ports: portsToExpose,
		Labels: map[string]string{
			constants.NodeName:    nodeName,
			constants.ClusterName: opts.ClusterName,
			constants.NodeType:    constants.LoadBalancer,
		},
	}

	node, err := runtime.RunNode(ctx, nodeConf)

	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer %v", err)
	}

	return node, nil
}

func convertToProxyConfig(opts *LoadBalancerCreateOptions) *loadbalancerConfig {
	ports := make(map[string][]string)
	for _, v := range opts.PortMappings {
		ports[fmt.Sprintf("%s.%s", v.Port, v.Proto)] = v.Servers
	}

	return &loadbalancerConfig{
		Ports: ports,
		Settings: loadBalancerSettings{
			WorkerConnections:   defaultLoadbalancerWorkerConnections * len(ports),
			DefaultProxyTimeout: 300,
		},
	}
}
