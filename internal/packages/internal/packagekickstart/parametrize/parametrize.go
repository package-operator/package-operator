// The package parametrize implements helpers and defaults
// to take a YAML documents and insert template expressions.
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
		blocks      []*block
		expressions []*expression
	)
	for _, i := range inst {
		if block, ok := i.(*block); ok {
			blocks = append(blocks, block)
		}
		if expression, ok := i.(*expression); ok {
			expressions = append(expressions, expression)
		}
	}

	for _, entry := range expressions {
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
	for _, entry := range expressions {
		var err error
		if b, err = entry.Replace(b); err != nil {
			return nil, err
		}
	}
	return b, nil
}

type block struct {
	marker     string
	expression string
	fieldPath  string

	originalValue interface{}
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
	if _, ok := parentValue.([]interface{}); ok {
		// wrap array items in an array again.
		origValue = []interface{}{origValue}
	} else {
		// wrap field values in map key.
		origValue = map[string]interface{}{
			b.fieldPath[lastDotIdx+1:]: origValue,
		}
	}

	b.originalValue = origValue
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

	origB, err := yaml.Marshal(b.originalValue)
	if err != nil {
		return nil, err
	}

	i := strings.Repeat(" ", indentLevel)
	var replacement string
	replacement += fmt.Sprintf("%s{{- %s }}\n", i, b.expression)
	for _, l := range bytes.Split(bytes.TrimSpace(origB), []byte("\n")) {
		replacement += fmt.Sprintf("%s%s\n", i, l)
	}
	replacement += fmt.Sprintf("%s{{- end }}\n", i)

	return re.ReplaceAll(in, []byte(replacement)), nil
}

// Wrap the given field path into a template block.
func Block(expression string, fieldPath string) Instruction {
	return &block{
		marker:     string(uuid.NewUUID()),
		expression: expression,
		fieldPath:  fieldPath,
	}
}

type expression struct {
	marker    string
	exp       string
	fieldPath string
}

func (e *expression) Mark(obj map[string]interface{}) error {
	return dotnotation.Set(obj, e.fieldPath, e.marker)
}
func (e *expression) Replace(in []byte) ([]byte, error) {
	return bytes.Replace(in, []byte(e.marker), []byte(fmt.Sprintf("{{ %s }}", e.exp)), 1), nil
}

// Insert a go template expression at the given location.
func Expression(exp string, fieldPath string) Instruction {
	return &expression{
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
	if obj.GroupVersionKind() == deployGVK && slices.Contains(paramsFlags, "replicas") {
		out, err := Deployment(obj, scheme, DeploymentOptions{Replicas: true})
		if err != nil {
			return nil, false, err
		}
		return out, true, nil
	}

	return nil, false, nil
}
