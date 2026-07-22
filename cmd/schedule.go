package cmd

import (
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
	Run: func(cmd *cobra.Command, args []string) {
		s, err := schedule.GetScheduler()
		if err != nil {
			fmt.Println(err)
			return
		}
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			fmt.Println(err)
			return
		}
		status, _ := s.Status(task)
		fmt.Printf("Schedule status (%s): %s\n", task, status)
	},
}

var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable a scheduled maintenance task",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := schedule.GetScheduler()
		if err != nil {
			fmt.Println(err)
			return
		}

		rawInterval, _ := cmd.Flags().GetString("interval")
		interval, err := schedule.ParseInterval(rawInterval)
		if err != nil {
			fmt.Println(err)
			return
		}
		hourStr, _ := cmd.Flags().GetString("hour")
		hour, err := schedule.ParseHour(hourStr)
		if err != nil {
			fmt.Println(err)
			return
		}
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			fmt.Println(err)
			return
		}

		if err := s.Enable(task, interval, hour); err != nil {
			fmt.Printf("Failed to enable schedule: %v\n", err)
			return
		}
		fmt.Printf("Schedule enabled for %s (%s)\n", task, schedule.FormatSchedule(interval, hour))
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable a scheduled maintenance task",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := schedule.GetScheduler()
		if err != nil {
			fmt.Println(err)
			return
		}
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			fmt.Println(err)
			return
		}

		if err := s.Disable(task); err != nil {
			fmt.Printf("Failed to disable schedule: %v\n", err)
			return
		}
		fmt.Printf("Schedule disabled for %s\n", task)
	},
}

var scheduleConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or update saved session-refresh schedule config",
	Run: func(cmd *cobra.Command, args []string) {
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			fmt.Println(err)
			return
		}
		if task != schedule.SessionRefreshTask {
			fmt.Println("schedule config currently supports session-refresh only")
			return
		}

		s, err := schedule.GetScheduler()
		if err != nil {
			fmt.Println(err)
			return
		}

		enabled, interval, hour := sessionRefreshScheduleConfig()
		status, _ := s.Status(task)
		changed := cmd.Flags().Changed("enabled") || cmd.Flags().Changed("interval") || cmd.Flags().Changed("hour")

		if !changed {
			fmt.Printf("Schedule config (%s): enabled=%v interval=%s hour=%02d status=%s\n", task, enabled, interval, hour, status)
			return
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
				fmt.Println(err)
				return
			}
		}

		if cmd.Flags().Changed("hour") {
			hourStr, _ := cmd.Flags().GetString("hour")
			hour, err = schedule.ParseHour(hourStr)
			if err != nil {
				fmt.Println(err)
				return
			}
		}

		viper.Set("session_refresh_enabled", enabled)
		viper.Set("session_refresh_interval", interval)
		viper.Set("session_refresh_hour", hour)
		if err := persistViperConfig(); err != nil {
			fmt.Printf("failed to write config: %v\n", err)
			return
		}

		if enabled {
			if err := s.Enable(task, interval, hour); err != nil {
				fmt.Printf("Failed to enable schedule: %v\n", err)
				return
			}
			fmt.Printf("Schedule config updated and enabled for %s (%s)\n", task, schedule.FormatSchedule(interval, hour))
			return
		}
		if explicitDisable {
			if err := s.Disable(task); err != nil {
				fmt.Printf("Failed to disable schedule: %v\n", err)
				return
			}
			fmt.Printf("Schedule config updated and disabled for %s\n", task)
			return
		}

		fmt.Printf("Schedule config updated for %s (%s, not enabled)\n", task, schedule.FormatSchedule(interval, hour))
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
