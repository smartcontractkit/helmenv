package tools

import (
	"path/filepath"
	"runtime"
)

var (
	_, b, _, _ = runtime.Caller(0)
	// ProjectRoot root folder of this project
	ProjectRoot = filepath.Join(filepath.Dir(b), "/..")
	// ChartsRoot root folder for all helm charts
	ChartsRoot = filepath.Join(ProjectRoot, "charts")
)
