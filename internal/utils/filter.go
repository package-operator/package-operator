package utils

type Predicate func(value interface{}) (bool, error)

// Filter creates a slice with elements from s that satisfy pred.
func Filter[T any](s []T, pred Predicate) ([]T, error) {
	var newSlice []T
	for _, e := range s {
		b, err := pred(e)
		if err != nil {
			return nil, err
		}
		if b {
			newSlice = append(newSlice, e)
		}
	}
	return newSlice, nil
}
