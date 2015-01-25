package docindex

import (
	"fmt"
	"net/http"

	"go/doc"

	"github.com/blevesearch/bleve"
	"github.com/golang/gddo/gosrc"
)

// SetLocalDevMode sets the package to local development mode.
func SetLocalDevMode(path string) {
	gosrc.SetLocalDevMode(path)
}

// IndexPackage ...
// TODO(alvivi): doc this
func IndexPackage(client *http.Client, index bleve.Index, pkgPath string) error {
	pkg, err := fetchPackage(client, pkgPath)
	if err != nil {
		return err
	}
	pkgDesc := NewPackage(doc.New(pkg, pkgPath, 0))
	for _, fnDesc := range pkgDesc.Funcs {
		fnName := fmt.Sprintf("%s.%s", pkgDesc.ImportPath, fnDesc.Name)
		index.Index(fnName, fnDesc)
	}
	return index.Index(pkgDesc.ImportPath, pkgDesc)
}
