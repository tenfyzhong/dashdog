package dashdog

import (
	"database/sql"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/go-resty/resty/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func init() {
	lvl := &slog.LevelVar{}
	lvl.Set(16)
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
}

func TestDash_Build(t *testing.T) {
	t.Run("tree.Rm failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Rm", func() error {
			return errors.New("Rm failed")
		})

		err = d.Build()
		require.ErrorContains(t, err, "rm")
	})

	t.Run("tree.Mkdir failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Rm", func() error {
			return nil
		})
		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Mkdir", func() error {
			return errors.New("make dir failed")
		})

		err = d.Build()
		require.ErrorContains(t, err, "mkdir")
	})

	t.Run("createDB failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Rm", func() error {
			return nil
		})
		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Mkdir", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(d), "createDB", func() error {
			return errors.New("create db failed")
		})

		err = d.Build()
		require.ErrorContains(t, err, "createDB")
	})

	t.Run("parse url failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Rm", func() error {
			return nil
		})
		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Mkdir", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(d), "createDB", func() error {
			return nil
		})
		patches.ApplyFunc(url.Parse, func(string) (*url.URL, error) {
			return nil, errors.New("url error")
		})

		err = d.Build()
		require.ErrorContains(t, err, "Parse https://github.com/stretchr/testify/require")
	})

	t.Run("infoPlist failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Rm", func() error {
			return nil
		})
		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Mkdir", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(d), "createDB", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(*d), "infoPlist", func() error {
			return errors.New("info list failed")
		})

		err = d.Build()
		require.ErrorContains(t, err, "infoPlist")
		require.Equal(t, "github.com/stretchr/testify/require.html", d.indexFilePath)
	})

	t.Run("populateData error", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Rm", func() error {
			return nil
		})
		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Mkdir", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(d), "createDB", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(*d), "infoPlist", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(d), "populateData", func() error {
			return errors.New("err")
		})

		err = d.Build()
		require.ErrorContains(t, err, "populateData")
		require.Equal(t, "github.com/stretchr/testify/require.html", d.indexFilePath)
	})

	t.Run("succ", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Rm", func() error {
			return nil
		})
		patches.ApplyMethod(reflect.TypeOf(*d.tree), "Mkdir", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(d), "createDB", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(*d), "infoPlist", func() error {
			return nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(d), "populateData", func() error {
			return nil
		})

		err = d.Build()
		require.NoError(t, err)
		require.Equal(t, "github.com/stretchr/testify/require.html", d.indexFilePath)
	})
}

func TestDash_createDB(t *testing.T) {
	t.Run("open db failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyFunc(sql.Open, func(string, string) (*sql.DB, error) {
			return nil, errors.New("open db failed")
		})

		err = d.createDB()
		require.ErrorContains(t, err, "Open db")
	})

	t.Run("create table failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyFunc(sql.Open, func(string, string) (*sql.DB, error) {
			return &sql.DB{}, nil
		})

		patches.ApplyMethodFunc(reflect.TypeOf(d.db), "Exec", func(query string, args ...any) (sql.Result, error) {
			return nil, errors.New("db exec")
		})

		err = d.createDB()
		require.ErrorContains(t, err, "create table")
		require.Nil(t, d.db)
	})

	t.Run("create index failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyFunc(sql.Open, func(string, string) (*sql.DB, error) {
			return &sql.DB{}, nil
		})

		output := []gomonkey.OutputCell{
			{
				Values: []interface{}{nil, nil},
				Times:  1,
			},
			{
				Values: []interface{}{nil, errors.New("db exec")},
				Times:  1,
			},
		}

		patches.ApplyMethodSeq(reflect.TypeOf(d.db), "Exec", output)

		err = d.createDB()
		require.ErrorContains(t, err, "create unique index")
		require.Nil(t, d.db)
	})

	t.Run("delete content failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyFunc(sql.Open, func(string, string) (*sql.DB, error) {
			return &sql.DB{}, nil
		})

		output := []gomonkey.OutputCell{
			{
				Values: []interface{}{nil, nil},
				Times:  1,
			},
			{
				Values: []interface{}{nil, nil},
				Times:  1,
			},
			{
				Values: []interface{}{nil, errors.New("Exec")},
				Times:  1,
			},
		}

		patches.ApplyMethodSeq(reflect.TypeOf(d.db), "Exec", output)

		err = d.createDB()
		require.ErrorContains(t, err, "delete rows")
		require.Nil(t, d.db)
	})

	t.Run("createDB success", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches.ApplyFunc(sql.Open, func(string, string) (*sql.DB, error) {
			return &sql.DB{}, nil
		})

		output := []gomonkey.OutputCell{
			{
				Values: []interface{}{nil, nil},
				Times:  1,
			},
			{
				Values: []interface{}{nil, nil},
				Times:  1,
			},
			{
				Values: []interface{}{nil, nil},
				Times:  1,
			},
		}

		patches.ApplyMethodSeq(reflect.TypeOf(d.db), "Exec", output)

		err = d.createDB()
		require.NoError(t, err)
		require.NotNil(t, d.db)
	})
}

