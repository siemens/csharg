// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package examples

import (
	"strings"

	"github.com/thediveo/go-plugger/v3"
)

// Examples maps command names to examples.
type Examples map[string]string

// CommandExamples defines an exposed symbol with CLI examples, indexed by a
// particular (sub) command, namely: “list” and “capture” at this time.
type CommandExamples func() Examples

// ExamplesFor collects all examples for the specified command from the
// registered plugins. The examples returned by plugins are always separated by
// empty lines, yet there isn't any trailing newline for the overall section.
func ExamplesFor(command string) string {
	var cmdExamples strings.Builder
	for _, examples := range plugger.Group[CommandExamples]().Symbols() {
		for cmd, text := range examples() {
			if cmd != command {
				continue
			}
			text = strings.TrimSuffix(text, "\n")
			if text == "" {
				continue
			}
			if cmdExamples.Len() > 0 {
				cmdExamples.WriteString("\n\n")
			}
			cmdExamples.WriteString(text)
		}
	}
	return cmdExamples.String()
}
