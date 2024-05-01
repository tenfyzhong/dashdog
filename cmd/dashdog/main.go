package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v3"

	"github.com/tenfyzhong/dashdog"
	"github.com/tenfyzhong/dashdog/cmd/dashdog/version"
	"gopkg.in/yaml.v3"
)

func main() {
	app := &cli.Command{
		Name:        "dashdog",
		Usage:       "a tool to generate docset for dash",
		UsageText:   "dashdog -c|--config <file> [--log off] [config options]",
		Version:     version.Version,
		Description: "",
		Commands:    []*cli.Command{},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:      flagConfig,
				OnlyOnce:  true,
				Usage:     "the config `file` to load",
				Required:  true,
				Aliases:   []string{"c"},
				TakesFile: true,
				Validator: func(v string) error {
					_, err := os.Stat(v)
					if err != nil && !os.IsNotExist(err) {
						return err
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:     flagLog,
				OnlyOnce: true,
				Usage:    "log `level`, the log will print to stdout, available value:[debug,info,warn,error,off]",
				Value:    "off",
				Validator: func(v string) error {
					str := strings.ToLower(v)
					if str != "debug" && str != "info" && str != "warn" && str != "error" && str != "off" {
						return errors.Errorf("invalid log level %s", v)
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:      flagPath,
				Category:  categoryConfig,
				OnlyOnce:  true,
				Usage:     "the `path` to generate docset, it will overwrite the value of `path` item in the config file",
				TakesFile: true,
			},
			&cli.StringFlag{
				Name:     flagName,
				Category: categoryConfig,
				OnlyOnce: true,
				Usage:    "the `name` of the docset, it will overwrite the value of `name` item in the config file",
			},
			&cli.StringFlag{
				Name:     flagURL,
				Category: categoryConfig,
				OnlyOnce: true,
				Usage:    "the source `url` of the docset, it will overwrite the value of `url` item in the config",
			},
			&cli.StringFlag{
				Name:     flagCFBundleName,
				Category: categoryConfig,
				OnlyOnce: true,
				Usage:    "the `bundle-name` of the root page, it will overwrite the value of `plist->cfbndle_name` item in the config",
			},
			&cli.IntFlag{
				Name:     flagDepth,
				Category: categoryConfig,
				OnlyOnce: true,
				Usage:    "the max `depth` of sub page to generate, at least 1, it will overwrite the value of `depth` item in the config",
				Value:    1,
			},
			&cli.StringFlag{
				Name:     flagPathRegex,
				Category: categoryConfig,
				OnlyOnce: true,
				Usage:    "only the path match the `pattern` will process, it will overwrite the value of `sub_path_regex` item in the config",
			},
			&cli.StringFlag{
				Name:     flagSubPathBundleNamePattern,
				Category: categoryConfig,
				OnlyOnce: true,
				Usage:    "a `pattern` to match the path of the sub module name, the group captured can be use in the sub-pattern-bundle-name-replace flag, it will overwrite the value of `sub_path_bundle_name->pattern` item in the config",
			},
			&cli.StringFlag{
				Name:     flagSubPathBundleNameReplace,
				Category: categoryConfig,
				OnlyOnce: true,
				Usage:    "a `replace-pattern` to replace the path which matched by sub-pattern-bundle-name-pattern flag, it will overwrite the value of `sub_pattern_bundle_name->replace` item in the config",
			},
		},
		HideHelp:                   false,
		HideHelpCommand:            true,
		HideVersion:                false,
		EnableShellCompletion:      true,
		ShellCompletionCommandName: "dashdog",
		Before: func(ctx context.Context, cmd *cli.Command) error {
			setLogLevel(cmd)
			return nil
		},
		Action:    action,
		Authors:   []any{"tenfyzhong"},
		Copyright: "Copyright (c) 2024 tenfy",
		ExitErrHandler: func(_ context.Context, _ *cli.Command, err error) {
			if err != nil {
				slog.Error("err", slog.String("err", fmt.Sprintf("%+v", err)))
				os.Exit(-11)
			}
		},
		Suggest: true,
	}

	app.Run(context.Background(), os.Args)
}

func action(_ context.Context, cmd *cli.Command) error {
	cfile := cmd.String("config")
	if cfile == "" {
		return errors.Errorf("config is empty")
	}
	data, err := os.ReadFile(cfile)
	if err != nil {
		return errors.Wrapf(err, "ReadFile")
	}

	config := dashdog.Config{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return errors.Wrapf(err, "Unmarshal config, [%s]", string(data))
	}

	dash, err := dashdog.NewDash(config)
	if err != nil {
		return errors.Wrapf(err, "NewDash %+v", config)
	}

	err = dash.Build()
	if err != nil {
		return errors.Wrapf(err, "dash.Build")
	}

	return nil
}

func setLogLevel(cmd *cli.Command) {
	level := cmd.String(flagLog)

	lvl := &slog.LevelVar{}
	switch level {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "info":
		lvl.Set(slog.LevelInfo)
	case "warn":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	case "off":
		lvl.Set(logOffLevel)
	default:
		lvl.Set(logOffLevel)
	}

	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     lvl,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				s := a.Value.Any().(*slog.Source)
				s.File = path.Base(s.File)
			}
			return a
		},
	})
	logger := slog.New(h)
	slog.SetDefault(logger)
}

