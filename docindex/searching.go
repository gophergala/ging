package docindex

import (
	"errors"
	"fmt"
	"html/template"
	"path"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/search"
)

// SearchResult ...
// TODO(alvivi): doc this
type SearchResult struct {
	Name       string
	Type       DocType
	Link       string
	Match      string
	Highlights SearchHighlights
}

// SearchHighlights ...
// TODO(alvivi): doc this
type SearchHighlights struct {
	Name    template.HTML
	Content template.HTML
}

// Search ...
// TODO(alvivi): doc this
func Search(index bleve.Index, queryString string) ([]*SearchResult, *bleve.SearchResult, error) {
	query := bleve.NewMatchPhraseQuery(queryString)
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{
		"name",
		"doctype",
		"import",
		"funcs",
	}
	search.Highlight = bleve.NewHighlightWithStyle("html")
	search.Explain = false
	sr, err := index.Search(search) // sr, err := ...
	if err != nil {
		return []*SearchResult{}, nil, err
	}
	entries := []*SearchResult{}
	for _, hit := range sr.Hits {
		entry, err := newSearchResult(hit.Fields, hit.Fragments)
		if err == nil {
			entries = append(entries, entry)
		}
	}
	return entries, sr, nil
}

func newSearchResult(fields map[string]interface{}, fragments search.FieldFragmentMap) (*SearchResult, error) {
	// Name
	nameValue, ok := fields["name"]
	if !ok {
		return nil, errors.New("Required field 'name' not found")
	}
	name := nameValue.(string)
	// Type
	doctypeValue, ok := fields["doctype"]
	if !ok {
		return nil, errors.New("Required field 'type' not found")
	}
	doctype := DocType(doctypeValue.(string))
	// Import Path
	importPathValue, ok := fields["import"]
	if !ok {
		return nil, errors.New("Required field 'import' not found")
	}
	importPath := importPathValue.(string)
	// Link
	var link string
	switch doctype {
	case PackageType:
		link = "http://" + path.Join("godoc.org/", importPath)
	case FuncType:
		basepath := "http://" + path.Join("godoc.org/", importPath)
		link = fmt.Sprintf("%s#%s", basepath, name)
	}
	// Highlights - Name
	var highlightName string
	if hname, ok := fragments["name"]; ok {
		highlightName = hname[0]
	}
	// Highlights - Content
	var highlightContent string
	if hcontent, ok := fragments["doc"]; ok {
		highlightContent = hcontent[0]
	}

	return &SearchResult{
		Name: name,
		Type: DocType(doctype),
		Link: link,
		Highlights: SearchHighlights{
			Name:    template.HTML(highlightName),
			Content: template.HTML(highlightContent),
		},
	}, nil
}
