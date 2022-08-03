package main

import (
	"embed"
	"fmt"
	"github.com/jmoiron/sqlx"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"html/template"
	"log"
	_ "modernc.org/sqlite"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
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

type LabelledResult struct {
	Header string
	Result Result
}

type Page struct {
	Title   string
	Content any
}

type Checkbox struct {
	Label   string
	Value   string
	Checked bool
}

var formatValues = []Checkbox{
	Checkbox{"Test", "test", false},
	Checkbox{"ODI", "odi", false},
	Checkbox{"T20I", "t20i", false},
}

var genderValues = []Checkbox{
	Checkbox{"Men", "men", false},
	Checkbox{"Women", "women", false},
}

var startsWithWith = regexp.MustCompile(`(?i)\AWITH`)

var matchDate = regexp.MustCompile(`\A\d{4}-\d{2}-\d{2}( 00:00:00 \+0000 UTC)?\z`)
var matchInteger = regexp.MustCompile(`\A\d+\z`)
var matchFloat = regexp.MustCompile(`\A\d+\.\d+\z`)

func format(value any) string {
	text := fmt.Sprint(value)
	bytes := []byte(text)

	if value == nil {
		return ""
	} else if matchDate.Match(bytes) {
		t, err := time.Parse("2006-01-02 15:04:05 +0000 UTC", text)

		if err != nil {
			t, err = time.Parse("2006-01-02", text)

			if err != nil {
				return text
			}
		}

		return t.Format("2 January 2006")

	} else if matchInteger.Match(bytes) {
		int, err := strconv.Atoi(text)

		if err != nil {
			return text
		}

		printer := message.NewPrinter(language.English)

		return printer.Sprintf("%d", int)
	} else if matchFloat.Match(bytes) {
		float, err := strconv.ParseFloat(text, 64)

		if err != nil {
			return text
		}

		printer := message.NewPrinter(language.English)

		return printer.Sprintf("%.2f", float)
	}

	return text
}

func projectQuery(formats []Checkbox, genders []Checkbox, query string, limit int) (out []LabelledResult) {
	for _, format := range formats {
		for _, gender := range genders {
			if format.Checked && gender.Checked {
				out = append(out, LabelledResult{
					fmt.Sprintf("%s's %s", gender.Label, format.Label),
					runQuery(addAliases(gender.Value, format.Value, query), limit),
				})
			}
		}
	}

	return
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

func addAliases(gender string, format string, query string) string {
	if startsWithWith.Match([]byte(query)) {
		query = startsWithWith.ReplaceAllString(query, ",")
	}

	return fmt.Sprintf(`
WITH
innings AS (SELECT * FROM %[1]s_%[2]s_batting_innings),
bowling_innings AS (SELECT * FROM %[1]s_%[2]s_bowling_innings),
team_innings AS (SELECT * FROM %[1]s_%[2]s_team_innings)
%[3]s
`, gender, format, query)
}

func inArray(needle string, haystack []string) bool {
	for _, value := range haystack {
		if value == needle {
			return true
		}
	}

	return false
}

func checkboxValues(checkboxes []Checkbox, checked []string) (out []Checkbox) {
	for _, checkbox := range checkboxes {
		out = append(
			out,
			Checkbox{
				checkbox.Label,
				checkbox.Value,
				len(checked) == 0 || inArray(checkbox.Value, checked),
			},
		)
	}

	return
}

func index(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	query := r.FormValue("query")
	formats := checkboxValues(formatValues, r.Form["format"])
	genders := checkboxValues(genderValues, r.Form["gender"])

	if query == "" {
		query = "SELECT * FROM innings ORDER BY runs DESC LIMIT 10;"
	}

	executeTemplate(w, "index.html", Page{
		Title: "Cricket query",
		Content: struct {
			Query           string
			LabelledResults []LabelledResult
			Formats         []Checkbox
			Genders         []Checkbox
		}{
			Query:           query,
			LabelledResults: projectQuery(formats, genders, query, 100),
			Formats:         formats,
			Genders:         genders,
		},
	})
}

func schema(w http.ResponseWriter, r *http.Request) {
	executeTemplate(w, "schema.html", Page{
		Title:   "Cricket query schema",
		Content: runQuery("SELECT name, sql FROM sqlite_schema WHERE type = 'table';", 100),
	})
}

func executeTemplate(w http.ResponseWriter, path string, page Page) {
	template.Must(
		template.
			New("").
			Funcs(template.FuncMap{
				"format": format,
			}).
			ParseFS(templatesFS, "template/_*.html", "template/"+path)).
		ExecuteTemplate(w, path, page)
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

	http.HandleFunc("/", index)
	http.HandleFunc("/schema", schema)

	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%s", port), logRequests(http.DefaultServeMux)))
}
