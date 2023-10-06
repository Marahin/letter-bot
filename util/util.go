package util

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

func StrToInt64(i string) (int64, error) {
	id, err := strconv.ParseInt(i, 10, 0)
	if err != nil {
		return 0, err
	}

	return id, nil
}

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

const DC_TIME_FORMAT = "15:04"

const DC_LONG_TIME_FORMAT = "2006-01-02 15:04"

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

type LogEntry interface {
	Error(args ...interface{})
}

// LogErrors allows for error handling in defer calls:
// ```
// defer functionThatReturnsError()
// ````
// This would lead to uncatched error. Wrapping it with LogError solves this issue:
// ```
// defer LogError(functionThatReturnsError())
// ```
func LogError(log LogEntry, err error) {
	if err != nil {
		log.Error(err)
	}
}

// Deliberately ignore error. This is specifically used in cases where we
// do not care about the error message at all, such as the case
// of `defer tx.Rollback(ctx)`: https://stackoverflow.com/a/62533516
func IgnoreError(err error) {
	logrus.Debugf("IgnoreError: %s", err)
}

// This is for case of `tx.Rollback(ctx)` in defer.
// defer calls happen when the function body ends; however,
// their arguments are evaluated at the moment `defer` is called.
// For instance, if we want to deliberately ignore errors (so golangci-lint doesn't yell at us)
// but still have `defer tx.Rollback(ctx)` (which does nothing if the transaction succeeded)
// we can't run `defer IgnoreError(tx.Rollback(ctx))` because the `tx.Rollback` part is evaluated
// the very moment. And so it cancels transaction at the moment of calling defer.
func ExecuteAndIgnoreErrorF(f func(context.Context) error, ctx context.Context) {
	IgnoreError(f(ctx))
}
