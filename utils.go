package main

import "strings"

func stringStartsWith(source string, check []string) bool {
	source = strings.ToLower(source)
	for _, st := range check {
		if strings.HasPrefix(source, st) {
			return true
		}
	}
	return false
}

func escapeMarkdownV2(s string) string {
	chars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, c := range chars {
		s = strings.Replace(s, c, `\`+c, -1)
	}
	return s
}
