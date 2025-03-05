package internal

import (
	"runtime/debug"
	"strings"
)

const (
	_moduleName     = "github.com/luizaranda/go-core"
	_unknownVersion = "v0.0.0-unknown"
)

var (
	Version = func() string {
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, dep := range bi.Deps {
				if strings.EqualFold(dep.Path, _moduleName) {
					return dep.Version
				}
			}
		}

		return _unknownVersion
	}()
)
