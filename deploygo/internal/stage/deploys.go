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
	for i, step := range deploys {
		log.Printf("Executing deploy %d/%d: %s", i+1, len(deploys), step.Name)

		server := config.GetServer(cfg, step.Server)
		if server == nil {
			return fmt.Errorf("server '%s' not found in configuration", step.Server)
		}

		if len(step.Commands) > 0 {
			if err := runSSHStep(server, &step); err != nil {
				return err
			}
		} else if step.From != "" && step.To != "" {
			if err := runTransferStep(server, &step, projectDir); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("deploy step '%s' has no commands or from/to fields", step.Name)
		}
	}
	return nil
}

func runSSHStep(server *config.ServerConfig, step *config.DeploymentStep) error {
	executor, err := deploy.NewSSHExecutor(server)
	if err != nil {
		return fmt.Errorf("failed to create SSH executor: %w", err)
	}
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

	uploader, err := deploy.NewSFTPUploader(server)
	if err != nil {
		return fmt.Errorf("failed to create SFTP uploader: %w", err)
	}
	defer uploader.Close()

	log.Printf("SFTP transferring: %s -> %s", source, dest)

	if srcInfo.IsDir() {
		return uploader.UploadDir(source, dest, nil)
	}

	return uploader.Upload(source, dest, nil)
}
