package nomad

import (
	"context"
	"fmt"
	"n3d/constants"
	"n3d/runtimes"

	log "github.com/sirupsen/logrus"
)

const (
	nomadServerImage = "multani/nomad:1.6.3"
	nomadClientImage = "mahammadagayev/nomad-client:1.6.3"
)

type NomadConfiguration struct {
	NetworkName string
	ClusterName string
	ConsulAddr  string
	VaultAddr   string
	VaultToken  string
	Id          int
	ExtraCerts  []string
}

func NewNomadServer(ctx context.Context, runtime runtimes.Runtime, config NomadConfiguration) (*runtimes.Node, error) {
	nodeName := fmt.Sprintf("%s-nomad-server-%d", config.ClusterName, config.Id)
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
			address =  "%s"
			token   =  "%s"
		}
	    `

	nomadConfig = fmt.Sprintf(nomadConfig, config.ConsulAddr, config.VaultAddr, config.VaultToken)

	volName := fmt.Sprintf("%s-nomad-server-vol-%d", config.ClusterName, config.Id)
	runtime.CreateVolume(ctx, volName, map[string]string{
		constants.ClusterName: config.ClusterName,
		constants.VolumeType:  constants.NomadServer,
		constants.NodeName:    nodeName,
	})

	ctn, err := runtime.RunNode(ctx, runtimes.NodeConfig{
		Name:        nodeName,
		Image:       nomadServerImage,
		NetworkName: config.NetworkName,
		Cmd:         []string{"agent"},
		Env:         []string{fmt.Sprintf("NOMAD_LOCAL_CONFIG=%s", nomadConfig)},
		Volumes: []*runtimes.Volume{
			{
				Name:   volName,
				Dest:   "/nomad/data",
				IsBind: false,
			},
		},
		Labels: map[string]string{
			constants.NodeType:    constants.NomadServer,
			constants.ClusterName: config.ClusterName,
		},
		ExtraCerts: config.ExtraCerts,
	})

	if err != nil {
		return nil, err
	}

	log.WithContext(ctx).WithFields(log.Fields{
		"name": ctn.Name,
	}).Trace("started nomad server")

	return ctn, nil
}

func NewNomadClient(ctx context.Context, runtime runtimes.Runtime, config NomadConfiguration) (*runtimes.Node, error) {
	nodeName := fmt.Sprintf("%s-nomad-client-%d", config.ClusterName, config.Id)

	nomadConfig := `
	client {
		enabled = true
	  }
	  bind_addr = "0.0.0.0"
	  
	  advertise { 
		http = "%s"
		rpc  = "%s" 
		serf = "%s"
	  }

	  data_dir = "/nomad/data/"
	  consul { 
		address = "%s"
	  }	  
	  vault {
		enabled = true
		address = "%s"
		token   = "%s"
	  }	  
	`
	nomadConfig = fmt.Sprintf(nomadConfig, nodeName, nodeName, nodeName, config.ConsulAddr, config.VaultAddr, config.VaultToken)

	volName := fmt.Sprintf("%s-nomad-client-vol-%d", config.ClusterName, config.Id)
	runtime.CreateVolume(ctx, volName, map[string]string{
		constants.ClusterName: config.ClusterName,
		constants.VolumeType:  constants.NomadClient,
		constants.NodeName:    constants.NodeName,
	})

	ctn, err := runtime.RunNode(ctx, runtimes.NodeConfig{
		Name:        nodeName,
		Image:       nomadClientImage,
		NetworkName: config.NetworkName,
		Cmd:         []string{"agent"},
		Env:         []string{fmt.Sprintf("NOMAD_LOCAL_CONFIG=%s", nomadConfig)},
		Privileged:  true,
		TmpFs: []string{
			"/var/run",
			"/run",
		},
		Volumes: []*runtimes.Volume{
			{
				Name:   volName,
				Dest:   "/nomad/data",
				IsBind: false,
			},
		},
		Labels: map[string]string{
			constants.NodeType:    constants.NomadClient,
			constants.ClusterName: config.ClusterName,
		},
		ExtraCerts: config.ExtraCerts,
	})

	if err != nil {
		return nil, err
	}

	return ctn, nil
}
