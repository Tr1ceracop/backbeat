package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"backbeat/internal/ipc"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current tracking status",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ipc.SendCommand(ipc.Request{Command: "status"})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("status failed: %s", resp.Error)
		}

		var data ipc.StatusData
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		fmt.Printf("State:   %s\n", data.State)
		if data.IssueKey != "" {
			fmt.Printf("Issue:   %s\n", data.IssueKey)
		}
		if data.SessionStart != "" {
			if t, err := time.Parse(time.RFC3339, data.SessionStart); err == nil {
				fmt.Printf("Since:   %s\n", t.Format("15:04"))
			}
		}
		if data.IsMeeting {
			fmt.Println("Meeting: yes")
		}
		fmt.Printf("Today:   %s active", formatDuration(data.TodayTotal))
		if data.TodayMeeting > 0 {
			fmt.Printf(", %s meetings", formatDuration(data.TodayMeeting))
		}
		fmt.Println()

		return nil
	},
}

func formatDuration(secs int) string {
	h := secs / 3600
	m := (secs % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
