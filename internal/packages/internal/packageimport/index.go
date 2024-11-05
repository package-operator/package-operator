package packageimport

import (
	"io/fs"
	"os"
	"path/filepath"
)

func Index(basepath string) (map[string]struct{}, error) {
	paths := map[string]struct{}{}

	return paths, walkWithSymlinks(basepath, basepath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		path, err = filepath.Rel(basepath, path)
		if err != nil {
			return err
		}
		paths[path] = struct{}{}
		return nil
	})
}

func walkWithSymlinks(filename string, linkDirname string, walkFn filepath.WalkFunc) error {
	symWalkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fname, err := filepath.Rel(filename, path); err == nil {
			path = filepath.Join(linkDirname, fname)
		} else {
			return err
		}

		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			resolvedPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			info, err := os.Lstat(resolvedPath)
			if err != nil {
				return walkFn(path, info, err)
			}
			if info.IsDir() {
				return walkWithSymlinks(resolvedPath, path, walkFn)
			}
		}
		return walkFn(path, info, err)
	}
	return filepath.Walk(filename, symWalkFunc)
}
