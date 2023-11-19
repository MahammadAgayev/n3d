package main

import "n3d/cmd"

type ConsulAcl struct {
	SecretID string `json:"SecretID"`
}

func main() {
	cmd.Execute()
}

// ctx := context.Background()

// client, err := containers.NewDockerClient()

// if err != nil {
// 	log.Panicln(err)
// }

// clusterName := "test"
// networkName := clusterName + "-net"
// client.CreateNetwork(ctx, networkName)

// consul, err := consul.NewConsulServer(ctx, client, consul.ConsulConfiguration{
// 	ClusterName: clusterName,
// 	NetworkName: networkName,
// 	Id:          0,
// })

// if err != nil {
// 	log.Panicln(err)
// }

// _, err = nomad.NewNomadServer(ctx, client, nomad.NomadConfiguration{
// 	NetworkName: networkName,
// 	ClusterName: clusterName,
// 	ConsulAddr:  fmt.Sprintf("%s:8500", consul.Ip),
// 	Id:          0,
// })

// if err != nil {
// 	log.WithError(err).Error("error create nomad server")
// }

// _, err = nomad.NewNomadWorker(ctx, client, nomad.NomadConfiguration{
// 	NetworkName: networkName,
// 	ClusterName: clusterName,
// 	ConsulAddr:  fmt.Sprintf("%s:8500", consul.Ip),
// 	Id:          0,
// })

// if err != nil {
// 	log.WithError(err).Error("error creating nomad worker")
// }

// vault, err := vault.NewVault(ctx, client, vault.VaultConfiguration{
// 	ClusterName: clusterName,
// 	ConsulAddr:  consul.Ip,
// 	Id:          0,
// 	NetworkName: networkName,
// })

// if err != nil {
// 	log.WithError(err).Error("error creating vault")
// }

// log.WithContext(ctx).WithFields(log.Fields{
// 	"UnsealKey": vault.UnsealKey,
// 	"RootToken": vault.RootToken,
// }).Info("started vault.")

// termChan := make(chan os.Signal, 10)
// signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)

// <-termChan
