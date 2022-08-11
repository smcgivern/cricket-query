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
	"strings"
	"time"
)

var (
	db *sqlx.DB
	//go:embed all:template
	templatesFS embed.FS
)

type Query struct {
	Formats     []Checkbox
	Genders     []Checkbox
	Query       string
	Subtitle    string
	Description string
}

type Result struct {
	Columns  []string
	Rows     [][]any
	Messages []string
	Duration time.Duration
}

type LabelledResult struct {
	Header string
	Id     string
	Result Result
}

type Page struct {
	Title   string
	Query   Query
	Content any
}

type Checkbox struct {
	Label   string
	Value   string
	Checked bool
}

var rowsLimit = 100

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
	} else if strings.HasPrefix(text, "'") {
		return strings.TrimPrefix(text, "'")
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

func baseUrl(url string) string {
	rootLeadingSlash := "/cricket-query"
	rootDoubleSlash := "/cricket-query/"

	if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, rootDoubleSlash) {
		return fmt.Sprintf("%s%s", rootLeadingSlash, url)
	} else {
		return url
	}
}

func projectQuery(query Query, limit int) (out []LabelledResult) {
	for _, format := range query.Formats {
		for _, gender := range query.Genders {
			if format.Checked && gender.Checked {
				out = append(out, LabelledResult{
					fmt.Sprintf("%s's %s", gender.Label, format.Label),
					fmt.Sprintf("%s-%s", gender.Value, format.Value),
					runQuery(addAliases(gender.Value, format.Value, query.Query), limit),
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
	start := time.Now()

	results, err := db.Queryx(query)
	elapsed := time.Now().Sub(start)
	if err != nil {
		return Result{Messages: []string{err.Error()}, Duration: elapsed}
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
		Duration: elapsed,
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
	var query Query
	var ok bool

	r.ParseForm()

	queryId := r.FormValue("queryId")

	if query, ok = savedQueries[queryId]; !ok {
		query = Query{
			Query:   r.FormValue("query"),
			Formats: checkboxValues(formatValues, r.Form["format"]),
			Genders: checkboxValues(genderValues, r.Form["gender"]),
		}
	}

	if query.Query == "" {
		query.Query = "SELECT * FROM innings ORDER BY runs DESC LIMIT 10;"
	}

	executeTemplate(w, "index.html", Page{
		Title: "Cricket query",
		Query: query,
		Content: struct {
			LabelledResults []LabelledResult
		}{
			projectQuery(query, rowsLimit),
		},
	})
}

func help(w http.ResponseWriter, r *http.Request) {
	executeTemplate(w, "help.html", Page{
		Title: "Cricket query help",
		Content: struct {
			SavedQueries map[string]Query
			Latest       Result
		}{
			savedQueries,
			runQuery(`
SELECT gender, format, team, opposition, ground, start_date FROM (
  SELECT * FROM (SELECT 1 AS sort, 'men' AS gender, 'test' AS format, team, opposition, ground, start_date FROM men_test_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 2 AS sort, 'women' AS gender, 'test' AS format, team, opposition, ground, start_date FROM women_test_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 3 AS sort, 'men' AS gender, 'odi' AS format, team, opposition, ground, start_date FROM men_odi_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 4 AS sort, 'women' AS gender, 'odi' AS format, team, opposition, ground, start_date FROM women_odi_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 5 AS sort, 'men' AS gender, 't20i' AS format, team, opposition, ground, start_date FROM men_t20i_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 6 AS sort, 'women' AS gender, 't20i' AS format, team, opposition, ground, start_date FROM women_t20i_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
) ORDER BY sort ASC;`,
				10,
			),
		},
	})
}

func executeTemplate(w http.ResponseWriter, path string, page Page) {
	template.Must(
		template.
			New("").
			Funcs(template.FuncMap{
				"format":  format,
				"baseUrl": baseUrl,
				"formatDuration": func(duration time.Duration) string {
					return fmt.Sprintf("%s", duration.Round(time.Millisecond))
				},
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

	http.HandleFunc(baseUrl("/"), index)
	http.HandleFunc(baseUrl("/help/"), help)

	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%s", port), logRequests(http.DefaultServeMux)))
}
