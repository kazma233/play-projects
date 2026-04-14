package cmd

import (
	"fmt"
	"log"

	"deploygo/internal/config"
	"deploygo/internal/fileutil"

	"github.com/spf13/cobra"
)

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available projects",
	Long:  `List all available deployment projects from workspace directory`,
	Run: func(cmd *cobra.Command, args []string) {
		workDir := fileutil.WorkspaceDir
		projects, err := config.LoadConfigInfo(workDir)
		if err != nil {
			log.Fatalf("Failed to load projects: %v", err)
		}

		if len(projects) == 0 {
			fmt.Printf("No projects found in %s/\n", workDir)
			return
		}

		fmt.Println("Available projects:")
		fmt.Println()
		for _, p := range projects {
			fmt.Printf("  - %s\n", p.DirName)
		}
	},
}
