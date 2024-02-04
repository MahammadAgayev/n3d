package cluster

import (
	"context"
	"errors"
	"fmt"
	"n3d/constants"
	"n3d/consul"
	"n3d/loadbalancer"
	"n3d/nomad"
	"n3d/runtimes"
	"n3d/vault"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

var (
	ErrorProvisionConsul      = errors.New("unable to provision consul server")
	ErrorProvisionNomadServer = errors.New("unable to provision nomad server")
	ErrorProvisionNomadWorker = errors.New("unable to provision nomad worker")
	ErrorProvisionVault       = errors.New("unable to provision vault")
	ErrorGetNetwork           = errors.New("unable to get network")
)

type ClusterConfig struct {
	ClusterName   string
	WorkerCount   int
	ExtraCerts    []string
	PortsToExpose []string
}

type Cluster struct {
	config ClusterConfig

	Network      *runtimes.Network
	NomadServer  *runtimes.Node
	NomadClients []*runtimes.Node
	Consul       *runtimes.Node
	Vault        *vault.VaultNode
	LoadBalancer *runtimes.Node
	Volumes      []*runtimes.Volume
}

func ClusterCreate(ctx context.Context, config ClusterConfig, runtime runtimes.Runtime) error {
	networkName := config.ClusterName + "-net"

	err := runtime.CreateNetwork(ctx, networkName, map[string]string{
		constants.ClusterName: config.ClusterName,
	})

	if err != nil {
		return err
	}

	consul, err := consul.NewConsulServer(ctx, runtime, consul.ConsulConfiguration{
		ClusterName: config.ClusterName,
		NetworkName: networkName,
		Id:          0,
	})

	if err != nil {
		return errors.Join(ErrorProvisionConsul, err)
	}

	log.WithContext(ctx).WithField("Name", consul.Name).Info("consul started.")

	vault, err := vault.NewVault(ctx, runtime, vault.VaultConfiguration{
		ClusterName: config.ClusterName,
		ConsulAddr:  fmt.Sprintf("%s:8500", consul.Name),
		Id:          0,
		NetworkName: networkName,
	})

	if err != nil {
		return errors.Join(ErrorProvisionVault, err)
	}

	log.WithContext(ctx).WithFields(log.Fields{
		"UnsealKey": vault.UnsealKey,
		"RootToken": vault.RootToken,
		"Name":      vault.Node.Name,
	}).Info("vault started.")

	nomadServer, err := nomad.NewNomadServer(ctx, runtime, nomad.NomadConfiguration{
		NetworkName: networkName,
		ClusterName: config.ClusterName,
		ConsulAddr:  fmt.Sprintf("%s:8500", consul.Name),
		VaultAddr:   fmt.Sprintf("http://%s:8200", vault.Node.Name),
		VaultToken:  vault.RootToken,
		Id:          0,
		ExtraCerts:  config.ExtraCerts,
	})

	if err != nil {
		return errors.Join(ErrorProvisionNomadServer, err)
	}

	log.WithContext(ctx).WithField("name", nomadServer.Name).Info("nomad server started.")

	workers := []string{}
	for i := 0; i < config.WorkerCount; i++ {
		w, err := nomad.NewNomadClient(ctx, runtime, nomad.NomadConfiguration{
			NetworkName: networkName,
			ClusterName: config.ClusterName,
			ConsulAddr:  fmt.Sprintf("%s:8500", consul.Name),
			VaultAddr:   fmt.Sprintf("http://%s:8200", vault.Node.Name),
			VaultToken:  vault.RootToken,
			Id:          i,
			ExtraCerts:  config.ExtraCerts,
		})

		if err != nil {
			return errors.Join(ErrorProvisionNomadWorker, err)
		}

		workers = append(workers, w.Name)
	}

	log.WithContext(ctx).WithField("name", nomadServer.Name).Info("nomad server started.")

	_, err = loadbalancer.NewLoadBalancer(ctx, runtime, loadbalancer.LoadBalancerCreateOptions{
		NetworkName:  networkName,
		ClusterName:  config.ClusterName,
		PortMappings: generatePortMappings(config.PortsToExpose, nomadServer.Name, consul.Name, vault.Node.Name, workers),
	})

	if err != nil {
		return fmt.Errorf("unable to create load balancer %v", err)
	}

	log.WithContext(ctx).WithField("cluster-name", config.ClusterName).Info("cluster provisioned.")

	return nil
}

func ClusterDelete(ctx context.Context, d *Cluster, runtime runtimes.Runtime) error {
	for _, w := range d.NomadClients {
		_ = runtime.StopNode(ctx, w)

		_ = runtime.RemoveNode(ctx, w)
	}

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed nomad workers.")

	if d.NomadServer != nil {
		_ = runtime.StopNode(ctx, d.NomadServer)
		_ = runtime.RemoveNode(ctx, d.NomadServer)

		log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed nomad server.")
	}

	if d.Vault != nil {
		_ = runtime.StopNode(ctx, d.Vault.Node)
		_ = runtime.RemoveNode(ctx, d.Vault.Node)

		log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed vault.")
	}

	if d.Consul != nil {
		_ = runtime.StopNode(ctx, d.Consul)
		_ = runtime.RemoveNode(ctx, d.Consul)

		log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed consul.")
	}

	_ = removeClusterVolumes(ctx, runtime, d.Volumes)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed volumes.")

	_ = runtime.StopNode(ctx, d.LoadBalancer)
	_ = runtime.RemoveNode(ctx, d.LoadBalancer)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed loadbalancer.")

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("cluster destroyed.")

	return nil
}

func ClusterGet(ctx context.Context, runtime runtimes.Runtime, config ClusterConfig) (*Cluster, error) {
	labels := map[string]string{
		constants.ClusterName: config.ClusterName,
	}

	nodes, err := runtime.GetNodesByLabel(ctx, labels)

	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, nil
	}

	cluster := &Cluster{
		NomadClients: make([]*runtimes.Node, 0),
		config:       config,
	}

	for _, v := range nodes {

		typ := v.Labels[constants.NodeType]

		switch typ {
		case constants.NomadServer:
			cluster.NomadServer = v
		case constants.NomadClient:
			cluster.NomadClients = append(cluster.NomadClients, v)
		case constants.Consul:
			cluster.Consul = v
		case constants.Vault:
			cluster.Vault = &vault.VaultNode{
				Node: v,
				//TODO find a way to set these to vault
				//UnsealKey: v.Labels[constants.VaultUnsealKey],
				//RootToken: v.Labels[constants.VaultRootToken],
			}
		case constants.LoadBalancer:
			cluster.LoadBalancer = v
		}
	}

	networks, err := runtime.GetNetworksByLabel(ctx, map[string]string{
		constants.ClusterName: config.ClusterName,
	})

	if err != nil {
		return nil, errors.Join(ErrorGetNetwork, err)
	}

	for _, v := range networks {
		cluster.Network = v
	}

	volumes, err := runtime.GetVolumesByLabel(ctx, map[string]string{
		constants.ClusterName: config.ClusterName,
	})

	if err != nil {
		return nil, fmt.Errorf("error listing cluster volumes %v", err)
	}

	cluster.Volumes = volumes

	return cluster, nil
}

