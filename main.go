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
				Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.WWgcDC3V5t4LLI9Eo9IlcSZgjLFgc52VbcxQBAsCH7g",
				Usage: "lotus token",
			},
			&cli.StringFlag{
				Name:  "venus-url",
				Value: "/ip4/127.0.0.1/tcp/3453",
				Usage: "venus url",
			},
			&cli.StringFlag{
				Name:  "venus-token",
				Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiYWRtaW4iLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.6MY0durlQKAl6dNn4_MVRTcn1Bd34Ip_3aGXgEJVV2k",
				Usage: "venus token",
			},
			&cli.IntFlag{
				Name:  "start-height",
				Usage: "Start comparing the height of the API",
			},
		},
		Action: cmd.Run,
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
	}
}
