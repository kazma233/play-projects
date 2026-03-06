package cmd

import (
	"log"

	"deploygo/internal/config"
	"deploygo/internal/stage"

	"github.com/spf13/cobra"
)

var deployStep string

var DeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the application",
	Long:  `Execute deployment steps defined in workspace/<project>/config.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		projectCtx, err := loadSelectedProjectConfig()
		if err != nil {
			log.Fatal(err)
		}
		cfg := projectCtx.Config
		basicPath := projectCtx.ProjectDir

		log.Printf("Project: %s", projectCtx.Name)
		log.Printf("Project directory: %s", basicPath)

		if deployStep != "" {
			step := config.FindDeploymentStep(cfg.Deploys, deployStep)
			if step == nil {
				log.Fatalf("Deploy step '%s' not found", deployStep)
			}
			if err := stage.RunDeploys(cfg, []config.DeploymentStep{*step}, basicPath); err != nil {
				log.Fatalf("Failed to execute '%s': %v", deployStep, err)
			}
		} else {
			if err := stage.RunDeploys(cfg, cfg.Deploys, basicPath); err != nil {
				log.Fatalf("Failed to execute deploy steps: %v", err)
			}
		}

		log.Println("Deployment completed successfully!")
	},
}

func init() {
	DeployCmd.Flags().StringVarP(&deployStep, "step", "s", "", "Specific deployment step to execute")
}
