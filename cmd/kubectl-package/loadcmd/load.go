package loadcmd

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/internal/cmd" // how does this get loaded
)

type Loader interface {
	LoadImage(ctx context.Context, imagePath string) error
}

// DefaultLoader is the default implementation of the Loader interface.
type DefaultLoader struct {
	// Add any fields if necessary
}

func NewCmd(loader Loader) *cobra.Command {
	const (
		loadUse   = "load image tar from file path"
		loadShort = "Load a PKO package"
		loadLong  = "Loads an image from a locally stored archive (tar file) into container storage using either Docker or Podman."
	)

	cmd := &cobra.Command{
		Use:   loadUse,
		Short: loadShort,
		Long:  loadLong,
		Args:  cobra.ExactArgs(1),
	}

	var opts options
	opts.AddFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		imagePath := args[0]
		if imagePath == "" {
			return fmt.Errorf("%w: image path empty", internalcmd.ErrInvalidArgs)
		}

		// Ensure the image path is absolute
		absImagePath, err := filepath.Abs(imagePath)
		if err != nil {
			return fmt.Errorf("resolving absolute path of image: %w", err)
		}

		return loader.LoadImage(cmd.Context(), absImagePath)
	}

	return cmd
}

type options struct {
	// Define any options your load command might need
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	// Define any flags for your load command here
}

// LoadImage implements the Loader interface for DefaultLoader.
func (dl *DefaultLoader) LoadImage(ctx context.Context, imagePath string) error {
	runtime, err := detectRuntime()
	if err != nil {
		return fmt.Errorf("detecting container runtime: %w", err)
	}

	var loadCmd *exec.Cmd
	if runtime == "docker" {
		loadCmd = exec.CommandContext(ctx, "docker", "load", "-i", imagePath)
	} else { // Assuming runtime is podman
		loadCmd = exec.CommandContext(ctx, "podman", "load", "-i", imagePath)
	}

	output, err := loadCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("loading image with %s: %s\n%s", runtime, err, string(output))
	}

	fmt.Printf("Image loaded using %s: %s\n", runtime, string(output))
	return nil
}

func detectRuntime() (string, error) {
	if isCommandAvailable("docker") {
		return "docker", nil
	} else if isCommandAvailable("podman") {
		return "podman", nil
	} else {
		return "", fmt.Errorf("neither Docker nor Podman is available, please install one of these container runtimes")
	}
}

func isCommandAvailable(name string) bool {
	cmd := exec.Command("command", "-v", name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