func ClusterStop(ctx context.Context, d *Cluster, runtime runtimes.Runtime) error {
	for _, w := range d.NomadClients {
		_ = runtime.StopNode(ctx, w)
	}

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped nomad workers.")

	_ = runtime.StopNode(ctx, d.NomadServer)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped nomad server.")

	_ = runtime.StopNode(ctx, d.Vault.Node)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped vault.")

	_ = runtime.StopNode(ctx, d.Consul)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped consul.")

	_ = runtime.StopNode(ctx, d.LoadBalancer)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped loadbalancer.")

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped cluster.")

	return nil
}

func ClusterStart(ctx context.Context, d *Cluster, runtime runtimes.Runtime) error {

	_ = runtime.StartNode(ctx, d.Consul)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started consul.")

	_ = runtime.StartNode(ctx, d.Vault.Node)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started vault.")

	_ = runtime.StartNode(ctx, d.NomadServer)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started nomad server.")

	for _, w := range d.NomadClients {
		_ = runtime.StartNode(ctx, w)
	}
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started nomad workers.")

	_ = runtime.StartNode(ctx, d.LoadBalancer)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started loadbalancer.")

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started cluster.")

	return nil
}

func removeClusterVolumes(ctx context.Context, runtime runtimes.Runtime, volumes []*runtimes.Volume) error {
	if len(volumes) == 0 {
		log.Warn("no volumes found to delete")
		return nil
	}

	for _, v := range volumes {
		err := runtime.RemoveVolume(ctx, v.Name)

		if err != nil {
			return err
		}
	}

	return nil
}

func generatePortMappings(portsToExpose []string, nomarServer string, consul string, vault string, nomadWorkers []string) []*loadbalancer.PortMapping {
	mappings := []*loadbalancer.PortMapping{
		{
			Proto: "tcp",
			Port:  "4646",
			Servers: []string{
				nomarServer,
			},
		},
		{
			Proto: "tcp",
			Port:  "8500",
			Servers: []string{
				consul,
			},
		},
		{
			Proto: "tcp",
			Port:  "8200",
			Servers: []string{
				vault,
			},
		},
	}

	for _, v := range portsToExpose {
		mappings = append(mappings, &loadbalancer.PortMapping{
			Proto:   "tcp",
			Servers: nomadWorkers,
			Port:    nat.Port(v),
		})
	}

	return mappings
}
