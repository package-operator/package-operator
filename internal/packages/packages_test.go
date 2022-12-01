package packages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		path string
		out  bool
	}{
		{
			path: "test.yml",
			out:  true,
		},
		{
			path: "test.yaml",
			out:  true,
		},
		{
			path: "test",
			out:  false,
		},
		{
			path: "test.txt",
			out:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			out := IsYAMLFile(test.path)
			assert.Equal(t, test.out, out)
		})
	}
}

func TestIsTemplateFile(t *testing.T) {
	tests := []struct {
		path string
		out  bool
	}{
		{
			path: "test.yml.gotmpl",
			out:  true,
		},
		{
			path: "test.yaml.gotmpl",
			out:  true,
		},
		{
			path: "test.gotmpl",
			out:  true,
		},
		{
			path: "test.yaml",
			out:  false,
		},
		{
			path: "test.txt",
			out:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			out := IsTemplateFile(test.path)
			assert.Equal(t, test.out, out)
		})
	}
}

func TestIsManifestFile(t *testing.T) {
	tests := []struct {
		path string
		out  bool
	}{
		{
			path: "manifest.yml",
			out:  true,
		},
		{
			path: "manifest.yaml",
			out:  true,
		},
		{
			path: "test.gotmpl",
			out:  false,
		},
		{
			path: "test.yaml",
			out:  false,
		},
		{
			path: "test.txt",
			out:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			out := IsManifestFile(test.path)
			assert.Equal(t, test.out, out)
		})
	}
}
