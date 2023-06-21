// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package cli

import (
	"strings"

	"github.com/thediveo/go-plugger/v3"
)

// Examples collects all examples for the specified command from the registered
// plugins. The examples returned by plugins are always separate with empty
// lines, yet there isn't any trailing newline for the overall section.
func Examples(command string) string {
	examples := ""
	for _, example := range plugger.Group[CommandExamples]().Symbols() {
		text := strings.TrimSuffix(example()[command], "\n")
		if text == "" {
			continue
		}
		if examples != "" {
			examples += "\n\n"
		}
		examples += text
	}
	return examples
}
