package casegen

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func ptrTo(i int) *int {
	return &i
}

func TestSeq(t *testing.T) {
	t.Parallel()

	type tcase struct {
		X string
		Y int
		Z *int
	}

	expected := []tcase{
		{X: "a", Y: 1, Z: ptrTo(1)},
		{X: "b", Y: 1, Z: ptrTo(1)},
		{X: "a", Y: 2, Z: ptrTo(1)},
		{X: "b", Y: 2, Z: ptrTo(1)},
		{X: "a", Y: 1, Z: ptrTo(2)},
		{X: "b", Y: 1, Z: ptrTo(2)},
		{X: "a", Y: 2, Z: ptrTo(2)},
		{X: "b", Y: 2, Z: ptrTo(2)},
	}
	generated := Generate(t, tcase{},
		[]string{"a", "b"},
		[]int{1, 2},
		[]*int{ptrTo(1), ptrTo(2)})

	if !reflect.DeepEqual(expected, generated) {
		t.Logf("Expected: %v, got: %v", expected, generated)
		t.Fail()
	}
}

func TestSeq_Funcs(t *testing.T) {
	t.Parallel()

	type tcase struct {
		X func() int
		Y func() string
	}

	type expectedValues struct {
		int
		string
	}

	expected := []expectedValues{
		{2, "a"},
		{3, "a"},
		{2, "b"},
		{3, "b"},
	}
	generated := Generate(t, tcase{},
		[]func() int{
			func() int { return 2 },
			func() int { return 3 },
		},
		[]func() string{
			func() string { return "a" },
			func() string { return "b" },
		},
	)

	if len(generated) != len(expected) {
		t.Logf("Expected %d items, got: %d", len(expected), len(generated))
		t.Fail()
	}

	for i, tcase := range generated {
		if tcase.X() != expected[i].int {
			t.Logf("Expected: %d, got: %d", expected[i].int, tcase.X())
			t.Fail()
		}
		if tcase.Y() != expected[i].string {
			t.Logf("Expected: %s, got: %s", expected[i].string, tcase.Y())
			t.Fail()
		}
	}
}

type testMock struct {
	mock.Mock
}

func (m *testMock) Helper() {
	m.Called()
}

func (m *testMock) Fatalf(format string, args ...any) {
	m.Called(format, args)
	panic(fmt.Sprintf(format, args...))
}

func TestSeq_UnexportedField(t *testing.T) {
	t.Parallel()

	type tcase struct {
		y string //nolint:unused
	}

	tm := &testMock{}

	tm.On("Helper")
	tm.On("Fatalf", "tcase must not have unexported fields, got: %s", []interface{}{"y"}).Once()

	assert.Panics(t, func() {
		_ = Generate(tm, tcase{}, []string{"a", "b"})
	})

	tm.AssertExpectations(t)
}
