package packagedeploy

import (
	"context"
	"encoding/json"
	"fmt"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
)

type (
	// NoOpChunker implements objectChunker, but does not actually chunk.
	NoOpChunker struct{}

	// EachObjectChunker implements objectChunker, by putting every object into it's own ObjectSlice.
	EachObjectChunker struct{}

	// BinpackNextFitChunker implements objectChunker by putting up to `binpackNextFitStrategyChunkLimit`
	// of object bytes into their own ObjectSlices.
	BinpackNextFitChunker struct{}
)

type (
	chunkingStrategy string

	// objectChunker implements how to offload objects within a phase
	// into multiple ObjectSlices to reduce load on etcd and the api server.
	objectChunker interface {
		Chunk(ctx context.Context, phase *corev1alpha1.ObjectSetTemplatePhase) ([][]corev1alpha1.ObjectSetObject, error)
	}
)

const (
	// Allows to force a chunking strategy when set on a Package object.
	chunkingStrategyAnnotation = "packages.package-operator.run/chunking-strategy"

	// Chunks no objects at all.
	chunkingStrategyNoOp chunkingStrategy = "NoOp"

	// Chunks objects by putting every single object into it's own slice.
	chunkingStrategyEachObject chunkingStrategy = "EachObject"

	// Chunks objects by putting up to `binpackNextFitStrategyChunkLimit` of object bytes into their own ObjectSlices.
	chunkingStrategyBinpackNextFit chunkingStrategy = "BinpackNextFit"

	// etcd - the default Kubernetes database - has an object size limit of
	// 1 MiB for etcd <=v3.2: https://etcd.io/docs/v3.2/dev-guide/limit/
	// and
	// 1.5 MiB for etcd >v3.2: https://etcd.io/docs/v3.3/dev-guide/limit/
	// .
	binpackNextFitStrategyChunkLimit int = 1024 * 1024 // 1 MiB
)

var (
	_ objectChunker = (*NoOpChunker)(nil)
	_ objectChunker = (*EachObjectChunker)(nil)
	_ objectChunker = (*BinpackNextFitChunker)(nil)
)

// Returns the chunkingStrategy implementation for the given Package.
func determineChunkingStrategyForPackage(pkg adapters.GenericPackageAccessor) objectChunker {
	strategy := pkg.ClientObject().GetAnnotations()[chunkingStrategyAnnotation]
	switch chunkingStrategy(strategy) {
	case chunkingStrategyEachObject:
		return &EachObjectChunker{}
	case chunkingStrategyBinpackNextFit:
		return &BinpackNextFitChunker{}
	case chunkingStrategyNoOp:
		return &NoOpChunker{}
	default:
		return &BinpackNextFitChunker{}
	}
}

func (c *NoOpChunker) Chunk(
	context.Context, *corev1alpha1.ObjectSetTemplatePhase,
) ([][]corev1alpha1.ObjectSetObject, error) {
	return nil, nil
}

func (c *EachObjectChunker) Chunk(
	_ context.Context, phase *corev1alpha1.ObjectSetTemplatePhase,
) ([][]corev1alpha1.ObjectSetObject, error) {
	out := make([][]corev1alpha1.ObjectSetObject, len(phase.Objects))
	for i, obj := range phase.Objects {
		out[i] = []corev1alpha1.ObjectSetObject{obj}
	}
	return out, nil
}

// Chunk produces either 0 chunks if all objects are too small to overflow the first chunk, or at least two chunks.
func (c *BinpackNextFitChunker) Chunk(
	_ context.Context, phase *corev1alpha1.ObjectSetTemplatePhase,
) ([][]corev1alpha1.ObjectSetObject, error) {
	chunks := make([][]corev1alpha1.ObjectSetObject, 0, 1)
	currentChuckSize := 0
	currentChunk := make([]corev1alpha1.ObjectSetObject, 0)

	for _, obj := range phase.Objects {
		b, err := json.Marshal(obj.Object)
		if err != nil {
			return nil, fmt.Errorf("marshaling object to json for chunk estimation: %w", err)
		}

		size := len(b)

		// Close open chunk and allocate new chunk if the open chunk already contains objects and would overflow.
		if currentChuckSize > 0 && currentChuckSize+size > binpackNextFitStrategyChunkLimit {
			currentChuckSize = 0
			chunks = append(chunks, currentChunk)
			currentChunk = make([]corev1alpha1.ObjectSetObject, 0)
		}

		// Add object to open chunk.
		currentChunk = append(currentChunk, obj)
		currentChuckSize += size
	}

	// Signal chunking bypass because no chunks have been made.
	if len(chunks) == 0 {
		return nil, nil
	}

	// flush open chunk
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks, nil
}
