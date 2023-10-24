package collections

import (
	"math"
	"time"
)

func PoorMansMap[T any, M any](a []T, f func(T) M) []M {
	n := make([]M, len(a))

	for i, element := range a {
		n[i] = f(element)
	}

	return n
}

// Returns []T consisting of all truthy elements from input.
func PoorMansFilter[T any](input []T, comparator func(el T) bool) []T {
	res := []T{}

	for _, el := range input {
		if comparator(el) {
			res = append(res, el)
		}
	}

	return res
}

// Divides []T to [][]T, such that each array element
// is an array of maximum length of batchSize.
func PoorMansPartition[T any](input []T, batchSize int) [][]T {
	var result [][]T

	for i := 0; i < len(input); i += batchSize {
		end := i + batchSize
		if end > len(input) {
			end = len(input)
		}
		result = append(result, input[i:end])
	}

	return result
}

// Returns first element and its index, of input for which comparator(N) is true.
// Returns null pointer and -1 if no element was found.
func PoorMansFind[T any](input []T, comparator func(el T) bool) (T, int) {
	var v T

	for ind, el := range input {
		if comparator(el) {
			return el, ind
		}
	}

	return v, -1
}

func Truncate[T any](input []T, limit int) []T {
	normalizedLimit := int(math.Min(float64(len(input)), float64(limit)))
	res := make([]T, normalizedLimit)
	for x := 0; x < normalizedLimit; x++ {
		res[x] = input[x]
	}

	return res
}

func PoorMansContains[T comparable](input []T, el T) bool {
	for _, sliceEl := range input {
		if sliceEl == el {
			return true
		}
	}

	return false
}

func PoorMansSum[T any, V int64 | float64 | int | time.Duration](input []T, f func(el T) V) V {
	var sum V

	for _, el := range input {
		sum += f(el)
	}

	return sum
}
