package main

import (
	"fmt"
	"io"
	"os"
)

func runCopyTo(target string) error {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("looking up current executable path: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("opening destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return os.Chmod(destFile.Name(), 0o755)
}
