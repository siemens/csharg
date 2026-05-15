// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package examples

import (
	"strings"

	"github.com/thediveo/go-plugger/v3"
)

// ForCommands maps command names to examples. Thus, a single plugin can
// contribute examples to multiple commands.
type ForCommands map[string]string

// Illustrate returns one or more CLI examples that are indexed by their
// particular (sub) command.
type Illustrate func() ForCommands

// For collects all examples for the specified command from the registered
// plugins. The examples returned by plugins are always separated by empty
// lines, yet there isn't any trailing newline for the overall section.
func For(command string) string {
	var cmdExamples strings.Builder
	for _, examples := range plugger.Group[Illustrate]().Symbols() {
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
