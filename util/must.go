package util

import "fmt"

func Must(err error) {
	if err != nil {
		panic(fmt.Errorf("unexpected error: %v", err))
	}
}

func Must2[T any](v T, err error) T {
	Must(err)
	return v
}
