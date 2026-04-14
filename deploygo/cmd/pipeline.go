package cmd

import (
	"log"
	"os"

	"deploygo/internal/container"
	"deploygo/internal/fileutil"
	"deploygo/internal/git"
	"deploygo/internal/stage"

	"github.com/spf13/cobra"
)

var PipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Run build and deploy pipeline",
	Long:  `Execute all build stages followed by deployment steps`,
	Run: func(cmd *cobra.Command, args []string) {
		projectCtx, err := loadSelectedProjectConfig()
		if err != nil {
			log.Fatal(err)
		}
		cfg := projectCtx.Config
		basicPath := projectCtx.ProjectDir

		containerMgr, err := container.NewManager(&container.ManagerConfig{
			Type: cfg.Container.Type,
		})
		if err != nil {
			log.Fatalf("Failed to initialize container runtime: %v", err)
		}
		defer containerMgr.Close()

		log.Printf("Using container runtime: %s", containerMgr.Name())
		log.Printf("Project: %s", projectCtx.Name)
		log.Printf("Project directory: %s", basicPath)

		projectDir := projectCtx.ProjectDir
		overlaysDir := fileutil.OverlaysDir(projectDir)
		sourceDir := fileutil.SourceDir(projectDir)

		// 执行 Git 克隆（如果配置了）
		if cfg.Clone != nil && cfg.Clone.URL != "" {
			log.Println("=== Cloning Git Repository ===")
			log.Printf("Git URL: %s", cfg.Clone.URL)
			if cfg.Clone.Branch != "" {
				log.Printf("Branch: %s", cfg.Clone.Branch)
			}
			opts := git.CloneOptions{
				URL:       cfg.Clone.URL,
				Branch:    cfg.Clone.Branch,
				TargetDir: sourceDir,
			}
			if err := git.Clone(opts); err != nil {
				log.Fatalf("Failed to clone repository: %v", err)
			}
		}

		if _, err := os.Stat(overlaysDir); err == nil {
			log.Println("=== Copying Overlays ===")
			if err := copyOverlays(overlaysDir, sourceDir); err != nil {
				log.Fatalf("Failed to copy overlays: %v", err)
			}
		}

		if len(cfg.Builds) > 0 {
			log.Println("=== Building ===")
			if err := stage.RunBuilds(containerMgr, cfg.Builds, basicPath); err != nil {
				log.Fatalf("Failed to build stages: %v", err)
			}
		}

		if len(cfg.Deploys) > 0 {
			log.Println("=== Deploying ===")
			if err := stage.RunDeploys(cfg, cfg.Deploys, basicPath); err != nil {
				log.Fatalf("Failed to deploy: %v", err)
			}
		}

		if cfg.Cleanup != nil {
			log.Println("=== Running Cleanup Tasks ===")
			if err := stage.RunCleanup(cfg.Cleanup, projectDir); err != nil {
				log.Fatalf("Failed to run cleanup tasks: %v", err)
			}
		}

		log.Println("Pipeline completed successfully!")
	},
}
