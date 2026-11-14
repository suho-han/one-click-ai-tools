package cmd

import "testing"

func TestMenubarRestartRegistered(t *testing.T) {
	c, _, err := menubarCmd.Find([]string{"restart"})
	if err != nil || c == nil || c.Name() != "restart" {
		t.Fatalf("menubar restart not registered: %v", err)
	}
}

func TestUpdateNoRestartFlagDefault(t *testing.T) {
	f := updateCmd.Flags().Lookup("no-restart")
	if f == nil || f.DefValue != "false" {
		t.Fatalf("--no-restart flag missing or wrong default: %+v", f)
	}
}
