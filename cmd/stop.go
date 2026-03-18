package cmd

import (
	"fmt"

	"backbeat/internal/ipc"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the backbeat daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ipc.SendCommand(ipc.Request{Command: "stop"})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("stop failed: %s", resp.Error)
		}
		fmt.Println("Daemon stopped.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
