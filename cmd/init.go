package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"backbeat/internal/config"
	"backbeat/internal/jira"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize backbeat configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.DefaultPath()
		if cfgFile != "" {
			path = cfgFile
		}

		// Check if config already exists
		if _, err := os.Stat(path); err == nil {
			overwrite, err := prompt("Config already exists at %s. Overwrite? [y/N]", path)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(strings.ToLower(overwrite), "y") {
				fmt.Println("Aborted.")
				return nil
			}
		}

		cfg := config.DefaultConfig()

		fmt.Println("Backbeat setup")
		fmt.Println()

		// Jira
		fmt.Println("-- Jira Cloud --")
		val, err := prompt("Jira base URL (e.g. https://yourcompany.atlassian.net):")
		if err != nil {
			return err
		}
		cfg.Jira.BaseURL = ensureHTTPS(strings.TrimRight(val, "/"))

		val, err = prompt("Jira email:")
		if err != nil {
			return err
		}
		cfg.Jira.Email = val

		val, err = prompt("Jira API token (https://id.atlassian.com/manage-profile/security/api-tokens):")
		if err != nil {
			return err
		}
		cfg.Jira.APIToken = val

		// Auto-fetch account ID
		fmt.Print("\nFetching your Atlassian account ID... ")
		jiraClient := jira.NewClient(cfg.Jira)
		accountID, displayName, err := jiraClient.GetMyself()
		if err != nil {
			fmt.Printf("failed: %v\n", err)
			val, err = prompt("Enter your Atlassian account ID manually:")
			if err != nil {
				return err
			}
			accountID = val
		} else {
			fmt.Printf("OK (%s)\n", displayName)
		}
		cfg.Tempo.AccountID = accountID

		fmt.Println()

		// Tempo
		fmt.Println("-- Tempo --")
		val, err = prompt("Tempo API token (Tempo > Settings > API Integration):")
		if err != nil {
			return err
		}
		cfg.Tempo.APIToken = val

		val, err = promptDefault("Tempo base URL:", cfg.Tempo.BaseURL)
		if err != nil {
			return err
		}
		cfg.Tempo.BaseURL = ensureHTTPS(strings.TrimRight(val, "/"))

		fmt.Println()

		// Tracking defaults
		fmt.Println("-- Tracking (press Enter for defaults) --")
		for _, dur := range []struct {
			label string
			dest  *config.Duration
		}{
			{"Idle threshold:", &cfg.Tracking.IdleThreshold},
			{"Poll interval:", &cfg.Tracking.PollInterval},
			{"Minimum session duration:", &cfg.Tracking.MinSession},
		} {
			val, err = promptDefault(dur.label, dur.dest.Duration.String())
			if err != nil {
				return err
			}
			if err := dur.dest.UnmarshalText([]byte(val)); err != nil {
				return fmt.Errorf("invalid duration %q: %w", val, err)
			}
		}

		// Save
		if err := cfg.Save(path); err != nil {
			return err
		}

		fmt.Printf("\nConfig written to %s\n", path)
		fmt.Println("Run 'backbeat start' to begin tracking.")
		return nil
	},
}

func ensureHTTPS(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "https://" + url
	}
	return url
}

func prompt(format string, args ...any) (string, error) {
	fmt.Printf(format+" ", args...)
	reader := bufio.NewReader(os.Stdin)
	val, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(val), nil
}

func promptDefault(label, defaultVal string) (string, error) {
	fmt.Printf("%s [%s] ", label, defaultVal)
	reader := bufio.NewReader(os.Stdin)
	val, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return defaultVal, nil
	}
	return val, nil
}

func init() {
	rootCmd.AddCommand(initCmd)
}
