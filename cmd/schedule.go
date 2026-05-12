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
	Short: "Manage update schedule",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := schedule.GetScheduler()
		if err != nil {
			fmt.Println(err)
			return
		}
		status, _ := s.Status()
		fmt.Printf("Schedule status: %s\n", status)
	},
}

var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable scheduled updates",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := schedule.GetScheduler()
		if err != nil {
			fmt.Println(err)
			return
		}

		interval, _ := cmd.Flags().GetString("interval")
		hourStr, _ := cmd.Flags().GetString("hour")
		hour, _ := strconv.Atoi(hourStr)

		if err := s.Enable(interval, hour); err != nil {
			fmt.Printf("Failed to enable schedule: %v\n", err)
			return
		}
		fmt.Printf("Schedule enabled (%s, %02d:00)\n", interval, hour)
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable scheduled updates",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := schedule.GetScheduler()
		if err != nil {
			fmt.Println(err)
			return
		}

		if err := s.Disable(); err != nil {
			fmt.Printf("Failed to disable schedule: %v\n", err)
			return
		}
		fmt.Println("Schedule disabled")
	},
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(enableCmd)
	scheduleCmd.AddCommand(disableCmd)

	enableCmd.Flags().String("interval", "daily", "Update interval (daily or weekly)")
	enableCmd.Flags().String("hour", "9", "Hour of the day (0-23)")
}
