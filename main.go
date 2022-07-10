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
	//go:embed template
	templatesFS embed.FS
	templates   *template.Template
)

type Result struct {
	Columns  []string
	Rows     [][]any
	Messages []string
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

func table(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	query := r.FormValue("query")

	if query == "" {
		query = "SELECT * FROM men_test_batting_innings LIMIT 10;"
	}

	templates.ExecuteTemplate(w, "table.html", runQuery(query, 9))
}

func schema(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "table.html", runQuery("SELECT name, sql FROM sqlite_schema WHERE type = 'table';", 100))
}

func logRequests(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func init() {
	db = sqlx.MustConnect("sqlite", "data/innings.sqlite3")

	templates = template.Must(template.ParseFS(templatesFS, "template/*"))
}

func main() {
	port, exists := os.LookupEnv("PORT")

	if !exists {
		port = "8080"
	}

	http.HandleFunc("/", table)
	http.HandleFunc("/schema", schema)

	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%s", port), logRequests(http.DefaultServeMux)))
}
