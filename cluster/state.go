package cluster

import (
	"encoding/json"
	"n3d/containers"
	"n3d/vault"
	"os"
)

type clusterState struct {
	ClusterName  string                       `json:"clusterName"`
	Network      *containers.ContainerNetwork `json:"network"`
	Consul       *containers.Container
	NomadServer  *containers.Container
	NomadWorkers []containers.Container
	Vault        *vault.VaultContainer
}

func CreateState(c *DockerClsuter) *clusterState {
	clusterState := &clusterState{
		ClusterName:  c.config.ClusterName,
		Network:      c.network,
		Consul:       c.consul,
		NomadServer:  c.nomadServer,
		Vault:        c.vault,
		NomadWorkers: c.nomadWorkers,
	}

	return clusterState
}

func LoadState(cluster *DockerClsuter, state *clusterState) error {
	cluster.config = ClusterConfig{
		ClusterName: state.ClusterName,
	}

	cluster.network = state.Network
	cluster.consul = state.Consul
	cluster.vault = state.Vault
	cluster.nomadServer = state.NomadServer
	cluster.nomadWorkers = state.NomadWorkers

	return nil
}

func ReadState(path string) (*clusterState, error) {
	reader, err := os.Open(path)

	if err != nil {
		return nil, err
	}

	defer reader.Close()

	var state clusterState

	decoder := json.NewDecoder(reader)

	decoder.Decode(&state)

	return &state, nil
}

func WriteState(path string, state *clusterState) error {
	writer, err := os.Create(path)

	if err != nil {
		return err
	}

	defer writer.Close()

	bytes, err := json.Marshal(state)

	if err != nil {
		return err
	}

	writer.WriteString(string(bytes))
	writer.Sync()

	return nil
}
