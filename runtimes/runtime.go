package runtimes

import (
	"context"
	"io"
	"log"
)

type Node struct {
	Id     string
	Name   string
	Ip     string
	Labels map[string]string
}

type Network struct {
	Id     string
	Name   string
	Labels map[string]string
}

type NodeConfig struct {
	NetworkName string
	Name        string
	Image       string
	Cmd         []string
	Env         []string
	User        string
	Volumes     []*Volume
	TmpFs       []string
	Privileged  bool
	Ports       []string
	Labels      map[string]string
}

type Volume struct {
	Name   string
	Dest   string
	IsBind bool
}

type Runtime interface {
	CreateNetwork(ctx context.Context, name string, labels map[string]string) error
	RunNode(ctx context.Context, config NodeConfig) (*Node, error)
	Logs(ctx context.Context, nodeName string, wait bool) (io.ReadCloser, error)

	StartNode(ctx context.Context, node *Node) error
	StopNode(ctx context.Context, node *Node) error
	RemoveNode(ctx context.Context, node *Node) error

	GetNodesByLabel(ctx context.Context, labels map[string]string) ([]*Node, error)
	GetNetworksByLabel(ctx context.Context, labels map[string]string) ([]*Network, error)
	GetVolumesByLabel(ctx context.Context, labels map[string]string) ([]*Volume, error)

	Exec(ctx context.Context, node *Node, cmd []string) (*string, error)
	CreateVolume(ctx context.Context, name string, labels map[string]string) error
	RemoveVolume(ctx context.Context, name string) error
}

var SelectedRuntime Runtime

func SetDockerRuntime() {
	runtime, err := NewDockerRuntime()

	if err != nil {
		log.Fatalln(err)
	}

	SelectedRuntime = runtime
}