func Test_localPathOfURL(t *testing.T) {
	type args struct {
		u      *url.URL
		suffix string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "has suffix",
			args: args{
				u: &url.URL{
					Host: "github.com",
					Path: "/stretchr/testify/require.html",
				},
				suffix: ".html",
			},
			want: "github.com/stretchr/testify/require.html",
		},
		{
			name: "no suffix",
			args: args{
				u: &url.URL{
					Host: "github.com",
					Path: "/stretchr/testify/require",
				},
				suffix: ".html",
			},
			want: "github.com/stretchr/testify/require.html",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := localPathOfURL(tt.args.u, tt.args.suffix); got != tt.want {
				t.Errorf("localPathOfURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDash_saveFile(t *testing.T) {
	t.Run("mkdir all failed", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		localPath := "github.com/stretchr/testify/require/hello.ico"

		d, err := NewDash(Config{
			Path:              tmpDir,
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyFunc(os.MkdirAll, func(string, os.FileMode) error {
			return errors.New("unknown error")
		})

		err = d.saveFile(localPath, []byte("hello"))
		require.ErrorContains(t, err, "MkdirAll")
	})

	t.Run("file exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		localPath := "github.com/stretchr/testify/require/hello.ico"

		d, err := NewDash(Config{
			Path:              tmpDir,
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		dirname := filepath.Dir(localPath)
		absPath := filepath.Join(d.tree.Documents(), dirname)
		err = os.MkdirAll(absPath, 0755)
		require.NoError(t, err)
		f, err := os.Create(filepath.Join(absPath, "hello.ico"))
		require.NoError(t, err)
		f.Close()

		err = d.saveFile(localPath, []byte("hello"))
		require.NoError(t, err)
	})

	t.Run("stat unknown error", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		localPath := "github.com/stretchr/testify/require/hello.ico"

		d, err := NewDash(Config{
			Path:              tmpDir,
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyFunc(os.MkdirAll, func(string, os.FileMode) error {
			return nil
		})
		patches.ApplyFunc(os.Stat, func(string) (os.FileInfo, error) {
			return nil, errors.New("unknown error")
		})

		err = d.saveFile(localPath, []byte("hello"))
		require.ErrorContains(t, err, "Stat file")
	})

	t.Run("write file failed", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		localPath := "github.com/stretchr/testify/require/hello.ico"

		d, err := NewDash(Config{
			Path:              tmpDir,
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyFunc(os.WriteFile, func(string, []byte, os.FileMode) error {
			return errors.New("write")
		})

		err = d.saveFile(localPath, []byte("hello"))
		require.ErrorContains(t, err, "WriteFile")
	})

	t.Run("succ", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		localPath := "github.com/stretchr/testify/require/hello.ico"

		d, err := NewDash(Config{
			Path:              tmpDir,
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify/require",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		err = d.saveFile(localPath, []byte("hello"))
		require.NoError(t, err)
	})
}

func TestDash_populateData(t *testing.T) {
	t.Run("downloaded", func(t *testing.T) {
		item := &fetchItem{
			u: &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "/stretchr/testify",
			},
			localPath:    "github.com/stretchr/testify.html",
			level:        0,
			needPopulate: false,
		}

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		d.downloaded["github.com/stretchr/testify"] = true
		require.NoError(t, err)
		require.NotNil(t, d)

		err = d.populateData(item)
		require.NoError(t, err)
	})

	t.Run("get url failed", func(t *testing.T) {
		item := &fetchItem{
			u: &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "/stretchr/testify",
			},
			localPath:    "github.com/stretchr/testify.html",
			level:        0,
			needPopulate: false,
		}

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyMethodFunc(reflect.TypeOf(d.httpClient.R()), "Get", func(string) (*resty.Response, error) {
			return nil, errors.New("unknown")
		})

		err = d.populateData(item)
		require.ErrorContains(t, err, "get https://github.com/stretchr/testify")
	})

	t.Run("get url status 502", func(t *testing.T) {
		item := &fetchItem{
			u: &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "/stretchr/testify",
			},
			localPath:    "github.com/stretchr/testify.html",
			level:        0,
			needPopulate: false,
		}

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyMethodFunc(reflect.TypeOf(d.httpClient.R()), "Get", func(string) (*resty.Response, error) {
			return &resty.Response{
				RawResponse: &http.Response{
					StatusCode: 502,
				},
			}, nil
		})

		err = d.populateData(item)
		require.ErrorContains(t, err, "https://github.com/stretchr/testify status 502")
	})

	t.Run("not need populate and save file failed", func(t *testing.T) {
		item := &fetchItem{
			u: &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "/stretchr/testify",
			},
			localPath:    "github.com/stretchr/testify.html",
			level:        0,
			needPopulate: false,
		}

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyMethodFunc(reflect.TypeOf(d.httpClient.R()), "Get", func(string) (*resty.Response, error) {
			return &resty.Response{
				RawResponse: &http.Response{
					StatusCode: 200,
				},
			}, nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(*d), "saveFile", func(string, []byte) error {
			return errors.New("unknwon save failed")
		})

		err = d.populateData(item)
		require.ErrorContains(t, err, "saveFile")
	})

	t.Run("not need populate and save file succ", func(t *testing.T) {
		item := &fetchItem{
			u: &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "/stretchr/testify",
			},
			localPath:    "github.com/stretchr/testify.html",
			level:        0,
			needPopulate: false,
		}

		d, err := NewDash(Config{
			Path:              "",
			Name:              "bundle",
			URL:               "https://github.com/stretchr/testify",
			Plist:             Plist{},
			Index:             Index{},
			Page:              Page{},
			Depth:             1,
			SubPathRegex:      "",
			SubPathBundleName: SubPathBundleName{},
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyMethodFunc(reflect.TypeOf(d.httpClient.R()), "Get", func(string) (*resty.Response, error) {
			return &resty.Response{
				RawResponse: &http.Response{
					StatusCode: 200,
				},
			}, nil
		})
		patches.ApplyPrivateMethod(reflect.TypeOf(*d), "saveFile", func(string, []byte) error {
			return nil
		})

		err = d.populateData(item)
		require.NoError(t, err)
	})
}
