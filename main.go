package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
)

var (
	port          = flag.Int("port", 8080, "Port")
	resourcesPath = flag.String("resources-path", "resources/", "Resources path")
	templates     *template.Template
)

func main() {
	http.HandleFunc("/", indexHandler)
	fs := http.FileServer(http.Dir(path.Join(*resourcesPath, "static/")))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/query", queryHandler)

	log.Printf("Listening on port %d\n", *port)
	http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
}

func init() {
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
	query := r.FormValue("query")
	if len(query) <= 0 {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	if len(r.URL.Query()) <= 0 {
		http.Redirect(w, r, "/query?"+r.Form.Encode(), http.StatusTemporaryRedirect)
		return
	}
	values := map[string]string{
		"QueryValue": query,
	}
	err := templates.ExecuteTemplate(w, "index.html", values)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
