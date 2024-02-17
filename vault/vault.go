package vault

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"n3d/constants"
	"n3d/runtimes"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const vaultImage = "vault:1.13.3"

type VaultConfiguration struct {
	ClusterName string
	ConsulAddr  string
	NetworkName string
	Id          int
}

type VaultNode struct {
	Node      *runtimes.Node
	UnsealKey string
	RootToken string
}

type vaultInitResponse struct {
	UnsealKeys []string `json:"unseal_keys_b64"`
	RootToken  string   `json:"root_token"`
}

func NewVault(ctx context.Context, runtime runtimes.Runtime, config VaultConfiguration) (*VaultNode, error) {
	nodeName := fmt.Sprintf("%s-vault-%d", config.ClusterName, config.Id)
	vaultConfig := `
	    ui            = true
	    log_level     = "trace"
		cluster_addr  = "http://127.0.0.1:8201"
        api_addr      = "http://127.0.0.1:8200"
		cluster_name  = "%s"

		storage "consul" {
			address = "%s"
			path = "vault/"
		}
		listener "tcp" {
			address = "0.0.0.0:8200"
			cluster_address  = "0.0.0.0:8201"
			tls_disable = 1
		}
		
		max_lease_ttl = "9000h"
		default_lease_ttl = "10h"
		ui = true		
	`

	vaultConfig = fmt.Sprintf(vaultConfig, config.ClusterName, config.ConsulAddr)

	ctn, err := runtime.RunNode(ctx, runtimes.NodeConfig{
		Name:        nodeName,
		Image:       vaultImage,
		NetworkName: config.NetworkName,
		Privileged:  true,
		Cmd:         []string{"server"},
		Files: []*runtimes.FileInNode{
			{
				Content:  []byte(vaultConfig),
				Path:     "/vault/config/vault.hcl",
				FileMode: 0644,
			},
		},
		Labels: map[string]string{
			constants.NodeType:    constants.Vault,
			constants.ClusterName: config.ClusterName,
		},
	})

	if err != nil {
		return nil, err
	}

	err = waitForVault(ctx, runtime, ctn)

	if err != nil {
		return nil, errors.Join(errors.New("unable to check vault status"), err)
	}

	cmd := []string{"vault", "operator", "init", "-key-shares=1", "-key-threshold=1", "-format=json", "-address=http://127.0.0.1:8200"}
	//cmd := []string{"ls", "-l"}

	respText, err := runtime.Exec(ctx, ctn, cmd)

	if err != nil {
		return nil, errors.Join(errors.New("unable to initialize vault"), err)
	}

	respObj := &vaultInitResponse{}
	err = json.Unmarshal([]byte(*respText), respObj)

	if err != nil {
		log.Info(*respText)
		return nil, errors.Join(fmt.Errorf("unable to parse vault response: %s", *respText), err)
	}

	return &VaultNode{Node: ctn, UnsealKey: respObj.UnsealKeys[0], RootToken: respObj.RootToken}, nil
}

func waitForVault(ctx context.Context, runtime runtimes.Runtime, container *runtimes.Node) error {
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*30)
	defer cancelFunc()

	logs, err := runtime.Logs(timeoutCtx, container.Name, true)

	if err != nil {
		return err
	}

	defer logs.Close()

	scanner := bufio.NewScanner(logs)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "core: Initializing version history cache for core") {
			break
		}
	}

	return nil
}
