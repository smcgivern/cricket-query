package main

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"html/template"
	"net/http"
	"os"
	"log"
	_ "modernc.org/sqlite"
)

var db *sqlx.DB

type Result struct {
	Columns []string
	Rows [][]interface{}
	Messages []string
}

func runQuery(query string, limit int) Result {
	messages := make([]string, 0)
	rows := make([][]interface{}, 0)
	i := 1

	results, err := db.Queryx(query)
	if err != nil {
		return Result{ Messages: []string{err.Error()} }
	}

	columns, err := results.Columns()
	if err != nil {
		messages = append(messages, err.Error())
	}

	for results.Next() {
		if i > limit {
			messages = append(messages, fmt.Sprintf("more rows than limit (%d)", limit))
			break
		}

		cols, err := results.SliceScan()
		if err != nil {
			messages = append(messages, err.Error())
		}

		rows = append(rows, cols)
		i += 1
	}

	return Result {
		Columns: columns,
			Rows: rows,
			Messages: messages,
	}
}

func table(w http.ResponseWriter, r *http.Request) {
	template := template.Must(template.New("table.html").ParseFiles("template/table.html"))

	template.Execute(w, runQuery("SELECT * FROM men_test_batting_innings LIMIT 10;", 9))
}

func logRequests(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func main() {
	db = sqlx.MustConnect("sqlite", "data/innings.sqlite3")

	port, exists := os.LookupEnv("PORT")

	if !exists {
		port = "8080"
	}

	http.HandleFunc("/", table)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), logRequests(http.DefaultServeMux)))
}
