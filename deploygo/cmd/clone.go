package cmd

import (
	"log"

	"deploygo/internal/fileutil"
	"deploygo/internal/git"

	"github.com/spf13/cobra"
)

var CloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone Git repository to source directory",
	Long:  `Clone a Git repository to the project's source directory, replacing any existing content.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectCtx, err := loadSelectedProjectConfig()
		if err != nil {
			log.Fatal(err)
		}
		cfg := projectCtx.Config
		basicPath := projectCtx.ProjectDir

		// 检查是否配置了 clone
		if cfg.Clone == nil || cfg.Clone.URL == "" {
			log.Fatalf("No clone configuration found for project '%s'. Please configure 'clone.url' in config.yaml", projectCtx.Name)
		}

		sourceDir := fileutil.SourceDir(basicPath)

		log.Printf("Project: %s", projectCtx.Name)
		log.Printf("Git URL: %s", cfg.Clone.URL)
		if cfg.Clone.Branch != "" {
			log.Printf("Branch: %s", cfg.Clone.Branch)
		} else {
			log.Printf("Branch: master (default)")
		}
		log.Printf("Target: %s", sourceDir)

		// 执行克隆
		opts := git.CloneOptions{
			URL:       cfg.Clone.URL,
			Branch:    cfg.Clone.Branch,
			TargetDir: sourceDir,
		}

		if err := git.Clone(opts); err != nil {
			log.Fatalf("Failed to clone repository: %v", err)
		}

		log.Println("Clone completed successfully!")
	},
}
