package packagedeploy

import (
	"context"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type (
	// NoOpChunker implements objectChunker, but does not actually chunk.
	NoOpChunker struct{}

	// NoOpChunker implements objectChunker, by putting every object into it's own ObjectSlice.
	EachObjectChunker struct{}
)

type (
	chunkingStrategy string

	// objectChunker implements how to offload objects within a phase into multiple ObjectSlices to reduce load on etcd and the api server.
	objectChunker interface {
		Chunk(ctx context.Context, phase *corev1alpha1.ObjectSetTemplatePhase) ([][]corev1alpha1.ObjectSetObject, error)
	}
)

const (
	// Allows to force a chunking strategy when set on a Package object.
	chunkingStrategyAnnotation = "packages.package-operator.run/chunking-strategy"

	// Chunks objects by putting every single object into it's own slice.
	chunkingStrategyEachObject chunkingStrategy = "EachObject"
)

var (
	_ objectChunker = (*NoOpChunker)(nil)
	_ objectChunker = (*EachObjectChunker)(nil)
)

// Returns the chunkingStrategy implementation for the given Package.
func determineChunkingStrategyForPackage(pkg genericPackage) objectChunker {
	strategy := pkg.ClientObject().GetAnnotations()[chunkingStrategyAnnotation]
	switch chunkingStrategy(strategy) {
	case chunkingStrategyEachObject:
		return &EachObjectChunker{}
	default:
		return &NoOpChunker{}
	}
}

func (c *NoOpChunker) Chunk(context.Context, *corev1alpha1.ObjectSetTemplatePhase) ([][]corev1alpha1.ObjectSetObject, error) {
	return nil, nil
}

func (c *EachObjectChunker) Chunk(_ context.Context, phase *corev1alpha1.ObjectSetTemplatePhase) ([][]corev1alpha1.ObjectSetObject, error) {
	var out [][]corev1alpha1.ObjectSetObject
	for _, obj := range phase.Objects {
		out = append(out, []corev1alpha1.ObjectSetObject{obj})
	}
	return out, nil
}
