package docindex

import (
	"bytes"
	"go/doc"

	"github.com/blevesearch/bleve"
)

// Package ...
// TODO(alvivi): doc this
type Package struct {
	Doc        string `json:"doc"`
	Name       string `json:"name"`
	ImportPath string `json:"-"`
}

// NewPackage ...
// TODO(alvivi): doc this
func NewPackage(pkg *doc.Package) *Package {
	buf := new(bytes.Buffer)
	doc.ToHTML(buf, pkg.Doc, nil)
	pkgDoc := removeDocSourcecode(buf.String())
	return &Package{
		Doc:        pkgDoc,
		Name:       pkg.Name,
		ImportPath: pkg.ImportPath,
	}
}

// Type ...
// TODO(alvivi): doc this
func (pkg Package) Type() string {
	return "package"
}

// OpenOrCreateIndex ...
// TODO(alvivi): doc this
func OpenOrCreateIndex(indexPath string) (bleve.Index, error) {
	idx, err := bleve.Open(indexPath)
	if err == nil {
		return idx, nil
	}
	mapping, err := buildDefaultMapping()
	if err != nil {
		return nil, err
	}
	return bleve.New(indexPath, mapping)
}

func buildDefaultMapping() (*bleve.IndexMapping, error) {
	// a generic reusable mapping for docucmentation content
	docFieldMapping := bleve.NewTextFieldMapping()
	docFieldMapping.Analyzer = "doc"

	// a generic reusable mapping for keyword text
	keywordFieldMapping := bleve.NewTextFieldMapping()
	keywordFieldMapping.Analyzer = "keyword"

	// Package Mapping
	packageMapping := bleve.NewDocumentStaticMapping()
	packageMapping.AddFieldMappingsAt("name", keywordFieldMapping)
	// TODO(alvivi): ImportPath must be indexed, but requires a custom analayzer
	// that removes the host.
	packageMapping.AddFieldMappingsAt("doc", docFieldMapping)

	// Index Mapping
	indexMapping := bleve.NewIndexMapping()
	err := indexMapping.AddCustomAnalyzer("doc",
		map[string]interface{}{
			"type":          "custom",
			"char_filters":  []string{"html"},
			"tokenizer":     "whitespace",
			"token_filters": []string{"to_lower", "stop_en"},
		})
	if err != nil {
		return nil, err
	}
	indexMapping.AddDocumentMapping("package", packageMapping)
	return indexMapping, nil
}
