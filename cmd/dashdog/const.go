package main

import "log/slog"

const (
	flagConfig = "config"
	flagLog    = "log"

	flagPath                     = "path"
	flagName                     = "name"
	flagURL                      = "url"
	flagCFBundleName             = "cfbundle"
	flagDepth                    = "depth"
	flagPathRegex                = "path-regex"
	flagSubPathBundleNamePattern = "bundle-pattern"
	flagSubPathBundleNameReplace = "bundle-replace"

	logOffLevel slog.Level = 16

	categoryGlobal = "global"
	categoryConfig = "config"

	defaultPath = "$HOME/dashdog-doc/"
)
