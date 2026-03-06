package cmd

import (
	"log"

	"deploygo/internal/config"
	"deploygo/internal/container"
	"deploygo/internal/stage"

	"github.com/spf13/cobra"
)

var buildStage string

var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all or specific stages",
	Long:  `Build all stages defined in workspace/<project>/config.yaml or a specific stage`,
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

		if buildStage != "" {
			build := config.FindStage(cfg.Builds, buildStage)
			if build == nil {
				log.Fatalf("Build '%s' not found", buildStage)
			}
			if err := stage.RunBuilds(containerMgr, []config.StageConfig{*build}, basicPath); err != nil {
				log.Fatalf("Failed to build '%s': %v", buildStage, err)
			}
		} else {
			if err := stage.RunBuilds(containerMgr, cfg.Builds, basicPath); err != nil {
				log.Fatalf("Failed to build stages: %v", err)
			}
		}

		log.Println("Build completed successfully!")
	},
}

func init() {
	BuildCmd.Flags().StringVarP(&buildStage, "stage", "s", "", "Specific stage to build")
}
