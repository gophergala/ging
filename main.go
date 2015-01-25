package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/blevesearch/bleve"
	"github.com/gophergala/ging/docindex"
	"github.com/gophergala/ging/utils/envtokensource"
	"golang.org/x/oauth2"
)

const (
	githubAccessTokenVarName = "GING_GITHUB_ACCESSTOKEN"
)

var (
	port          = flag.Int("port", 8080, "Port")
	resourcesPath = flag.String("resources-path", "resources/", "Resources path")
	indexPrefix   = flag.String("index-prefix", ".", "Indexes path")
	docindexName  = flag.String("docindex", "docindex.bleve", "Docindex path")
	localDevMode  = flag.Bool("local", false, "Enable local development mode")
	templates     *template.Template
	index         bleve.Index

	repList = []string{
		"github.com/gorilla/mux",
		"github.com/gorilla/websocket",
	}
)

func main() {
	var client *http.Client
	if *localDevMode {
		log.Println("Local development mode enabled")
		docindex.SetLocalDevMode(os.Getenv("GOPATH"))
		client = http.DefaultClient
	} else {
		tokenSource, err := envtokensource.NewEnvTokenSource(githubAccessTokenVarName)
		if err != nil {
			log.Fatalln("A github access token is required. GING_GITHUB_ACCESSTOKEN.")
		}
		client = oauth2.NewClient(oauth2.NoContext, tokenSource)
	}
	var err error
	index, err = docindex.OpenOrCreateIndex(path.Join(*indexPrefix, *docindexName))
	if err != nil {
		log.Fatalln(err.Error())
	}
	for _, rep := range repList {
		err := docindex.IndexPackage(client, index, rep)
		if err != nil {
			log.Printf("Error indexing package %s: %s.\n", rep, err.Error())
			continue
		}
		log.Printf("Package %s indexed.\n", rep)
	}

	http.HandleFunc("/", indexHandler)
	fs := http.FileServer(http.Dir(path.Join(*resourcesPath, "static/")))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/query", queryHandler)

	log.Printf("Listening on port %d\n", *port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func init() {
	flag.Parse()
	templates = template.Must(template.ParseFiles(
		path.Join(*resourcesPath, "templates/head.html"),
		path.Join(*resourcesPath, "templates/navbar.html"),
		path.Join(*resourcesPath, "templates/query.html"),
		path.Join(*resourcesPath, "templates/scripts.html"),
		path.Join(*resourcesPath, "templates/index.html"),
	))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	queryString := r.FormValue("query")
	if len(queryString) <= 0 {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	if len(r.URL.Query()) <= 0 {
		http.Redirect(w, r, "/query?"+r.Form.Encode(), http.StatusTemporaryRedirect)
		return
	}
	values := map[string]string{
		"QueryValue": queryString,
	}
	err := templates.ExecuteTemplate(w, "index.html", values)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Search for '%s'\n", queryString)
	query := bleve.NewMatchPhraseQuery(queryString)
	search := bleve.NewSearchRequest(query)
	searchResults, err := index.Search(search)
	if err != nil {
		log.Fatalln(err.Error())
		return
	}
	log.Println(searchResults)
}