// func main() {
// 	lvl := &slog.LevelVar{}
// 	lvl.Set(slog.LevelInfo)
// 	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
// 		AddSource: true,
// 		Level:     lvl,
// 		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
// 			if a.Key == slog.SourceKey {
// 				s := a.Value.Any().(*slog.Source)
// 				s.File = path.Base(s.File)
// 			}
// 			return a
// 		},
// 	})
// 	logger := slog.New(h)
// 	slog.SetDefault(logger)

// 	config := dashdog.Config{
// 		Name: "testify",
// 		URL:  "https://pkg.go.dev/github.com/stretchr/testify",
// 		Plist: dashdog.Plist{
// 			CFBundleIdentifier:   "godoc",
// 			CFBundleName:         "stretchr/testify",
// 			DocSetPlatformFamily: "godoc",
// 			DashDocSetPlayURL:    "https://go.dev/play/",
// 			IsJavaScriptEnabled:  true,
// 		},
// 		Depth:        2,
// 		SubPathRegex: `^\/github\.com\/stretchr\/testify@v\d+\.\d+\.\d+/\w+$`,
// 		SubPathBundleNameReplace: dashdog.SubPathBundleNameReplace{
// 			Pattern: `^\/github\.com\/(stretchr\/testify)@v\d+\.\d+\.\d+/(\w+)$`,
// 			Replace: `$1/$2`,
// 		},
// 		Index: dashdog.Index{
// 			IndexRows: []dashdog.IndexRow{
// 				{
// 					Selector: "h3#pkg-index",
// 					Type:     "Section",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeConstant,
// 						Value: "Sections",
// 					},
// 					Level:      1,
// 					AnchorOnly: true,
// 				},
// 				{
// 					Selector: ".Documentation-indexConstants",
// 					Type:     "Section",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeConstant,
// 						Value: "Constants",
// 					},
// 					Level:      0,
// 					AnchorOnly: true,
// 				},
// 				{
// 					Selector: ".Documentation-indexVariables",
// 					Type:     "Section",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeConstant,
// 						Value: "Variables",
// 					},
// 					Level:      0,
// 					AnchorOnly: true,
// 				},
// 				{
// 					Selector: "h3#pkg-functions",
// 					Type:     "Function",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeConstant,
// 						Value: "Functions",
// 					},
// 					Level:      1,
// 					AnchorOnly: true,
// 				},
// 				{
// 					Selector: "h4[data-kind=function]",
// 					Type:     "Function",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeAttr,
// 						Value: "id",
// 					},
// 					Level: 0,
// 				},
// 				{
// 					Selector: "h4[data-kind=type]",
// 					Type:     "tdef",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeAttr,
// 						Value: "id",
// 					},
// 					Level:      1,
// 					AnchorOnly: true,
// 				},
// 				{
// 					Selector: "h4[data-kind=type]",
// 					Type:     "tdef",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeAttr,
// 						Value: "id",
// 					},
// 					Level: 0,
// 				},
// 				{
// 					Selector: "h4[data-kind=method]",
// 					Type:     "Function",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeAttr,
// 						Value: "id",
// 					},
// 					Level: 0,
// 				},
// 				{
// 					Selector: "h3#pkg-constants",
// 					Type:     "Constant",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeConstant,
// 						Value: "Constants",
// 					},
// 					Level:      1,
// 					AnchorOnly: true,
// 				},
// 				{
// 					Selector: "span[data-kind=constant]",
// 					Type:     "Constant",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeAttr,
// 						Value: "id",
// 					},
// 					Level: 0,
// 				},
// 				{
// 					Selector: "h3#pkg-variables",
// 					Type:     "Variable",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeConstant,
// 						Value: "Variables",
// 					},
// 					Level:      1,
// 					AnchorOnly: true,
// 				},
// 				{
// 					Selector: "span[data-kind=variable]",
// 					Type:     "Variable",
// 					Name: dashdog.IndexName{
// 						Type:  dashdog.IndexNameTypeAttr,
// 						Value: "id",
// 					},
// 					Level: 0,
// 				},
// 			},
// 		},
// 		Page: dashdog.Page{
// 			RemoveNodeSelector: []string{
// 				".go-Header",
// 				".go-Main-header",
// 				".go-Main-banner",
// 				".go-Main-nav",
// 				".go-Main-aside",
// 				".go-Main-footer",
// 				".go-Footer",
// 				".UnitReadme-content img[src*=badge]",
// 			},
// 			SetAttrs: []dashdog.SelectAttr{
// 				{
// 					Selector: ".go-Main",
// 					Attr: dashdog.Attr{
// 						Key:   "style",
// 						Value: "display: block",
// 					},
// 				},
// 			},
// 		},
// 	}

// 	dash, err := dashdog.NewDash(config)
// 	if err != nil {
// 		panic(err)
// 	}

// 	err = dash.Build()
// 	if err != nil {
// 		panic(err)
// 		// slog.ErrorContext(ctx, "build", slog.Any("config", config), slog.String("err", fmt.Sprintf("%+v", err)))
// 		// fmt.Printf("err:%+v\n", err)
// 	}
// }
