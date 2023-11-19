package cmd

import (
	"n3d/cmd/cluster"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "N3D",
	Short: "N3D will be neat tool for local nomad env",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(cluster.NewClusterCommand())
}
