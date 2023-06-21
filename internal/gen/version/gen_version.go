// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// This program generates defs_version.go with version information queried from
// git.

package main

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"regexp"
)

// https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
// -- but without anchoring it to begin and end.
const semVerPattern = `((?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?)`

var versionExtractor = regexp.MustCompile(`^v` + semVerPattern + `\n?$`)
var branchVersionExtractor = regexp.MustCompile(`^release/v` + semVerPattern + `\n?$`)

var defsVersionGoTemplate = template.Must(template.New("defs_version.go").Parse(
	`// Let goreportcard check us.
// Code generated by gen_version; DO NOT EDIT.

//go:generate go run ./internal/gen/version

package csharg

// SemVersion is the semantic version string of the csharg module.
const SemVersion = "{{ . }}"
`))

func main() {
	var version string
	// Are we on a git-flow release branch?
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		panic(err)
	}
	if match := branchVersionExtractor.FindStringSubmatch(string(out)); match != nil {
		version = match[1]
	} else {
		out, err = exec.Command("git", "describe").Output()
		if err != nil {
			panic(err)
		}
		match := versionExtractor.FindStringSubmatch(string(out))
		if match == nil {
			panic(fmt.Sprintf("error: invalid version %q", string(out)))
		}
		version = match[1]
	}
	fmt.Printf("defs_version.go: version %q\n", version)
	f, err := os.Create("defs_version.go")
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()
	if err := defsVersionGoTemplate.Execute(f, version); err != nil {
		panic(err)
	}
}