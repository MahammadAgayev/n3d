package cmd

import (
	"log"
	"n3d/cmd/cluster"
	"n3d/runtimes"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "N3D",
		Short: "N3D will be neat tool for local nomad env",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Usage(); err != nil {
				log.Fatalln(err)
			}
		},
	}

	cobra.OnInitialize(initRuntime)

	rootCmd.AddCommand(cluster.NewClusterCommand())

	return rootCmd
}

func initRuntime() {
	runtimes.SetDockerRuntime()
}
