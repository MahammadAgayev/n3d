package cluster

import (
	"context"
	"errors"
	"fmt"
	"n3d/constants"
	"n3d/consul"
	"n3d/containers"
	"n3d/nomad"
	"n3d/vault"

	log "github.com/sirupsen/logrus"
)

var (
	ErrorProvisionConsul      = errors.New("unable to provision consul server")
	ErrorProvisionNomadServer = errors.New("unable to provision nomad server")
	ErrorProvisionNomadWorker = errors.New("unable to provision nomad worker")
	ErrorProvisionVault       = errors.New("unable to provision vault")
)

type ClusterConfig struct {
	ClusterName string
}

type Cluster struct {
	config ClusterConfig
	cli    containers.Runtime

	network      *containers.NodeNetwork
	NomadServer  *containers.Node
	NomadClients []*containers.Node
	Consul       *containers.Node
	Vault        *vault.VaultNode
}

// func NewDockerCluster(cli containers.ContainerClient, config ClusterConfig, db db.Db) (Cluster, error) {
// 	return &DockerClsuter{
// 		cli:    cli,
// 		config: config,
// 	}, nil
// }

func ClusterCreate(ctx context.Context, config ClusterConfig, runtime containers.Runtime) error {
	networkName := config.ClusterName + "-net"

	_, err := runtime.CreateNetwork(ctx, networkName)

	if err != nil {
		return err
	}

	consul, err := consul.NewConsulServer(ctx, runtime, consul.ConsulConfiguration{
		ClusterName: config.ClusterName,
		NetworkName: networkName,
		Id:          0,
	})

	if err != nil {
		return errors.Join(err, ErrorProvisionConsul)
	}

	log.WithContext(ctx).WithField("Name", consul.Name).Info("consul started.")

	vault, err := vault.NewVault(ctx, runtime, vault.VaultConfiguration{
		ClusterName: config.ClusterName,
		ConsulAddr:  consul.Ip,
		Id:          0,
		NetworkName: networkName,
	})

	if err != nil {
		return errors.Join(err, ErrorProvisionVault)
	}

	log.WithContext(ctx).WithFields(log.Fields{
		"UnsealKey": vault.UnsealKey,
		"RootToken": vault.RootToken,
		"Name":      vault.Node.Name,
	}).Info("vault started.")

	nomadServer, err := nomad.NewNomadServer(ctx, runtime, nomad.NomadConfiguration{
		NetworkName: networkName,
		ClusterName: config.ClusterName,
		ConsulAddr:  fmt.Sprintf("%s:8500", consul.Ip),
		VaultAddr:   fmt.Sprintf("http://%s:8200", vault.Node.Ip),
		VaultToken:  vault.RootToken,
		Id:          0,
	})

	if err != nil {
		return errors.Join(err, ErrorProvisionNomadServer)
	}

	nomadServer = nomadServer
	log.WithContext(ctx).WithField("name", nomadServer.Name).Info("nomad server started.")

	_, err = nomad.NewNomadClient(ctx, runtime, nomad.NomadConfiguration{
		NetworkName: networkName,
		ClusterName: config.ClusterName,
		ConsulAddr:  fmt.Sprintf("%s:8500", consul.Ip),
		VaultAddr:   fmt.Sprintf("http://%s:8200", vault.Node.Ip),
		VaultToken:  vault.RootToken,
		Id:          0,
	})

	if err != nil {
		return errors.Join(err, ErrorProvisionNomadWorker)
	}

	log.WithContext(ctx).WithField("name", nomadServer.Name).Info("nomad worker started.")
	log.WithContext(ctx).WithField("cluster-name", config.ClusterName).Info("cluster provisioned.")

	return nil
}

func ClusterDelete(ctx context.Context, d Cluster) error {
	for _, w := range d.NomadClients {
		_ = d.cli.StopContainer(ctx, w.Id)

		_ = d.cli.RemoveContainer(ctx, w.Id)
	}

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed nomad workers.")

	_ = d.cli.StopContainer(ctx, d.NomadServer.Id)
	_ = d.cli.RemoveContainer(ctx, d.NomadServer.Id)

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed nomad server.")

	_ = d.cli.StopContainer(ctx, d.Vault.Node.Id)
	_ = d.cli.RemoveContainer(ctx, d.Vault.Node.Id)

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed vault.")

	_ = d.cli.StopContainer(ctx, d.Consul.Id)
	_ = d.cli.RemoveContainer(ctx, d.Consul.Id)

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("removed consul.")

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("cluster destroyed.")

	return nil
}

func ClusterGet(ctx context.Context, runtime containers.Runtime, config ClusterConfig) (*Cluster, error) {

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
		NomadClients: make([]*containers.Node, 0),
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
				Node:      v,
				UnsealKey: v.Labels[constants.VaultUnsealKey],
				RootToken: v.Labels[constants.VaultRootToken],
			}
		}
	}

	return cluster, nil
}
