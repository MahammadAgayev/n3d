package nomad

import (
	"context"
	"fmt"
	"n3d/containers"

	log "github.com/sirupsen/logrus"
)

const (
	nomadServerImage = "multani/nomad:1.6.3"
	nomadClientImage = "mahammad/nomad:1.6.3-a6"
)

type NomadConfiguration struct {
	NetworkName string
	ClusterName string
	ConsulAddr  string
	VaultAddr   string
	VaultToken  string
	Id          int
}

func NewNomadServer(ctx context.Context, cli containers.ContainerClient, config NomadConfiguration) (*containers.Container, error) {
	nomadConfig := `
	    server {
	    	enabled = true
	    	bootstrap_expect = 1
	      }

	      data_dir = "/nomad/data/"
	      
	      bind_addr = "0.0.0.0"

		consul { 
			address = "%s"
		}

		vault {
			enabled = true
			address = "%s:8200"
			token   =  "%s"
		}
	    `

	nomadConfig = fmt.Sprintf(nomadConfig, config.ConsulAddr, config.VaultAddr, config.VaultToken)

	ctn, err := cli.RunContainer(ctx, containers.ContainerConfig{
		Name:        fmt.Sprintf("%s-nomad-server-%d", config.ClusterName, config.Id),
		Image:       nomadServerImage,
		NetworkName: config.NetworkName,
		Cmd:         []string{"agent"},
		Env:         []string{fmt.Sprintf("NOMAD_LOCAL_CONFIG=%s", nomadConfig)},
		Ports:       []string{"4646/tcp:4646"},
		TmpFs:       []string{"/nomad/data/"},
	})

	if err != nil {
		return nil, err
	}

	log.WithContext(ctx).WithFields(log.Fields{
		"name": ctn.Name,
	}).Info("started nomad server")

	return ctn, nil
}

func NewNomadWorker(ctx context.Context, cli containers.ContainerClient, config NomadConfiguration) (*containers.Container, error) {
	nomadConfig := `
	client {
		enabled = true
	  }
	  bind_addr = "0.0.0.0"
	  data_dir = "/nomad/data/"
	  consul { 
		address = "%s"
	  }	  
	  vault {
		enabled = true
		address = "%s:8200"
		token   =  "%s"
	  }	  
	`
	nomadConfig = fmt.Sprintf(nomadConfig, config.ConsulAddr, config.VaultAddr, config.VaultToken)

	ctn, err := cli.RunContainer(ctx, containers.ContainerConfig{
		Name:        fmt.Sprintf("%s-nomad-client-%d", config.ClusterName, config.Id),
		Image:       nomadClientImage,
		NetworkName: config.NetworkName,
		Cmd:         []string{"agent"},
		Env:         []string{fmt.Sprintf("NOMAD_LOCAL_CONFIG=%s", nomadConfig)},
		Privileged:  true,
		TmpFs: []string{
			"/var/run",
			"/run",
			"/nomad/data/",
		},
	})

	if err != nil {
		return nil, err
	}

	return ctn, nil
}
