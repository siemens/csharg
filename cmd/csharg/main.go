// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// This is the main entry of the csharg CLI tool. There isn't actually much
// here to do except for running the csharg "root" command which will parse
// the CLI args and then hopefully invoke the correct command and sub-command.

package main

import (
	"os"
)

func main() {
	// This is cobra boilerplate documentation, except for the missing call to
	// fmt.Println(err) which in the original boilerplate is just plain wrong:
	// it renders the error message twice, see also:
	// https://github.com/spf13/cobra/issues/304
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
