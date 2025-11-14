//go:build test
// +build test

package api

import "embed"

// 测试时使用空的文件系统
var staticFiles embed.FS

