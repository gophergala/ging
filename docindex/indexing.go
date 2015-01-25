package docindex

import (
	"bytes"
	"go/doc"

	"github.com/blevesearch/bleve"
)

// DocType ...
// TODO(alvivi): doc this
type DocType string

const (
	// PackageType ...
	// TODO(alvivi): doc this
	PackageType DocType = "p"

	// FuncType ...
	// TODO(alvivi): doc this
	FuncType DocType = "f"
)

// Package ...
// TODO(alvivi): doc this
type Package struct {
	Doc        string  `json:"doc"`
	Name       string  `json:"name"`
	ImportPath string  `json:"import"`
	DocType    DocType `json:"doctype"`

	Funcs []*Func `json:"funcs"`
}

// NewPackage ...
// TODO(alvivi): doc this
func NewPackage(pkgDoc *doc.Package) *Package {
	pkg := new(Package)
	pkg.DocType = PackageType
	pkg.Name = pkgDoc.Name
	pkg.ImportPath = pkgDoc.ImportPath
	buf := new(bytes.Buffer)
	doc.ToHTML(buf, pkgDoc.Doc, nil)
	pkg.Doc = removeDocSourcecode(buf.String())
	// Top level functions
	funcs := make([]*Func, len(pkgDoc.Funcs))
	for i, fn := range pkgDoc.Funcs {
		funcs[i] = NewFunction(pkg, fn)
	}
	pkg.Funcs = funcs
	return pkg
}

// Type ...
// TODO(alvivi): doc this
func (pkg Package) Type() string {
	return "package"
}

// Func ...
// TODO(alvivi): doc this
type Func struct {
	Doc        string  `json:"doc"`
	Name       string  `json:"name"`
	ImportPath string  `json:"import"`
	DocType    DocType `json:"doctype"`
}

// NewFunction ...
// TODO(alvivi): doc this
func NewFunction(pkg *Package, fn *doc.Func) *Func {
	return &Func{
		Doc:        fn.Doc,
		Name:       fn.Name,
		ImportPath: pkg.ImportPath,
		DocType:    FuncType,
	}
}

// Type ...
// TODO(alvivi): doc this
func (pkg Func) Type() string {
	return "func"
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

	// a generic reusable mapping which only stores (but no index) a text
	noindexTextFieldMapping := bleve.NewTextFieldMapping()
	noindexTextFieldMapping.Store = true
	noindexTextFieldMapping.Index = false

	// Function Mapping
	funcMapping := bleve.NewDocumentStaticMapping()
	funcMapping.AddFieldMappingsAt("name", keywordFieldMapping)
	funcMapping.AddFieldMappingsAt("doc", docFieldMapping)
	funcMapping.AddFieldMappingsAt("doctype", noindexTextFieldMapping)

	// Package Mapping
	packageMapping := bleve.NewDocumentStaticMapping()
	packageMapping.AddFieldMappingsAt("name", keywordFieldMapping)
	// TODO(alvivi): ImportPath must be indexed, but requires a custom analayzer
	// that removes the host.
	packageMapping.AddFieldMappingsAt("import", noindexTextFieldMapping)
	packageMapping.AddFieldMappingsAt("doc", docFieldMapping)
	packageMapping.AddFieldMappingsAt("doctype", noindexTextFieldMapping)
	packageMapping.AddSubDocumentMapping("funcs", funcMapping)

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
	indexMapping.AddDocumentMapping("func", funcMapping)
	return indexMapping, nil
}
