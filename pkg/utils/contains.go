package utils

import "slices"

func Contains(slice []string, str string) bool {
	return slices.Contains(slice, str)
}
