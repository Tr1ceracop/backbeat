package cmd

import (
	"os"

	"backbeat/internal/config"
	"backbeat/internal/daemon"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the backbeat daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
			With().Timestamp().Logger()

		d, err := daemon.New(cfg, logger)
		if err != nil {
			return err
		}

		return d.Run()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
