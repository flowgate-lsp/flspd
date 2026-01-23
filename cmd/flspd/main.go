package main

import (
	"os"

	"github.com/rs/zerolog/log"

	"github.com/flowgate-lsp/flspd/build"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "flspd"
	app.Version = build.GetTag() + " commit=" + build.GetRevision()
	app.Usage = "LSP implementation for LND"
	app.Action = runLspd
	app.Commands = []cli.Command{
		genKeyCommand,
		migrateCommand,
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal().Msgf("Failed to run app: %v", err)
	}
}
