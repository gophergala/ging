package main

import (
	"bytes"
	"errors"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/blevesearch/bleve"
	"github.com/golang/gddo/gosrc"
	"golang.org/x/net/html"
	"golang.org/x/oauth2"
)

const (
	githubAccessTokenVarName = "GING_GITHUB_ACCESSTOKEN"
	indexPath                = "index.bleve"
)

var (
	repositories = []string{
		// "github.com/gorilla/mux", // 39 func decls
		"github.com/gorilla/mux",
		"github.com/gorilla/websocket",
	}

	buildEnvs = []struct{ GOOS, GOARCH string }{
		{"linux", "amd64"},
		{"darwin", "amd64"},
		{"windows", "amd64"},
	}
)

func main() {
	index, err := getDefaultIndex()
	if err != nil {
		log.Fatalf("Error opening/creating the index: %s\n", err.Error())
		return
	}
	// TODO(alvivi): Remove this in production
	gosrc.SetLocalDevMode(os.Getenv("GOPATH"))

	tokenSource, err := newEnvTokenSource(githubAccessTokenVarName)
	if err != nil {
		log.Fatalln("A github access token is required. GING_GITHUB_ACCESSTOKEN.")
		return
	}
	client := oauth2.NewClient(oauth2.NoContext, tokenSource)

	log.Printf("Processing %d repositories...", len(repositories))
	for _, repPath := range repositories {
		log.Printf("Fetching package %s contents...", repPath)
		pkg, err := fetchPackage(client, repPath)
		if err != nil {
			log.Printf("Error: %s\n", err.Error())
			continue
		}
		pkgDesc := NewPackage(doc.New(pkg, repPath, 0))
		index.Index(pkgDesc.ImportPath, pkgDesc)
		log.Printf("Package %s indexed.\n", pkgDesc.ImportPath)

		// log.Println(pkgDoc.Doc)
		// for i, constDoc := range pkgDoc.Consts {
		// 	for j, constName := range constDoc.Names {
		// 		log.Printf("\t[const/%d-%d] %s()\n", i+1, j+1, constName)
		// 	}
		// }
		// for i, varDoc := range pkgDoc.Vars {
		// 	for j, varName := range varDoc.Names {
		// 		log.Printf("\t[var/%d-%d] %s()\n", i+1, j+1, varName)
		// 	}
		// }
		// for i, funcDoc := range pkgDoc.Funcs {
		// 	log.Printf("\t[func/%d] %s()\n", i+1, funcDoc.Name)
		// }
	}
	phrase := "implements"
	log.Printf("Search for '%s'\n", phrase)
	query := bleve.NewMatchPhraseQuery(phrase)
	search := bleve.NewSearchRequest(query)
	searchResults, err := index.Search(search)
	if err != nil {
		log.Fatalln(err.Error())
		return
	}
	log.Println(searchResults)
}

/*
	Indexing
*/

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

func getDefaultIndex() (bleve.Index, error) {
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

/*
  HTML Utilities
*/

func removeDocSourcecode(text string) string {
	buf := bytes.NewBufferString(text)
	root, err := html.Parse(buf)
	if err != nil {
		return text
	}
	var visit func(n *html.Node)
	visit = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "pre" {
			if n.Parent != nil {
				next := n.NextSibling
				n.Parent.RemoveChild(n)
				visit(next)
			}
		}
		if n.FirstChild != nil {
			visit(n.FirstChild)
		}
		if n.NextSibling != nil {
			visit(n.NextSibling)
		}
	}
	visit(root)
	rbuf := new(bytes.Buffer)
	html.Render(rbuf, root)
	return rbuf.String()
}

/*
	Fetching packages
*/

func fetchPackage(client *http.Client, path string) (*ast.Package, error) {
	dir, err := gosrc.Get(client, path, "")
	if err != nil {
		return nil, err
	}
	// Fisrt, we have to find a proper build context for the package
	bctx := build.Context{
		CgoEnabled:  true,
		ReleaseTags: build.Default.ReleaseTags,
		BuildTags:   build.Default.BuildTags,
		Compiler:    "gc",
	}
	var bpkg *build.Package
	for _, env := range buildEnvs {
		bctx.GOOS = env.GOOS
		bctx.GOARCH = env.GOARCH
		bpkg, err = dir.Import(&bctx, build.ImportComment)
		if _, ok := err.(*build.NoGoError); !ok {
			break
		}
	}
	// Parse all package's sourcecode
	fileSet := token.NewFileSet()
	filesData := map[string][]byte{}
	pkgFiles := map[string]*ast.File{}
	for _, file := range dir.Files {
		if strings.HasSuffix(file.Name, ".go") {
			gosrc.OverwriteLineComments(file.Data)
		}
		filesData[file.Name] = file.Data
		// TODO(alvivi): else { addReferences(references, file.Data) }
	}
	fileNames := append(bpkg.GoFiles, bpkg.CgoFiles...)
	for _, fname := range fileNames {
		pfile, err :=
			parser.ParseFile(fileSet, fname, filesData[fname], parser.ParseComments)
		if err != nil {
			return nil, err
		}
		pkgFiles[fname] = pfile
	}
	// Actually, we don't care about building the package. Only the parser have
	// to succeed to read its documentation.
	pkg, _ := ast.NewPackage(fileSet, pkgFiles, simpleImporter, nil)
	return pkg, nil
}

/*
	Github OAuth2 Utilities
*/

type envTokenSource struct {
	token *oauth2.Token
}

func newEnvTokenSource(envVarName string) (*envTokenSource, error) {
	accessToken := os.Getenv(envVarName)
	if len(accessToken) <= 0 {
		return nil, errors.New("No envvar found")
	}
	ts := envTokenSource{
		token: &oauth2.Token{
			AccessToken: accessToken,
		},
	}
	return &ts, nil
}

func (ts envTokenSource) Token() (*oauth2.Token, error) {
	return ts.token, nil
}

/*
	A stub importer implementation
*/

// simpleImporter is a importert which actually does not import anything.
// From github.com/golang/gddo/doc/builder.go, at line 335.
func simpleImporter(imports map[string]*ast.Object, path string) (*ast.Object, error) {
	pkg := imports[path]
	if pkg != nil {
		return pkg, nil
	}
	// Guess the package name without importing it.
	for _, pat := range packageNamePats {
		m := pat.FindStringSubmatch(path)
		if m != nil {
			pkg = ast.NewObj(ast.Pkg, m[1])
			pkg.Data = ast.NewScope(nil)
			imports[path] = pkg
			return pkg, nil
		}
	}
	return nil, errors.New("package not found")
}

// From github.com/golang/gddo/doc/builder.go, at line 315.
var packageNamePats = []*regexp.Regexp{
	regexp.MustCompile(`/([^-./]+)[-.](?:git|svn|hg|bzr|v\d+)$`),
	regexp.MustCompile(`/([^-./]+)[-.]go$`),
	regexp.MustCompile(`/go[-.]([^-./]+)$`),
	regexp.MustCompile(`^code\.google\.com/p/google-api-go-client/([^/]+)/v[^/]+$`),
	regexp.MustCompile(`^code\.google\.com/p/biogo\.([^/]+)$`),
	regexp.MustCompile(`([^/]+)$`),
}
