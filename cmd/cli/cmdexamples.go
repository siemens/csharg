// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package cli

import (
	"strings"

	"github.com/thediveo/go-plugger/v3"
)

// Examples collects all examples for the specified command from the registered
// plugins. The examples returned by plugins are always separated by empty
// lines, yet there isn't any trailing newline for the overall section.
func Examples(command string) string {
	var examples strings.Builder
	for _, example := range plugger.Group[CommandExamples]().Symbols() {
		cmd, text := example()
		if cmd != command {
			continue
		}
		text = strings.TrimSuffix(text, "\n")
		if text == "" {
			continue
		}
		if examples.Len() > 0 {
			examples.WriteString("\n\n")
		}
		examples.WriteString(text)
	}
	return examples.String()
}
