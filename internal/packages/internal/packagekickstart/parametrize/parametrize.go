// The package parametrize implements helpers and defaults
// to take a YAML documents and insert template Pipelines.
package parametrize

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/joeycumines/go-dotnotation/dotnotation"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/yaml"
)

type Instruction interface {
	Mark(obj map[string]interface{}) error
	Replace(in []byte) ([]byte, error)
}

func Execute(obj unstructured.Unstructured, inst ...Instruction) ([]byte, error) {
	obj = *obj.DeepCopy()

	var (
		blocks    []Instruction
		pipelines []*pipeline
	)
	for _, i := range inst {
		if block, ok := i.(*block); ok {
			blocks = append(blocks, block)
		}
		if block, ok := i.(*mergeBlock); ok {
			blocks = append(blocks, block)
		}
		if Pipeline, ok := i.(*pipeline); ok {
			pipelines = append(pipelines, Pipeline)
		}
	}

	for _, entry := range pipelines {
		if err := entry.Mark(obj.Object); err != nil {
			return nil, err
		}
	}
	for _, entry := range blocks {
		if err := entry.Mark(obj.Object); err != nil {
			return nil, err
		}
	}

	b, err := yaml.Marshal(obj.Object)
	if err != nil {
		return nil, err
	}

	for _, entry := range blocks {
		var err error
		if b, err = entry.Replace(b); err != nil {
			return nil, err
		}
	}
	for _, entry := range pipelines {
		var err error
		if b, err = entry.Replace(b); err != nil {
			return nil, err
		}
	}
	return b, nil
}

type mergeBlock struct {
	block
}

func (b *mergeBlock) Replace(in []byte) ([]byte, error) {
	re := regexp.MustCompile(`(?m).*` + b.marker + `\n*`)
	found := re.Find(in)
	if found == nil {
		return nil, nil
	}

	var indentLevel int
	for _, c := range found {
		if rune(c) == rune(' ') {
			indentLevel++
		} else {
			break
		}
	}

	origB, err := yaml.Marshal(b.writeValue)
	if err != nil {
		return nil, err
	}

	i := strings.Repeat(" ", indentLevel)
	var replacement string
	replacement += fmt.Sprintf("%s{{- define %q }}\n", i, b.marker)
	for _, l := range bytes.Split(bytes.TrimSpace(origB), []byte("\n")) {
		replacement += fmt.Sprintf("%s%s\n", i, l)
	}
	replacement += fmt.Sprintf(`%s{{- end }}{{"\n"}}`+"\n", i)
	if _, isSlice := b.originalValue.([]interface{}); isSlice {
		replacement += fmt.Sprintf(`%s{{- dict %q (concat (fromYAML (include %q .)).%s (%s)) | toYAML | indent %d }}`+"\n", i, b.mapKey, b.marker, b.mapKey, b.pipeline, len(i))
	} else {
		// assume map
		replacement += fmt.Sprintf(`%s{{- merge (fromYAML (include %q .)) (%s)  | toYAML | indent %d }}`+"\n", i, b.marker, b.pipeline, len(i))
	}

	return re.ReplaceAll(in, []byte(replacement)), nil
}

// Wrap the given field path into a template block.
func MergeBlock(pipeline string, fieldPath string) Instruction {
	return &mergeBlock{
		block: block{
			marker:    string(uuid.NewUUID()),
			pipeline:  pipeline,
			fieldPath: fieldPath,
		},
	}
}

type block struct {
	marker    string
	pipeline  string
	fieldPath string

	mapKey        string
	originalValue interface{}
	writeValue    interface{}
}

func (b *block) Mark(obj map[string]interface{}) error {
	lastDotIdx := strings.LastIndex(b.fieldPath, ".")
	parentPath := b.fieldPath[:lastDotIdx]
	parentValue, err := dotnotation.Get(obj, parentPath)
	if err != nil {
		return err
	}

	origValue, err := dotnotation.Get(obj, b.fieldPath)
	if err != nil {
		return err
	}
	b.originalValue = origValue
	if _, ok := parentValue.([]interface{}); ok {
		// wrap array items in an array again.
		origValue = []interface{}{origValue}
	} else {
		b.mapKey = b.fieldPath[lastDotIdx+1:]
		// wrap field values in map key.
		origValue = map[string]interface{}{
			b.mapKey: origValue,
		}
	}

	b.writeValue = origValue
	return dotnotation.Set(obj, b.fieldPath, b.marker)
}

func (b *block) Replace(in []byte) ([]byte, error) {
	re := regexp.MustCompile(`(?m).*` + b.marker + `\n*`)
	found := re.Find(in)
	if found == nil {
		return nil, nil
	}

	var indentLevel int
	for _, c := range found {
		if rune(c) == rune(' ') {
			indentLevel++
		} else {
			break
		}
	}

	origB, err := yaml.Marshal(b.writeValue)
	if err != nil {
		return nil, err
	}

	i := strings.Repeat(" ", indentLevel)
	var replacement string
	replacement += fmt.Sprintf("%s{{- %s }}\n", i, b.pipeline)
	for _, l := range bytes.Split(bytes.TrimSpace(origB), []byte("\n")) {
		replacement += fmt.Sprintf("%s%s\n", i, l)
	}
	replacement += fmt.Sprintf("%s{{- end }}\n", i)

	return re.ReplaceAll(in, []byte(replacement)), nil
}

// Wrap the given field path into a template block.
func Block(pipeline string, fieldPath string) Instruction {
	return &block{
		marker:    string(uuid.NewUUID()),
		pipeline:  pipeline,
		fieldPath: fieldPath,
	}
}

type pipeline struct {
	marker    string
	exp       string
	fieldPath string
}

func (e *pipeline) Mark(obj map[string]interface{}) error {
	return dotnotation.Set(obj, e.fieldPath, e.marker)
}
func (e *pipeline) Replace(in []byte) ([]byte, error) {
	return bytes.Replace(in, []byte(e.marker), []byte(fmt.Sprintf("{{ %s }}", e.exp)), 1), nil
}

// Insert a go template Pipeline at the given location.
func Pipeline(exp string, fieldPath string) Instruction {
	return &pipeline{
		marker:    string(uuid.NewUUID()),
		exp:       exp,
		fieldPath: fieldPath,
	}
}

func Parametrize(
	obj unstructured.Unstructured,
	scheme *apiextensionsv1.JSONSchemaProps,
	paramsFlags []string,
) ([]byte, bool, error) {
	if len(paramsFlags) == 0 {
		return nil, false, nil
	}

	deployGVK := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}
	if obj.GroupVersionKind() == deployGVK && len(paramsFlags) > 0 {
		out, err := Deployment(obj, scheme, DeploymentOptions{
			Replicas:    slices.Contains(paramsFlags, "replicas"),
			Tolerations: slices.Contains(paramsFlags, "tolerations"),
		})
		if err != nil {
			return nil, false, err
		}
		return out, true, nil
	}

	return nil, false, nil
}
