package main

import (
	"fmt"

	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/urfave/cli"
)

var genKeyCommand = cli.Command{
	Name:   "genkey",
	Usage:  "Generate an flspd private key for the FLSPD_PRIVATE_KEY environment variable.",
	Action: genKey,
}

func genKey(cliCtx *cli.Context) error {
	p, err := btcec.NewPrivateKey()
	if err != nil {
		return fmt.Errorf("btcec.NewPrivateKey() error: %w", err)
	}
	fmt.Printf("FLSPD_PRIVATE_KEY=\"%x\"\n", p.Serialize())
	return nil
}
