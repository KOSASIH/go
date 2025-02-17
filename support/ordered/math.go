package ordered

import (
	"golang.org/x/exp/constraints"
)

// Min returns the smaller of the given items.
func Min[T constraints.Ordered](a, b T) T {
	if a <= b {
		return a
	}
	return b
}

// Min returns the larger of the given items.
func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// MinSlice returns the smallest element in a slice-like container.
func MinSlice[T constraints.Ordered](slice []T) T {
	var smallest T

	for i := 0; i < len(slice); i++ {
		if i == 0 || slice[i] < smallest {
			smallest = slice[i]
		}
	}

	return smallest
}

// MaxSlice returns the largest element in a slice-like container.
func MaxSlice[T constraints.Ordered](slice []T) T {
	var largest T

	for i := 0; i < len(slice); i++ {
		if i == 0 || slice[i] > largest {
			largest = slice[i]
		}
	}

	return largest
}
