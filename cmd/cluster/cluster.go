package cluster

import (
	"n3d/cluster"
	"n3d/containers"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const statePath = ".n3d/state.json"

func NewClusterCommand() *cobra.Command {
	var clusterName string

	cmd := &cobra.Command{
		Use: "cluster",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	addCmd := &cobra.Command{
		Use: "create NAME",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := containers.NewDockerClient()

			if err != nil {
				log.WithError(err).Error("unable to connect docker")
			}

			ctl, err := cluster.NewDockerCluster(client, cluster.ClusterConfig{
				ClusterName: clusterName,
			})

			if err != nil {
				log.WithError(err).Error("unable to create cluster")
			}

			ctl.Provision(cmd.Context())

			state := cluster.CreateState(ctl.(*cluster.DockerClsuter))

			cluster.WriteState(statePath, state)
		},
	}

	destroyCmd := &cobra.Command{
		Use: "delete NAME",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := containers.NewDockerClient()

			if err != nil {
				log.WithError(err).Error("unable to connect docker")
			}

			ctl, err := cluster.NewDockerCluster(client, cluster.ClusterConfig{
				ClusterName: clusterName,
			})

			if err != nil {
				log.WithError(err).Error("unable to create cluster")
			}

			state, err := cluster.ReadState(statePath)

			if err != nil {
				log.WithError(err).Error("unable to read state")
			}

			cluster.LoadState(ctl.(*cluster.DockerClsuter), *state)

			ctl.Destroy(cmd.Context())
		},
	}

	addCmd.Flags().StringVar(&clusterName, "name", "default", "name for your cluster")
	destroyCmd.Flags().StringVar(&clusterName, "name", "default", "name for your cluster")

	cmd.AddCommand(addCmd, destroyCmd)

	return cmd
}
