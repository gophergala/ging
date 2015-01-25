package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/blevesearch/bleve"
	"github.com/gophergala/ging/docindex"
	"github.com/gophergala/ging/utils/envtokensource"
	"golang.org/x/oauth2"
)

const (
	githubAccessTokenVarName = "GING_GITHUB_ACCESSTOKEN"
)

var (
	port            = flag.Int("port", 8080, "Port")
	resourcesPath   = flag.String("resources-path", "resources/", "Resources path")
	indexPrefix     = flag.String("index-prefix", ".", "Indexes path")
	docindexName    = flag.String("docindex", "docindex.bleve", "Docindex path")
	localDevMode    = flag.Bool("local", false, "Enable local development mode")
	fetchFilePath   = flag.String("fetch-file", "", "Fetch and index package from the specified file")
	templates       *template.Template
	index           bleve.Index
	indexationMutex = new(sync.Mutex)
)

func main() {
	var err error
	index, err = docindex.OpenOrCreateIndex(path.Join(*indexPrefix, *docindexName))
	if err != nil {
		log.Fatalln(err.Error())
	}
	fetchPackagesFromFetchFile()

	http.HandleFunc("/", indexHandler)
	fs := http.FileServer(http.Dir(path.Join(*resourcesPath, "static/")))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/query", queryHandler)
	http.HandleFunc("/package/add", addPackageHandle)

	log.Printf("Listening on port %d\n", *port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func fetchPackagesFromFetchFile() {
	if len(*fetchFilePath) <= 0 {
		return
	}
	f, err := os.Open(*fetchFilePath)
	if err != nil {
		log.Printf("Error opening fetch-file: %s.\n", err.Error())
		return
	}
	defer f.Close()
	repList := []struct {
		Path string `json:"path"`
	}{}
	err = json.NewDecoder(f).Decode(&repList)
	if err != nil {
		log.Printf("Error reading fetch-file: %s.\n", err.Error())
		return
	}
	for _, rep := range repList {
		fetchPackage(rep.Path)
	}
}

func fetchPackage(pacakgePath string) {
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
	err := docindex.IndexPackage(client, index, pacakgePath)
	if err != nil {
		log.Printf("Error indexing package %s: %s.\n", pacakgePath, err.Error())
		return
	}
	log.Printf("Package %s indexed.\n", pacakgePath)
}

func init() {
	flag.Parse()
	templates = template.Must(template.ParseFiles(
		path.Join(*resourcesPath, "templates/head.html"),
		path.Join(*resourcesPath, "templates/navbar.html"),
		path.Join(*resourcesPath, "templates/query-input.html"),
		path.Join(*resourcesPath, "templates/scripts.html"),
		path.Join(*resourcesPath, "templates/index.html"),
		path.Join(*resourcesPath, "templates/query.html"),
		path.Join(*resourcesPath, "templates/package-add.html"),
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

	results, sr, err := docindex.Search(index, queryString)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	subtitle :=
		fmt.Sprintf("<strong>%d</strong> results in <strong>%s</strong>", sr.Total, sr.Took)
	values := map[string]interface{}{
		"QueryValue": queryString,
		"Subtitle":   template.HTML(subtitle),
		"Results":    results,
	}
	err = templates.ExecuteTemplate(w, "query.html", values)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func addPackageHandle(w http.ResponseWriter, r *http.Request) {
	packageName := r.FormValue("package")
	vars := map[string]string{}
	if len(packageName) > 0 {
		go func() {
			indexationMutex.Lock()
			fetchPackage(packageName)
			indexationMutex.Unlock()
		}()
		vars["PackageName"] = packageName
	}
	err := templates.ExecuteTemplate(w, "package-add.html", vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
