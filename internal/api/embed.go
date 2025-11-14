//go:build !test
// +build !test

package api

import "embed"

//go:embed static
var staticFiles embed.FS
