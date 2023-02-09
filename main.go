package main

import (
	"fmt"
	"os"

	"github.com/simlecode/api-compare/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "Compare the apis of venus and lotus",
		Usage: "",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "lotus-url",
				Value: "/ip4/127.0.0.1/tcp/1234",
				Usage: "lotus url",
			},
			&cli.StringFlag{
				Name:  "lotus-token",
				Value: "",
				Usage: "lotus token",
			},
			&cli.StringFlag{
				Name:  "venus-url",
				Value: "/ip4/127.0.0.1/tcp/3453",
				Usage: "venus url",
			},
			&cli.StringFlag{
				Name:  "venus-token",
				Value: "",
				Usage: "venus token",
			},
			&cli.IntFlag{
				Name:  "start-height",
				Usage: "Start comparing the height of the API",
			},
			&cli.IntFlag{
				Name:  "concurrency",
				Value: 2,
			},
		},
		Action: cmd.Run,
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
	}
}
