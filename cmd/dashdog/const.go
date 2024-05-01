package main

import "log/slog"

const (
	flagConfig = "config"
	flagLog    = "log"

	flagPath                     = "path"
	flagName                     = "name"
	flagURL                      = "url"
	flagCFBundleName             = "cfbundle-name"
	flagDepth                    = "depth"
	flagPathRegex                = "sub-path-regex"
	flagSubPathBundleNamePattern = "sub-pattern-bundle-name-pattern"
	flagSubPathBundleNameReplace = "sub-pattern-bundle-name-replace"

	logOffLevel slog.Level = 16

	categoryGlobal = "global"
	categoryConfig = "config"
)
