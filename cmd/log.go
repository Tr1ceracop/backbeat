package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"backbeat/internal/config"
	"backbeat/internal/store"

	"github.com/spf13/cobra"
)

var (
	logDate string
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show tracked sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		date := time.Now()
		if logDate != "" {
			var err error
			date, err = time.Parse("2006-01-02", logDate)
			if err != nil {
				return fmt.Errorf("invalid date %q (use YYYY-MM-DD): %w", logDate, err)
			}
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}
		_ = cfg

		// Open store read-only (direct access, no IPC needed)
		dir := os.Getenv("XDG_DATA_HOME")
		if dir == "" {
			home, _ := os.UserHomeDir()
			dir = home + "/.local/share"
		}
		dbPath := dir + "/backbeat/backbeat.db"

		st, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		sessions, err := st.GetSessionsForDate(date)
		if err != nil {
			return fmt.Errorf("get sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Printf("No sessions for %s.\n", date.Format("2006-01-02"))
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "START\tEND\tDURATION\tISSUE\tMEETING\tSYNCED\n")

		var totalSecs int
		for _, s := range sessions {
			start := s.StartTime.Format("15:04")
			end := "..."
			dur := time.Since(s.StartTime)
			if s.EndTime != nil {
				end = s.EndTime.Format("15:04")
				dur = s.EndTime.Sub(s.StartTime)
			}
			totalSecs += int(dur.Seconds())

			issue := "-"
			if s.IssueKey != nil {
				issue = *s.IssueKey
			}

			meeting := ""
			if s.IsMeeting {
				meeting = "yes"
			}

			synced := ""
			if s.Synced {
				synced = "yes"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				start, end, formatDuration(int(dur.Seconds())), issue, meeting, synced)
		}

		fmt.Fprintf(w, "\t\t%s\t\t\t\n", formatDuration(totalSecs))
		w.Flush()

		return nil
	},
}

func init() {
	logCmd.Flags().StringVar(&logDate, "date", "", "date to show (YYYY-MM-DD, default today)")
	rootCmd.AddCommand(logCmd)
}
