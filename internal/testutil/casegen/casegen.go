package casegen

import (
	"reflect"
)

type testingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Generate generates all possible combinations of values for fields of the type T.
// - len(dims) must equal len(fields(T))
// - dim[i].(type) must equal [](fields(T)[i].(type))
// - T must only have exported fields.
func Generate[T any](t testingT, tcase T, dims ...any) []T {
	t.Helper()

	typ := reflect.TypeOf(tcase)

	if typ.Kind() != reflect.Struct {
		t.Fatalf("tcase must be a struct type, got %s", typ.Kind())
	}

	if typ.NumField() == 0 {
		t.Fatalf("tcase must have at least one field")
	}

	if len(dims) == 0 {
		t.Fatalf("must supply one dim per tcase field")
	}

	if len(dims) != typ.NumField() {
		t.Fatalf("tcase must have same amount of fields as len(dims), got: %d", len(dims))
	}

	for i := range typ.NumField() {
		f := typ.Field(i)
		if !f.IsExported() {
			t.Fatalf("tcase must not have unexported fields, got: %s", f.Name)
		}
	}

	rvs := make([][]reflect.Value, len(dims))
	for i, dim := range dims {
		typ := reflect.TypeOf(dim)
		if typ.Kind() != reflect.Slice {
			t.Fatalf("dims must be slices of tcase field values, got: %s", typ.Kind())
		}
		val := reflect.ValueOf(dim)
		irvs := make([]reflect.Value, 0, val.Len())
		for j := range val.Len() {
			irvs = append(irvs, val.Index(j))
		}
		rvs[i] = irvs
	}

	g := &casegen[T]{
		t:    t,
		typ:  typ,
		dims: rvs,
		n:    make([]int, len(rvs)),
	}

	total := len(g.dims[0])
	for _, d := range g.dims[1:] {
		total *= len(d)
	}

	cases := make([]T, 0, total)

	for range total {
		cases = append(cases, g.elem())
		g.inc()
	}

	return cases
}

type casegen[T any] struct {
	t    testingT
	typ  reflect.Type
	dims [][]reflect.Value
	n    []int
}

func (g *casegen[T]) inc() {
	g.t.Helper()

	for i := 0; i < len(g.n); i++ {
		if g.n[i]+1 < len(g.dims[i]) {
			g.n[i]++
			break
		}
		g.n[i] = 0
	}
}

func (g *casegen[T]) elem() T {
	g.t.Helper()

	v := reflect.New(g.typ)

	for i := range v.Elem().NumField() {
		field := v.Elem().Field(i)
		ftype := field.Type()
		val := g.dims[i][g.n[i]]

		if ftype.Kind() == reflect.Pointer {
			ptr := reflect.New(val.Type().Elem())
			ptr.Elem().Set(val.Elem())
			field.Set(ptr)
		} else {
			field.Set(val)
		}
	}
	return v.Elem().Interface().(T)
}
