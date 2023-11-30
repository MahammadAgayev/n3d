package vault

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"n3d/constants"
	"n3d/containers"
	"os"
	"regexp"
	"strings"
	"time"
)

const vaultImage = "vault:1.13.3"

type VaultConfiguration struct {
	ClusterName string
	ConsulAddr  string
	NetworkName string
	Id          int
}

type VaultNode struct {
	Node      *containers.Node
	UnsealKey string
	RootToken string
}

func NewVault(ctx context.Context, runtime containers.Runtime, config VaultConfiguration) (*VaultNode, error) {
	vaultConfig := `
	    ui            = true
	    log_level     = "trace"
		cluster_addr  = "http://127.0.0.1:8201"
        api_addr      = "http://127.0.0.1:8200"
		cluster_name  = "%s"

		storage "consul" {
			address = "%s:8500"
			path = "vault/"
		}
	`

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("n3d-vault-%s-*.conf", config.ClusterName))

	if err != nil {
		return nil, errors.Join(err, errors.New("unable to create temp file for vault config"))
	}

	tmpFile.WriteString(fmt.Sprintf(vaultConfig, config.ClusterName, config.ConsulAddr))

	//close file as we don't need from here on
	tmpFile.Close()

	ctn, err := runtime.RunContainer(ctx, containers.NodeConfig{
		Name:        fmt.Sprintf("%s-vault-%d", config.ClusterName, config.Id),
		Image:       vaultImage,
		NetworkName: config.NetworkName,
		Privileged:  true,
		Cmd:         []string{"server", "-dev", "-config=/vault/config/vault.hcl"},
		Ports:       []string{"8200/tcp:8200"},
		VolumeBinds: []string{
			fmt.Sprintf("%s:/vault/config/vault.hcl", tmpFile.Name()),
		},
		Labels: map[string]string{
			constants.NodeType:    constants.Vault,
			constants.ClusterName: config.ClusterName,
		},
	})

	if err != nil {
		return nil, err
	}

	unsealKey, rootToken, err := getVaultCreds(ctx, runtime, ctn)

	if err != nil {
		return nil, errors.Join(err, errors.New("unable to fetch vault creds(unseal key, rootToken) from vault"))
	}

	return &VaultNode{Node: ctn, UnsealKey: unsealKey, RootToken: rootToken}, nil
}

func getVaultCreds(ctx context.Context, cli containers.Runtime, container *containers.Node) (string, string, error) {
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*30)
	defer cancelFunc()

	logs, err := cli.Logs(timeoutCtx, container.Name, true)

	if err != nil {
		return "", "", err
	}

	defer logs.Close()

	scanner := bufio.NewScanner(logs)

	unsealKeyRegex := regexp.MustCompile(`Unseal Key: (.*)`)
	rootTokenRegex := regexp.MustCompile(`Root Token: (.*)`)

	var unsealKey string
	var rootToken string

	for scanner.Scan() {
		line := scanner.Text()

		unsealKeyMatches := unsealKeyRegex.FindStringSubmatch(line)
		rootTokenMatches := rootTokenRegex.FindStringSubmatch(line)

		if len(unsealKeyMatches) > 1 {
			unsealKey = strings.Replace(unsealKeyMatches[1], "\x1b[0m", "", 1)
		}

		if len(rootTokenMatches) > 1 {
			rootToken = strings.Replace(rootTokenMatches[1], "\x1b[0m", "", 1)
		}

		if unsealKey != "" && rootToken != "" {
			break
		}
	}

	return unsealKey, rootToken, nil
}
