package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/agstrc/qlp/qlp"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "QLP Parser",
		Usage:       "Parses game data from a file and outputs it in JSON format.",
		UsageText:   "qlp-parser [command options] [file]",
		Description: "This program takes a file path as an argument, parses the game data contained within, and outputs the data in a nicely formatted JSON structure.",
		ArgsUsage:   "[file]",
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				cli.ShowAppHelpAndExit(c, 1)
			}

			filePath := c.Args().Get(0)
			file, err := os.Open(filePath)
			if err != nil {
				return cli.Exit(fmt.Sprintf("Failed to open file: %s", err), 2)
			}
			defer file.Close()

			games, err := qlp.ParseLog(file)
			if err != nil {
				return fmt.Errorf("Failed to parse file: %s", err)
			}

			jsonOutput, err := json.MarshalIndent(games, "", "  ")
			if err != nil {
				return cli.Exit(fmt.Sprintf("Failed to marshal game data: %s", err), 4)
			}
			os.Stdout.Write(jsonOutput)
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
