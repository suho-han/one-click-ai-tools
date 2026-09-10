package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/schedule"
)

var scheduleCmd = &cobra.Command{
	Use:     "schedule",
	GroupID: "manage",
	Short:   "Manage scheduled maintenance tasks",
	Long: `Show, enable, or disable scheduled maintenance tasks (agent-update,
session-refresh) via the platform scheduler (launchd/cron/SchTasks).`,
	Example: `  oct schedule --task agent-update                         show status
  oct schedule enable --task agent-update --interval daily --hour 9
  oct schedule enable --task session-refresh --interval 6h
  oct schedule disable --task agent-update
  oct schedule config --interval 12h --hour 8              saved session-refresh config`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := schedule.GetScheduler()
		if err != nil {
			return fmt.Errorf("scheduler unavailable: %w", err)
		}
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			return fmt.Errorf("invalid task: %w", err)
		}
		status, err := s.Status(task)
		if err != nil {
			return fmt.Errorf("failed to read schedule status: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Schedule status (%s): %s\n", task, status)
		return nil
	},
}

var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable a scheduled maintenance task",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := schedule.GetScheduler()
		if err != nil {
			return fmt.Errorf("scheduler unavailable: %w", err)
		}

		rawInterval, _ := cmd.Flags().GetString("interval")
		interval, err := schedule.ParseInterval(rawInterval)
		if err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}
		hourStr, _ := cmd.Flags().GetString("hour")
		hour, err := schedule.ParseHour(hourStr)
		if err != nil {
			return fmt.Errorf("invalid hour: %w", err)
		}
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			return fmt.Errorf("invalid task: %w", err)
		}

		if err := s.Enable(task, interval, hour); err != nil {
			return fmt.Errorf("failed to enable schedule: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Schedule enabled for %s (%s)\n", task, schedule.FormatSchedule(interval, hour))
		return nil
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable a scheduled maintenance task",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := schedule.GetScheduler()
		if err != nil {
			return fmt.Errorf("scheduler unavailable: %w", err)
		}
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			return fmt.Errorf("invalid task: %w", err)
		}

		if err := s.Disable(task); err != nil {
			return fmt.Errorf("failed to disable schedule: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Schedule disabled for %s\n", task)
		return nil
	},
}

var scheduleConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or update saved session-refresh schedule config",
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			return fmt.Errorf("invalid task: %w", err)
		}
		if task != schedule.SessionRefreshTask {
			return errors.New("schedule config currently supports session-refresh only")
		}

		s, err := schedule.GetScheduler()
		if err != nil {
			return fmt.Errorf("scheduler unavailable: %w", err)
		}

		enabled, interval, hour := sessionRefreshScheduleConfig()
		status, err := s.Status(task)
		if err != nil {
			return fmt.Errorf("failed to read schedule status: %w", err)
		}
		changed := cmd.Flags().Changed("enabled") || cmd.Flags().Changed("interval") || cmd.Flags().Changed("hour")

		if !changed {
			fmt.Fprintf(cmd.OutOrStdout(), "Schedule config (%s): enabled=%v interval=%s hour=%02d status=%s\n", task, enabled, interval, hour, status)
			return nil
		}

		explicitDisable := false
		if cmd.Flags().Changed("enabled") {
			enabled, _ = cmd.Flags().GetBool("enabled")
			explicitDisable = !enabled
		} else if strings.EqualFold(status, "enabled") {
			enabled = true
		}

		if cmd.Flags().Changed("interval") {
			rawInterval, _ := cmd.Flags().GetString("interval")
			interval, err = schedule.ParseInterval(rawInterval)
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}
		}

		if cmd.Flags().Changed("hour") {
			hourStr, _ := cmd.Flags().GetString("hour")
			hour, err = schedule.ParseHour(hourStr)
			if err != nil {
				return fmt.Errorf("invalid hour: %w", err)
			}
		}

		viper.Set("session_refresh_enabled", enabled)
		viper.Set("session_refresh_interval", interval)
		viper.Set("session_refresh_hour", hour)
		if err := persistViperConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		if enabled {
			if err := s.Enable(task, interval, hour); err != nil {
				return fmt.Errorf("failed to enable schedule: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Schedule config updated and enabled for %s (%s)\n", task, schedule.FormatSchedule(interval, hour))
			return nil
		}
		if explicitDisable {
			if err := s.Disable(task); err != nil {
				return fmt.Errorf("failed to disable schedule: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Schedule config updated and disabled for %s\n", task)
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Schedule config updated for %s (%s, not enabled)\n", task, schedule.FormatSchedule(interval, hour))
		return nil
	},
}

func selectedScheduleTask(cmd *cobra.Command) (schedule.Task, error) {
	raw, _ := cmd.Flags().GetString("task")
	return schedule.ParseTask(raw)
}

func sessionRefreshScheduleConfig() (bool, string, int) {
	enabled := viper.GetBool("session_refresh_enabled")
	interval, err := schedule.ParseInterval(viper.GetString("session_refresh_interval"))
	if err != nil {
		interval = schedule.DailyInterval
	}
	hour := viper.GetInt("session_refresh_hour")
	if hour < 0 || hour > 23 {
		hour = 9
	}
	return enabled, interval, hour
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(enableCmd)
	scheduleCmd.AddCommand(disableCmd)
	scheduleCmd.AddCommand(scheduleConfigCmd)

	enableCmd.Flags().String("interval", "daily", "Update interval (weekly, daily, 12h, 6h, or 1h)")
	enableCmd.Flags().String("hour", "9", "Hour of the day (0-23)")
	for _, c := range []*cobra.Command{scheduleCmd, enableCmd, disableCmd} {
		c.Flags().String("task", string(schedule.AgentUpdateTask), "Scheduled task (agent-update or session-refresh)")
	}
	scheduleConfigCmd.Flags().String("task", string(schedule.SessionRefreshTask), "Scheduled task (session-refresh)")
	scheduleConfigCmd.Flags().Bool("enabled", false, "Enable or disable the saved schedule")
	scheduleConfigCmd.Flags().String("interval", "", "Saved interval (weekly, daily, 12h, 6h, or 1h)")
	scheduleConfigCmd.Flags().String("hour", "", "Hour of the day for weekly/daily schedules (0-23)")
}
