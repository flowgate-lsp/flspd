package main

import (
	"fmt"
	"os"

	"github.com/flowgate-lsp/flspd/config"
	"gopkg.in/yaml.v3"
)

// loadNodeConfigs reads and parses the nodes configuration the same way
// runLspd does — NODES_JSON if set, otherwise the file at
// NODES_CONFIG_PATH — so the `flspd config` subcommands (T5-6) inspect
// exactly what the daemon itself would load, not a separate code path
// that could drift from it.
func loadNodeConfigs() ([]*config.NodeConfig, error) {
	var nodesContent []byte
	var err error

	if nodesJson := os.Getenv("NODES_JSON"); nodesJson != "" {
		nodesContent = []byte(nodesJson)
	} else {
		nodesConfigPath := os.Getenv("NODES_CONFIG_PATH")
		if nodesConfigPath == "" {
			return nil, fmt.Errorf("NODES_CONFIG_PATH environment variable is not set")
		}

		nodesContent, err = os.ReadFile(nodesConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read nodes config file: %w", err)
		}
	}

	var nodeConfigs []*config.NodeConfig
	if err := yaml.Unmarshal(nodesContent, &nodeConfigs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes config: %w", err)
	}

	if len(nodeConfigs) == 0 {
		return nil, fmt.Errorf("need at least one node configured in nodes config file")
	}

	return nodeConfigs, nil
}
