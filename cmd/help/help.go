package help

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// ShowCommandHelp shows help for a specific command
func ShowCommandHelp(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		ShowGeneralHelp(cmd)
		return
	}

	// Find the command
	targetCmd := cmd
	for _, arg := range args {
		found := false
		for _, subCmd := range targetCmd.Commands() {
			if subCmd.Name() == arg {
				targetCmd = subCmd
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Unknown command: %s\n", strings.Join(args, " "))
			return
		}
	}

	// Show command-specific help
	fmt.Println(targetCmd.Long)
	fmt.Println("\nUsage:")
	fmt.Printf("  %s\n", targetCmd.Use)
	if targetCmd.HasAvailableFlags() {
		fmt.Println("\nFlags:")
		targetCmd.Flags().PrintDefaults()
	}
	if targetCmd.HasAvailableSubCommands() {
		fmt.Println("\nAvailable Commands:")
		for _, subCmd := range targetCmd.Commands() {
			fmt.Printf("  %-15s %s\n", subCmd.Name(), subCmd.Short)
		}
	}
}

// ShowGeneralHelp shows general help information
func ShowGeneralHelp(cmd *cobra.Command) {
	fmt.Println("flowctl - Control plane for data pipelines for the Stellar Blockchain")
	fmt.Println("\nUsage:")
	fmt.Println("  flowctl [command] [TYPE/NAME] [flags]")
	fmt.Println("\nCommon resource types:")
	fmt.Println("  pipelines, sources, processors, sinks, topics, secrets, users")
	fmt.Println("\nAvailable Commands:")

	// Group commands by their first letter
	groups := make(map[string][]*cobra.Command)
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == "help" {
			continue
		}
		firstChar := string(subCmd.Name()[0])
		groups[firstChar] = append(groups[firstChar], subCmd)
	}

	// Print commands in alphabetical order
	for _, group := range groups {
		for _, subCmd := range group {
			fmt.Printf("  %-15s %s\n", subCmd.Name(), subCmd.Short)
		}
	}

	fmt.Println("\nGlobal Flags:")
	cmd.Flags().PrintDefaults()
}
