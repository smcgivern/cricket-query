package main

import (
	"embed"
	"fmt"
	"github.com/jmoiron/sqlx"
	"html/template"
	"log"
	_ "modernc.org/sqlite"
	"net/http"
	"os"
)

var (
	db *sqlx.DB
	//go:embed all:template
	templatesFS embed.FS
)

type Result struct {
	Columns  []string
	Rows     [][]any
	Messages []string
}

type Page struct {
	Title   string
	Content any
}

func runQuery(query string, limit int) Result {
	messages := make([]string, 0)
	rows := make([][]any, 0)
	i := 1

	results, err := db.Queryx(query)
	if err != nil {
		return Result{Messages: []string{err.Error()}}
	}

	columns, err := results.Columns()
	if err != nil {
		messages = append(messages, err.Error())
	}

	for results.Next() {
		if i > limit {
			messages = append(messages, fmt.Sprintf("Too many rows returned; stopping at %d", limit))
			break
		}

		cols, err := results.SliceScan()
		if err != nil {
			messages = append(messages, err.Error())
		}

		rows = append(rows, cols)
		i += 1
	}

	return Result{
		Columns:  columns,
		Rows:     rows,
		Messages: messages,
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	query := r.FormValue("query")

	if query == "" {
		query = "SELECT * FROM men_test_batting_innings LIMIT 10;"
	}

	executeTemplate(w, "index.html", Page{
		Title: "Results",
		Content: struct {
			Query   string
			Results Result
		}{
			Query:   query,
			Results: runQuery(query, 9),
		},
	})
}

func schema(w http.ResponseWriter, r *http.Request) {
	executeTemplate(w, "schema.html", Page{
		Title:   "Schema",
		Content: runQuery("SELECT name, sql FROM sqlite_schema WHERE type = 'table';", 100),
	})
}

func executeTemplate(w http.ResponseWriter, path string, page Page) {
	template.
		Must(template.ParseFS(templatesFS, "template/_*.html", "template/"+path)).
		ExecuteTemplate(w, path, page)
}

func logRequests(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func init() {
	db = sqlx.MustConnect("sqlite", "data/innings.sqlite3")
}

func main() {
	port, exists := os.LookupEnv("PORT")

	if !exists {
		port = "8080"
	}

	http.HandleFunc("/", index)
	http.HandleFunc("/schema", schema)

	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%s", port), logRequests(http.DefaultServeMux)))
}
