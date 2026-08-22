package main

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var embeddedVersion string

func toolVersion() string {
	return strings.TrimSpace(embeddedVersion)
}
