
# Ging

Like *Bing* (or *Google*, but *'G'* + *'Google'* is already taken by somebody
called *Google*) but for Go.

**Ging** is a search tool for [GoDoc](http://godoc.org/). You can use *Ging* to
look up inside each package available in GoDoc. Right now only a small set of
packages are indexed, but everybody can add a package to the index.

## Location

Right now *Ging* is accessible at http://ging.ngrok.com, but It's temporal.

## Implementation Details

*Ging* uses [bleve](blevesearch.com) for indexation and
[Leveldb](http://leveldb.org/) for storage. The http layer is written with help
of [gorilla/websocket](https://github.com/gorilla/websocket) to implement
auto-completion.
