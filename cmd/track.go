package cmd

import (
	"encoding/json"
	"fmt"

	"backbeat/internal/ipc"

	"github.com/spf13/cobra"
)

var trackCmd = &cobra.Command{
	Use:   "track ISSUE-KEY",
	Short: "Set the active Jira issue to track time against",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		trackArgs := ipc.TrackArgs{IssueKey: args[0]}
		raw, _ := json.Marshal(trackArgs)

		resp, err := ipc.SendCommand(ipc.Request{
			Command: "track",
			Args:    raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("track failed: %s", resp.Error)
		}

		fmt.Printf("Now tracking: %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(trackCmd)
}
