package cmd

import (
	"encoding/json"
	"fmt"

	"backbeat/internal/ipc"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync tracked time to Jira Tempo",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ipc.SendCommand(ipc.Request{Command: "sync"})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("sync failed: %s", resp.Error)
		}

		var data ipc.SyncData
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if data.WorklogsCreated == 0 {
			fmt.Println("Nothing to sync.")
		} else {
			fmt.Printf("Created %d worklog(s) in Tempo.\n", data.WorklogsCreated)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
