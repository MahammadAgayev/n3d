package cluster

import (
	"context"
	"errors"
	"fmt"
	"n3d/constants"
	"n3d/consul"
	"n3d/nomad"
	"n3d/runtimes"
	"n3d/vault"

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
	ClusterName string
	WorkerCount int
	ExtraCerts  []string
}

type Cluster struct {
	config ClusterConfig

	Network      *runtimes.Network
	NomadServer  *runtimes.Node
	NomadClients []*runtimes.Node
	Consul       *runtimes.Node
	Vault        *vault.VaultNode
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

	for i := 0; i < config.WorkerCount; i++ {
		_, err = nomad.NewNomadClient(ctx, runtime, nomad.NomadConfiguration{
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
	}

	log.WithContext(ctx).WithField("name", nomadServer.Name).Info("nomad worker started.")
	log.WithContext(ctx).WithField("cluster-name", config.ClusterName).Info("cluster provisioned.")

	return nil
}

func ClusterDelete(ctx context.Context, d *Cluster, runtime runtimes.Runtime) error {
	for _, w := range d.NomadClients {
		_ = runtime.StopContainer(ctx, w.Id)

		_ = runtime.RemoveContainer(ctx, w.Id)
	}

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed nomad workers.")

	if d.NomadServer != nil {
		_ = runtime.StopContainer(ctx, d.NomadServer.Id)
		_ = runtime.RemoveContainer(ctx, d.NomadServer.Id)

		log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed nomad server.")
	}

	if d.Vault != nil {
		_ = runtime.StopContainer(ctx, d.Vault.Node.Id)
		_ = runtime.RemoveContainer(ctx, d.Vault.Node.Id)

		log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed vault.")
	}

	if d.Consul != nil {
		_ = runtime.StopContainer(ctx, d.Consul.Id)
		_ = runtime.RemoveContainer(ctx, d.Consul.Id)

		log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed consul.")
	}

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

	return cluster, nil
}

func ClusterStop(ctx context.Context, d *Cluster, runtime runtimes.Runtime) error {
	for _, w := range d.NomadClients {
		_ = runtime.StopContainer(ctx, w.Id)
	}

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped nomad workers.")

	_ = runtime.StopContainer(ctx, d.NomadServer.Id)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped nomad server.")

	_ = runtime.StopContainer(ctx, d.Vault.Node.Id)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped vault.")

	_ = runtime.StopContainer(ctx, d.Consul.Id)

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped consul.")
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("stopped cluster.")

	return nil
}

func ClusterStart(ctx context.Context, d *Cluster, runtime runtimes.Runtime) error {

	_ = runtime.StartContainer(ctx, d.Consul.Id)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started consul.")

	_ = runtime.StartContainer(ctx, d.Vault.Node.Id)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started vault.")

	_ = runtime.StartContainer(ctx, d.NomadServer.Id)
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started nomad server.")

	for _, w := range d.NomadClients {
		_ = runtime.StartContainer(ctx, w.Id)
	}
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started nomad workers.")

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("started cluster.")

	return nil
}
