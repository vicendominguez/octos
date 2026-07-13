// Package main — sanitize.go provides string sanitization utilities for agent output.
// stripANSI removes ANSI escape codes, spinner characters, cursor movement sequences,
// and other terminal control characters from captured text.
package main

import (
	"regexp"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b(\[[0-9;?]*[a-zA-Z]|\(B|\)0)`)
var spinnerRegex = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]\s*`)
var controlCharsRegex = regexp.MustCompile(`[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F]`)
var cursorMovementRegex = regexp.MustCompile(`\x1b\[[0-9]*[ABCDEFGHJKST]`)

func stripANSI(s string) string {
	// Remove ANSI escape codes (colors, styles)
	s = ansiRegex.ReplaceAllString(s, "")
	// Remove cursor movement codes
	s = cursorMovementRegex.ReplaceAllString(s, "")
	// Remove spinner characters
	s = spinnerRegex.ReplaceAllString(s, "")
	// Remove other control characters (except \n and \t)
	s = controlCharsRegex.ReplaceAllString(s, "")
	// Remove carriage returns
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
