package transform

import (
	"errors"
	"text/template"

	"github.com/gobwas/glob"
)

func FileFuncs(files map[string][]byte) template.FuncMap {
	return template.FuncMap{
		"getFile":     getFile(files),
		"getFileGlob": getFileGlob(files),
	}
}

var ErrFileNotFound = errors.New("file not found")

func getFile(files map[string][]byte) func(path string) (string, error) {
	return func(path string) (string, error) {
		c, ok := files[path]
		if !ok {
			return "", ErrFileNotFound
		}
		return string(c), nil
	}
}

func getFileGlob(files map[string][]byte) func(pattern string) (
	map[string]string, error,
) {
	return func(pattern string) (map[string]string, error) {
		g, err := glob.Compile(pattern, '/')
		if err != nil {
			return nil, err
		}

		out := map[string]string{}
		for path, content := range files {
			if g.Match(path) {
				out[path] = string(content)
			}
		}
		return out, nil
	}
}
