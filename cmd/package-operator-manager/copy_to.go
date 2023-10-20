package main

import (
	"fmt"
	"io"
	"os"
)

func deferedClose(dst *error, closer io.Closer) {
	cErr := closer.Close()
	if *dst == nil && cErr != nil {
		*dst = cErr
	}
}

func runCopyTo(target string) (rErr error) {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("looking up current executable path: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer deferedClose(&rErr, srcFile)

	dstFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("opening destination file: %w", err)
	}
	defer deferedClose(&rErr, dstFile)

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return os.Chmod(dstFile.Name(), 0o755)
}
