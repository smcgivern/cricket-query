package main

import (
	"context"
	"database/sql/driver"
	"embed"
	"fmt"
	"github.com/jmoiron/sqlx"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"html/template"
	"log"
	sqlite3 "modernc.org/sqlite"
	"net/http"
	"os"
	"regexp"
	"sort"
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
	SQL         string
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
var defaultTimeout = 5000

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

var matchLink = regexp.MustCompile(`\A[mp]\d+\z`)
var matchDate = regexp.MustCompile(`\A\d{4}-\d{2}-\d{2}( 00:00:00 \+0000 UTC)?\z`)
var matchInteger = regexp.MustCompile(`\A-?\d+\z`)
var matchFloat = regexp.MustCompile(`\A-?\d+\.\d+\z`)

var playerPrefix = "https://www.espncricinfo.com/ci/content/player/"
var matchPrefix = "https://www.espncricinfo.com/ci/content/match/"

type medianFunction struct {
	vals []float64
}

func (f *medianFunction) Step(ctx *sqlite3.FunctionContext, args []driver.Value) error {
	switch resTyped := args[0].(type) {
	case int64:
		f.vals = append(f.vals, float64(resTyped))
	case float64:
		f.vals = append(f.vals, resTyped)
	case nil:
	default:
		return fmt.Errorf("value is not a number: %T", resTyped)
	}
	return nil
}

func (f *medianFunction) WindowInverse(ctx *sqlite3.FunctionContext, args []driver.Value) error {
	first, rest := f.vals[0], f.vals[1:]

	switch resTyped := args[0].(type) {
	case int64:
		if first == float64(resTyped) {
			f.vals = rest
		}
	case float64:
		if first == resTyped {
			f.vals = rest
		}
	case nil:
	default:
		return fmt.Errorf("value is not a number: %T", resTyped)
	}
	return nil
}

func (f *medianFunction) WindowValue(ctx *sqlite3.FunctionContext) (driver.Value, error) {
	l := len(f.vals)

	sort.Float64s(f.vals)

	if l == 0 {
		return int64(0), nil
	} else if l%2 == 0 {
		return (f.vals[l/2-1] + f.vals[l/2]) / 2, nil
	} else {
		return f.vals[l/2], nil
	}
}

func (f *medianFunction) Final(ctx *sqlite3.FunctionContext) {}

func escape(s string) template.HTML {
	return template.HTML(template.HTMLEscapeString(s))
}

func format(value any) template.HTML {
	text := fmt.Sprint(value)
	bytes := []byte(text)

	if value == nil {
		return ""
	} else if strings.HasPrefix(text, "'") {
		return escape(strings.TrimPrefix(text, "'"))
	} else if matchLink.Match(bytes) {
		if strings.HasPrefix(text, "p") {
			return template.HTML(fmt.Sprintf(`<a href="%s%s.html">%s</a>`, playerPrefix, strings.TrimPrefix(text, "p"), text))
		} else if strings.HasPrefix(text, "m") {
			return template.HTML(fmt.Sprintf(`<a href="%s%s.html">%s</a>`, matchPrefix, strings.TrimPrefix(text, "m"), text))
		} else {
			return escape(text)
		}
	} else if matchDate.Match(bytes) {
		t, err := time.Parse("2006-01-02 15:04:05 +0000 UTC", text)

		if err != nil {
			t, err = time.Parse("2006-01-02", text)

			if err != nil {
				return escape(text)
			}
		}

		return escape(t.Format("2 January 2006"))

	} else if matchInteger.Match(bytes) {
		int, err := strconv.Atoi(text)

		if err != nil {
			return escape(text)
		}

		printer := message.NewPrinter(language.English)

		return escape(printer.Sprintf("%d", int))
	} else if matchFloat.Match(bytes) {
		float, err := strconv.ParseFloat(text, 64)

		if err != nil {
			return escape(text)
		}

		printer := message.NewPrinter(language.English)

		return escape(printer.Sprintf("%.2f", float))
	}

	return escape(text)
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

func projectQuery(ctx context.Context, query Query, limit int, timeout int) (out []LabelledResult) {
	for _, format := range query.Formats {
		for _, gender := range query.Genders {
			if format.Checked && gender.Checked {
				out = append(out, LabelledResult{
					fmt.Sprintf("%s's %s", gender.Label, format.Label),
					fmt.Sprintf("%s-%s", gender.Value, format.Value),
					runQuery(ctx, addAliases(gender.Value, format.Value, query.SQL), limit, timeout),
				})
			}
		}
	}

	return
}

func runQuery(ctx context.Context, sql string, limit int, timeout int) Result {
	messages := make([]string, 0)
	rows := make([][]any, 0)
	i := 1
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	results, err := db.QueryxContext(ctx, sql)
	elapsed := time.Now().Sub(start)

	if err == nil {
		err = ctx.Err()
	}

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

func addAliases(gender string, format string, sql string) string {
	if startsWithWith.Match([]byte(sql)) {
		sql = startsWithWith.ReplaceAllString(sql, ",")
	}

	return fmt.Sprintf(`
WITH
innings AS (SELECT * FROM %[1]s_%[2]s_batting_innings),
bowling_innings AS (SELECT * FROM %[1]s_%[2]s_bowling_innings),
team_innings AS (SELECT * FROM %[1]s_%[2]s_team_innings)
%[3]s
`, gender, format, sql)
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

	savedQuery := r.FormValue("query")

	if query, ok = savedQueries[savedQuery]; !ok {
		query = Query{
			SQL:     r.FormValue("sql"),
			Formats: checkboxValues(formatValues, r.Form["format"]),
			Genders: checkboxValues(genderValues, r.Form["gender"]),
		}
	}

	if query.SQL == "" {
		query.SQL = "SELECT * FROM innings ORDER BY runs DESC LIMIT 10;"
	}

	executeTemplate(w, "index.html", Page{
		Title: "Cricket query",
		Query: query,
		Content: struct {
			LabelledResults []LabelledResult
		}{
			projectQuery(r.Context(), query, rowsLimit, defaultTimeout),
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
			runQuery(
				r.Context(),
				`
SELECT gender, format, team, opposition, ground, start_date, match_id FROM (
  SELECT * FROM (SELECT 1 AS sort, 'men' AS gender, 'test' AS format, team, opposition, ground, start_date, match_id FROM men_test_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 2 AS sort, 'women' AS gender, 'test' AS format, team, opposition, ground, start_date, match_id FROM women_test_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 3 AS sort, 'men' AS gender, 'odi' AS format, team, opposition, ground, start_date, match_id FROM men_odi_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 4 AS sort, 'women' AS gender, 'odi' AS format, team, opposition, ground, start_date, match_id FROM women_odi_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 5 AS sort, 'men' AS gender, 't20i' AS format, team, opposition, ground, start_date, match_id FROM men_t20i_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
  UNION
  SELECT * FROM (SELECT 6 AS sort, 'women' AS gender, 't20i' AS format, team, opposition, ground, start_date, match_id FROM women_t20i_team_innings ORDER BY start_date DESC, i DESC LIMIT 1)
) ORDER BY sort ASC;`,
				10,
				defaultTimeout,
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

func init() {
	sqlite3.MustRegisterFunction("median", &sqlite3.FunctionImpl{
		NArgs:         1,
		Deterministic: true,
		MakeAggregate: func(ctx sqlite3.FunctionContext) (sqlite3.AggregateFunction, error) {
			return &medianFunction{}, nil
		},
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
