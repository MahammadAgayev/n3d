package cluster

import (
	"n3d/cluster"
	"n3d/runtimes"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewClusterCommand() *cobra.Command {
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
				log.Info("cluster already exist")
				return
			}

			err = cluster.ClusterCreate(cmd.Context(), cluster.ClusterConfig{
				ClusterName: args[0],
			}, runtime)

			if err != nil {
				log.WithError(err).Error("unable to create cluster")
			}
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

			if cl == nil {
				log.Info("cluster doesn't exist")
				return
			}

			err = cluster.ClusterDelete(cmd.Context(), cl, runtime)

			if err != nil {
				log.WithError(err).Error("unable to delete cluster")
			}
		},
	}

	stopCmd := &cobra.Command{
		Use:  "stop NAME",
		Args: cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			runtime := runtimes.SelectedRuntime

			cl, err := cluster.ClusterGet(cmd.Context(), runtime, cluster.ClusterConfig{
				ClusterName: args[0],
			})

			if err != nil {
				log.WithError(err).Error("unable to stop cluster")
				return
			}

			if cl == nil {
				log.Info("cluster doesn't exist")
				return
			}

			err = cluster.ClusterStop(cmd.Context(), cl, runtime)

			if err != nil {
				log.WithError(err).Error("unable to stop cluster")
			}
		},
	}

	startCmd := &cobra.Command{
		Use:  "start NAME",
		Args: cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			runtime := runtimes.SelectedRuntime

			cl, err := cluster.ClusterGet(cmd.Context(), runtime, cluster.ClusterConfig{
				ClusterName: args[0],
			})

			if err != nil {
				log.WithError(err).Error("unable to stop cluster")
				return
			}

			if cl == nil {
				log.Info("cluster doesn't exist")
				return
			}

			err = cluster.ClusterStart(cmd.Context(), cl, runtime)

			if err != nil {
				log.WithError(err).Error("unable to stop cluster")
			}
		},
	}

	cmd.AddCommand(addCmd, destroyCmd, stopCmd, startCmd)

	return cmd
}
