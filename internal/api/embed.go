//go:build !test
// +build !test

package api

import "embed"

//go:embed all:static
var staticFiles embed.FS
