package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-tools/internal/schedule"
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

		interval, _ := cmd.Flags().GetString("interval")
		hourStr, _ := cmd.Flags().GetString("hour")
		hour, _ := strconv.Atoi(hourStr)
		task, err := selectedScheduleTask(cmd)
		if err != nil {
			fmt.Println(err)
			return
		}

		if err := s.Enable(task, interval, hour); err != nil {
			fmt.Printf("Failed to enable schedule: %v\n", err)
			return
		}
		fmt.Printf("Schedule enabled for %s (%s, %02d:00)\n", task, interval, hour)
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

func selectedScheduleTask(cmd *cobra.Command) (schedule.Task, error) {
	raw, _ := cmd.Flags().GetString("task")
	return schedule.ParseTask(raw)
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(enableCmd)
	scheduleCmd.AddCommand(disableCmd)

	enableCmd.Flags().String("interval", "daily", "Update interval (daily or weekly)")
	enableCmd.Flags().String("hour", "9", "Hour of the day (0-23)")
	for _, c := range []*cobra.Command{scheduleCmd, enableCmd, disableCmd} {
		c.Flags().String("task", string(schedule.AgentUpdateTask), "Scheduled task (agent-update or session-refresh)")
	}
}
