package main

import "strings"

func stringContains(source string, check []string) bool {
	source = strings.ToLower(source)
	for _, st := range check {
		if strings.Contains(source, st) {
			return true
		}
	}
	return false
}
