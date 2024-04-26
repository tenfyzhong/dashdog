package main

import (
	"log/slog"
	"os"
	"path"

	cli "github.com/urfave/cli/v3"

	"github.com/tenfyzhong/dashdog"
)

func main() {
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelInfo)
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
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

	config := dashdog.Config{
		Name: "testify",
		URL:  "https://pkg.go.dev/github.com/stretchr/testify",
		Plist: dashdog.Plist{
			CFBundleIdentifier:   "godoc",
			CFBundleName:         "stretchr/testify",
			DocSetPlatformFamily: "godoc",
			DashDocSetPlayURL:    "https://go.dev/play/",
			IsJavaScriptEnabled:  true,
		},
		Depth:        2,
		SubPathRegex: `^\/github\.com\/stretchr\/testify@v\d+\.\d+\.\d+/\w+$`,
		SubPathBundleNameReplace: dashdog.SubPathBundleNameReplace{
			Pattern: `^\/github\.com\/(stretchr\/testify)@v\d+\.\d+\.\d+/(\w+)$`,
			Replace: `$1/$2`,
		},
		Index: dashdog.Index{
			IndexRows: []dashdog.IndexRow{
				{
					Selector: "h3#pkg-index",
					Type:     "Section",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeConstant,
						Value: "Sections",
					},
					Level:      1,
					AnchorOnly: true,
				},
				{
					Selector: ".Documentation-indexConstants",
					Type:     "Section",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeConstant,
						Value: "Constants",
					},
					Level:      0,
					AnchorOnly: true,
				},
				{
					Selector: ".Documentation-indexVariables",
					Type:     "Section",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeConstant,
						Value: "Variables",
					},
					Level:      0,
					AnchorOnly: true,
				},
				{
					Selector: "h3#pkg-functions",
					Type:     "Function",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeConstant,
						Value: "Functions",
					},
					Level:      1,
					AnchorOnly: true,
				},
				{
					Selector: "h4[data-kind=function]",
					Type:     "Function",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeAttr,
						Value: "id",
					},
					Level: 0,
				},
				{
					Selector: "h4[data-kind=type]",
					Type:     "tdef",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeAttr,
						Value: "id",
					},
					Level:      1,
					AnchorOnly: true,
				},
				{
					Selector: "h4[data-kind=type]",
					Type:     "tdef",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeAttr,
						Value: "id",
					},
					Level: 0,
				},
				{
					Selector: "h4[data-kind=method]",
					Type:     "Function",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeAttr,
						Value: "id",
					},
					Level: 0,
				},
				{
					Selector: "h3#pkg-constants",
					Type:     "Constant",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeConstant,
						Value: "Constants",
					},
					Level:      1,
					AnchorOnly: true,
				},
				{
					Selector: "span[data-kind=constant]",
					Type:     "Constant",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeAttr,
						Value: "id",
					},
					Level: 0,
				},
				{
					Selector: "h3#pkg-variables",
					Type:     "Variable",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeConstant,
						Value: "Variables",
					},
					Level:      1,
					AnchorOnly: true,
				},
				{
					Selector: "span[data-kind=variable]",
					Type:     "Variable",
					Name: dashdog.IndexName{
						Type:  dashdog.IndexNameTypeAttr,
						Value: "id",
					},
					Level: 0,
				},
			},
		},
		Page: dashdog.Page{
			RemoveNodeSelector: []string{
				".go-Header",
				".go-Main-header",
				".go-Main-banner",
				".go-Main-nav",
				".go-Main-aside",
				".go-Main-footer",
				".go-Footer",
				".UnitReadme-content img[src*=badge]",
			},
			SetAttrs: []dashdog.SelectAttr{
				{
					Selector: ".go-Main",
					Attr: dashdog.Attr{
						Key:   "style",
						Value: "display: block",
					},
				},
			},
		},
	}

	dash, err := dashdog.NewDash(config)
	if err != nil {
		panic(err)
	}

	err = dash.Build()
	if err != nil {
		panic(err)
		// slog.ErrorContext(ctx, "build", slog.Any("config", config), slog.String("err", fmt.Sprintf("%+v", err)))
		// fmt.Printf("err:%+v\n", err)
	}
}
