package cluster

import (
	"context"
	"errors"
	"fmt"
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

type Cluster interface {
	Provision(ctx context.Context) error
	Destroy(ctx context.Context) error
	// Resume() error
	// Stop() error
}

type DockerClsuter struct {
	config ClusterConfig
	cli    containers.ContainerClient

	network      *containers.ContainerNetwork
	nomadServer  *containers.Container
	nomadWorkers []containers.Container
	consul       *containers.Container
	vault        *vault.VaultContainer
}

func NewDockerCluster(cli containers.ContainerClient, config ClusterConfig) (Cluster, error) {
	return &DockerClsuter{
		cli:    cli,
		config: config,
	}, nil
}

func (d *DockerClsuter) Provision(ctx context.Context) error {
	networkName := d.config.ClusterName + "-net"

	network, err := d.cli.CreateNetwork(ctx, networkName)

	if err != nil {
		return err
	}

	d.network = &network

	consul, err := consul.NewConsulServer(ctx, d.cli, consul.ConsulConfiguration{
		ClusterName: d.config.ClusterName,
		NetworkName: networkName,
		Id:          0,
	})

	if err != nil {
		return errors.Join(err, ErrorProvisionConsul)
	}

	d.consul = consul
	log.WithContext(ctx).WithField("Name", consul.Name).Info("consul started.")

	vault, err := vault.NewVault(ctx, d.cli, vault.VaultConfiguration{
		ClusterName: d.config.ClusterName,
		ConsulAddr:  consul.Ip,
		Id:          0,
		NetworkName: networkName,
	})

	d.vault = vault

	log.WithContext(ctx).WithFields(log.Fields{
		"UnsealKey": vault.UnsealKey,
		"RootToken": vault.RootToken,
		"Name":      vault.Container.Name,
	}).Info("vault started.")

	if err != nil {
		return errors.Join(err, ErrorProvisionVault)
	}

	nomadServer, err := nomad.NewNomadServer(ctx, d.cli, nomad.NomadConfiguration{
		NetworkName: networkName,
		ClusterName: d.config.ClusterName,
		ConsulAddr:  fmt.Sprintf("%s:8500", consul.Ip),
		VaultAddr:   fmt.Sprintf("%s:8200", vault.Container.Ip),
		VaultToken:  vault.RootToken,
		Id:          0,
	})

	d.nomadServer = nomadServer
	log.WithContext(ctx).WithField("name", nomadServer.Name).Info("nomad server started.")

	if err != nil {
		return errors.Join(err, ErrorProvisionNomadServer)
	}

	d.nomadWorkers = make([]containers.Container, 0)
	nomadWorker, err := nomad.NewNomadWorker(ctx, d.cli, nomad.NomadConfiguration{
		NetworkName: networkName,
		ClusterName: d.config.ClusterName,
		ConsulAddr:  fmt.Sprintf("%s:8500", consul.Ip),
		VaultAddr:   fmt.Sprintf("%s:8200", vault.Container.Ip),
		VaultToken:  vault.RootToken,
		Id:          len(d.nomadWorkers),
	})

	if err != nil {
		return errors.Join(err, ErrorProvisionNomadWorker)
	}

	d.nomadWorkers = append(d.nomadWorkers, *nomadWorker)

	log.WithContext(ctx).WithField("name", nomadServer.Name).Info("nomad worker started.")
	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("cluster provisioned.")

	return nil
}

func (d *DockerClsuter) Destroy(ctx context.Context) error {
	for _, w := range d.nomadWorkers {
		_ = d.cli.StopContainer(ctx, w.Id)

		_ = d.cli.RemoveContainer(ctx, w.Id)
	}

	_ = d.cli.StopContainer(ctx, d.nomadServer.Id)
	_ = d.cli.RemoveContainer(ctx, d.nomadServer.Id)

	_ = d.cli.StopContainer(ctx, d.vault.Container.Id)
	_ = d.cli.RemoveContainer(ctx, d.vault.Container.Id)

	_ = d.cli.StopContainer(ctx, d.consul.Id)
	_ = d.cli.RemoveContainer(ctx, d.consul.Id)

	log.WithContext(ctx).WithField("cluster-name", d.config.ClusterName).Info("cluster destroyed.")

	return nil
}
