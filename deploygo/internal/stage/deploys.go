package stage

import (
	"fmt"
	"log"
	"os"

	"deploygo/internal/config"
	"deploygo/internal/deploy"
	"deploygo/internal/fileutil"
)

func RunDeploys(cfg *config.Config, deploys []config.DeploymentStep, projectDir string) error {
	for i := range deploys {
		if err := runDeployEntry(cfg, &deploys[i], projectDir, i+1, len(deploys)); err != nil {
			return err
		}
	}

	return nil
}

func RunDeploy(cfg *config.Config, step *config.DeploymentStep, projectDir string) error {
	return runDeployEntry(cfg, step, projectDir, 0, 0)
}

func runDeployEntry(cfg *config.Config, step *config.DeploymentStep, projectDir string, index, total int) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if step == nil {
		return fmt.Errorf("deploy step is nil")
	}

	if total > 0 {
		log.Printf("Executing deploy %d/%d: %s", index, total, step.Name)
	} else {
		log.Printf("Executing deploy: %s", step.Name)
	}

	server := cfg.FindServer(step.Server)
	if server == nil {
		return fmt.Errorf("server '%s' not found in configuration", step.Server)
	}

	if len(step.Commands) > 0 {
		return runSSHStep(server, step)
	}

	if step.From != "" && step.To != "" {
		return runTransferStep(server, step, projectDir)
	}

	return fmt.Errorf("deploy step '%s' has no commands or from/to fields", step.Name)
}

func runSSHStep(server *config.ServerConfig, step *config.DeploymentStep) error {
	executor := deploy.NewSSHExecutor(server)
	defer executor.Close()

	if len(step.Commands) == 0 {
		return fmt.Errorf("no commands specified for SSH deployment step")
	}

	for _, command := range step.Commands {
		log.Printf("Executing: %s", command)
		if err := executor.Execute(command); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
	}

	return nil
}

func runTransferStep(server *config.ServerConfig, step *config.DeploymentStep, projectDir string) error {
	source, err := fileutil.ResolveWithin(projectDir, step.From)
	if err != nil {
		return fmt.Errorf("invalid deploy source path: %w", err)
	}
	dest := step.To

	if source == "" || dest == "" {
		return fmt.Errorf("missing source or destination path in deployment step")
	}

	srcInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	uploader := deploy.NewSFTPUploader(server)
	defer uploader.Close()

	log.Printf("SFTP transferring: %s -> %s", source, dest)

	if srcInfo.IsDir() {
		return uploader.UploadDir(source, dest, nil)
	}

	return uploader.Upload(source, dest, nil)
}
