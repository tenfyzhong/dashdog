package dashdog

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type docTree struct {
	path string
	name string

	documents string
	plist     string
	db        string
}

func newDocTree(path, name string) *docTree {
	if path == "" {
		path, _ = os.Getwd()
	}

	docset := fmt.Sprintf("%s.docset", name)
	documents := filepath.Join(path, docset, "Contents", "Resources", "Documents")
	plist := filepath.Join(path, docset, "Info.plist")
	db := filepath.Join(path, docset, "Contents", "Resources", "docSet.dsidx")

	return &docTree{
		path:      path,
		name:      name,
		documents: documents,
		plist:     plist,
		db:        db,
	}
}

func (t docTree) Documents() string {
	return t.documents
}

func (t docTree) InfoPlist() string {
	return t.plist
}

func (t docTree) DB() string {
	return t.db
}

func (t docTree) Mkdir() error {
	err := os.MkdirAll(t.documents, 0755)
	if err != nil {
		return errors.Wrapf(err, "MkdirAll %s", t.documents)
	}
	return nil
}

func (t docTree) Rm() error {
	err := os.RemoveAll(t.documents)
	if err != nil {
		return errors.Wrapf(err, "RemoveAll %s", t.documents)
	}
	return nil
}
