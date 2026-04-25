package container

import (
	"context"
	"fmt"
	"log"
)

func pullImageIfMissing(
	ctx context.Context,
	pulls *imagePullRegistry,
	image string,
	imageExists func(context.Context, string) (bool, error),
	pull func(context.Context, string) error,
) error {
	return pulls.do(ctx, image, func(ctx context.Context) error {
		exists, err := imageExists(ctx, image)
		if err != nil {
			return fmt.Errorf("failed to check image existence: %w", err)
		}
		if exists {
			log.Printf("Image '%s' already exists locally, skipping pull", image)
			return nil
		}

		log.Printf("Pulling image '%s'...", image)
		return pull(ctx, image)
	})
}
