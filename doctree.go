package dashdog

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

type docTree struct {
	name string

	documents string
	plist     string
	db        string
}

func newDocTree(name string) *docTree {
	return &docTree{
		name:      name,
		documents: fmt.Sprintf("%s.docset/Contents/Resources/Documents/", name),
		plist:     fmt.Sprintf("%s.docset/Contents/Info.plist", name),
		db:        fmt.Sprintf("%s.docset/Contents/Resources/docSet.dsidx", name),
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
