package cluster

import (
	"n3d/cluster"
	"n3d/runtimes"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewClusterCommand() *cobra.Command {
	var clusterName string

	cmd := &cobra.Command{
		Use: "cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Help(); err != nil {
				log.Error("Couldn't get help text")
				log.Fatalln(err)
			}
		},
	}

	addCmd := &cobra.Command{
		Use:  "create NAME",
		Args: cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			runtime := runtimes.SelectedRuntime

			cl, err := cluster.ClusterGet(cmd.Context(), runtime, cluster.ClusterConfig{
				ClusterName: args[0],
			})

			if err != nil {
				log.WithError(err).Error("unable to fetch cluster")
			}

			if cl != nil {
				log.Info("cluster already exists")
			}

			cluster.ClusterCreate(cmd.Context(), cluster.ClusterConfig{
				ClusterName: clusterName,
			}, runtime)
		},
	}

	destroyCmd := &cobra.Command{
		Use:  "delete NAME",
		Args: cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			runtime := runtimes.SelectedRuntime

			cl, err := cluster.ClusterGet(cmd.Context(), runtime, cluster.ClusterConfig{
				ClusterName: args[0],
			})

			if err != nil {
				log.WithError(err).Error("unable to delete cluster")
				return
			}

			err = cluster.ClusterDelete(cmd.Context(), cl, runtime)

			if err != nil {
				log.WithError(err).Error("unable to delete cluster")
			}
		},
	}

	cmd.AddCommand(addCmd, destroyCmd)

	return cmd
}
