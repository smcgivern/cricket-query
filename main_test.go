package main

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jmoiron/sqlx"
	"html/template"
	"testing"
	"time"

	sqlite3 "modernc.org/sqlite"
)

func init() {
	sqlite3.MustRegisterScalarFunction(
		"test_sleep_150",
		0,
		func(ctx *sqlite3.FunctionContext, args []driver.Value) (driver.Value, error) {
			time.Sleep(150 * time.Millisecond)

			return int64(1), nil
		},
	)
}

var ignoreDuration = cmpopts.IgnoreFields(Result{}, "Duration")

func makeSingleRow(val any) [][]any {
	rows := make([][]any, 1)
	rows[0] = make([]any, 1)
	rows[0][0] = val

	return rows
}

func TestMain(m *testing.M) {
	db = sqlx.MustConnect("sqlite", "testdata/innings.sqlite3")
	m.Run()
}

func TestFormat(t *testing.T) {
	cases := []struct {
		input    any
		expected string
	}{
		{"C Bannerman", "C Bannerman"},
		{"'2001", "2001"},
		{"'<html>", "&lt;html&gt;"},
		{"'2.001", "2.001"},
		{"'2001-01-01", "2001-01-01"},
		{"'p123", "p123"},
		{"p123", `<a href="https://www.espncricinfo.com/ci/content/player/123.html">p123</a>`},
		{"m123", `<a href="https://www.espncricinfo.com/ci/content/match/123.html">m123</a>`},
		{"p123m", "p123m"},
		{"p", "p"},
		{"6996", "6,996"},
		{"-6996", "-6,996"},
		{"6996.01", "6,996.01"},
		{"6996.015", "6,996.02"},
		{"99.9400000", "99.94"},
		{"-99.9400000", "-99.94"},
		{"1877-03-15 00:00:00 +0000 UTC", "15 March 1877"},
		{"1877-03-15", "15 March 1877"},
		{"1 string with 2 numbers", "1 string with 2 numbers"},
		{nil, ""},
		{6996, "6,996"},
		{6996.015, "6,996.02"},
	}

	for _, c := range cases {
		if format(c.input) != template.HTML(c.expected) {
			t.Errorf("format(%q) == %v, want %v", c.input, format(c.input), c.expected)
		}
	}
}

func TestBaseUrl(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"/help/", "/cricket-query/help/"},
		{"help", "help"},
		{"/cricket-query/help/", "/cricket-query/help/"},
	}

	for _, c := range cases {
		if baseUrl(c.input) != c.expected {
			t.Errorf("baseUrl(%q) == %v, want %v", c.input, baseUrl(c.input), c.expected)
		}
	}
}

func TestProjectQuery(t *testing.T) {
	rows := make([][]interface{}, 1)
	rows[0] = make([]interface{}, 1)
	rows[0][0] = int64(25)
	ctx := context.Background()

	cases := []struct {
		query    Query
		limit    int
		expected []LabelledResult
	}{
		{
			Query{
				Formats: checkboxValues(formatValues, []string{"odi", "t20i"}),
				Genders: checkboxValues(genderValues, []string{"men"}),
				// Using full table names means that the projected
				// tables don't apply.
				SQL: "SELECT runs FROM women_test_batting_innings WHERE runs IS NOT NULL ORDER BY runs DESC LIMIT 1;",
			},
			1,
			[]LabelledResult{
				LabelledResult{
					Header: "Men's ODI",
					Id:     "men-odi",
					Result: Result{
						Columns:  []string{"runs"},
						Rows:     rows,
						Messages: []string{},
					},
				},
				LabelledResult{
					Header: "Men's T20I",
					Id:     "men-t20i",
					Result: Result{
						Columns:  []string{"runs"},
						Rows:     rows,
						Messages: []string{},
					},
				},
			},
		},
		{
			Query{
				Formats: checkboxValues(formatValues, []string{"test"}),
				Genders: checkboxValues(genderValues, []string{"women"}),
				SQL:     "SELECT runs FROM innings WHERE runs IS NOT NULL ORDER BY runs DESC LIMIT 1;",
			},
			1,
			[]LabelledResult{
				LabelledResult{
					Header: "Women's Test",
					Id:     "women-test",
					Result: Result{
						Columns:  []string{"runs"},
						Rows:     rows,
						Messages: []string{},
					},
				},
			},
		},
	}

	for _, c := range cases {
		result := projectQuery(ctx, c.query, c.limit, 100)

		if diff := cmp.Diff(c.expected, result, ignoreDuration); diff != "" {
			t.Errorf("projectQuery(ctx, %v, %d, 100) mismatch (-expected +result):\n%s", c.query, c.limit, diff)
		}

		for _, r := range result {
			if r.Result.Duration > 100000000 || r.Result.Duration == 0 {
				t.Errorf("projectQuery(ctx, %v, %d, 100) unexpected duration: %s", c.query, c.limit, r.Result.Duration)
			}
		}
	}
}

func TestRunQuery(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		sql      string
		limit    int
		expected Result
	}{
		{
			"SELECT runs FROM women_test_batting_innings WHERE runs IS NOT NULL ORDER BY runs ASC;",
			1,
			Result{
				Columns:  []string{"runs"},
				Rows:     makeSingleRow(int64(0)),
				Messages: []string{"Too many rows returned; stopping at 1"},
			},
		},
		{
			"SELECT runs FROM women_test_batting_innings WHERE runs IS NOT NULL ORDER BY runs ASC LIMIT 1;",
			2,
			Result{
				Columns:  []string{"runs"},
				Rows:     makeSingleRow(int64(0)),
				Messages: []string{},
			},
		},
		{
			"SELECT median(mins) FROM women_test_batting_innings;",
			1,
			Result{
				Columns:  []string{"median(mins)"},
				Rows:     makeSingleRow(int64(0)),
				Messages: []string{},
			},
		},
		{
			"SELECT median(runs) FROM (SELECT runs FROM women_test_batting_innings ORDER BY runs DESC LIMIT 2);",
			1,
			Result{
				Columns:  []string{"median(runs)"},
				Rows:     makeSingleRow(float64(14.5)),
				Messages: []string{},
			},
		},
		{
			"UPDATE women_test_batting_innings SET runs = 100;",
			1,
			Result{Messages: []string{"attempt to write a readonly database (8)"}},
		},
		{
			// https://dba.stackexchange.com/a/203607
			"WITH RECURSIVE r(i) AS (VALUES(0) UNION ALL SELECT i FROM r LIMIT 10000000) SELECT i FROM r WHERE i = 1;",
			1,
			Result{Messages: []string{"interrupted (9)"}},
		},
		{
			"SELECT test_sleep_150();",
			1,
			Result{Messages: []string{"context deadline exceeded"}},
		},
	}

	for _, c := range cases {
		result := runQuery(ctx, c.sql, c.limit, 100)

		if diff := cmp.Diff(c.expected, result, ignoreDuration); diff != "" {
			t.Errorf("runQuery(ctx, %q, %d, 100) mismatch (-expected +result):\n%s", c.sql, c.limit, diff)
		}

		// Add padding for timeout tests
		if result.Duration > 200000000 || result.Duration == 0 {
			t.Errorf("runQuery(ctx, %q, %d, 100) unexpected duration: %v", c.sql, c.limit, result.Duration)
		}
	}
}

func TestAddAliases(t *testing.T) {
	cases := []struct {
		gender   string
		format   string
		sql      string
		expected string
	}{
		{
			"men",
			"t20i",
			"WITH foo AS (SELECT * FROM bar) SELECT COUNT(*) FROM foo;",
			`
WITH
innings AS (SELECT * FROM men_t20i_batting_innings),
bowling_innings AS (SELECT * FROM men_t20i_bowling_innings),
team_innings AS (SELECT * FROM men_t20i_team_innings)
, foo AS (SELECT * FROM bar) SELECT COUNT(*) FROM foo;
`,
		},
		{
			"women",
			"test",
			"SELECT COUNT(*) FROM foo;",
			`
WITH
innings AS (SELECT * FROM women_test_batting_innings),
bowling_innings AS (SELECT * FROM women_test_bowling_innings),
team_innings AS (SELECT * FROM women_test_team_innings)
SELECT COUNT(*) FROM foo;
`,
		},
	}

	for _, c := range cases {
		if addAliases(c.gender, c.format, c.sql) != c.expected {
			t.Errorf("addAliases(%q, %q, %q) == %v, want %v", c.gender, c.format, c.sql, addAliases(c.gender, c.format, c.sql), c.expected)
		}
	}
}

func TestInArray(t *testing.T) {
	cases := []struct {
		needle   string
		haystack []string
		expected bool
	}{
		{"men", []string{"men", "women"}, true},
		{"women", []string{"men", "women"}, true},
		{"test", []string{"men", "women"}, false},
		{"test", []string{"test", "odi", "t20i"}, true},
	}

	for _, c := range cases {
		if inArray(c.needle, c.haystack) != c.expected {
			t.Errorf("inArray(%q, %q) == %v, want %v", c.needle, c.haystack, inArray(c.needle, c.haystack), c.expected)
		}
	}
}

func TestCheckboxValues(t *testing.T) {
	cases := []struct {
		checkboxes []Checkbox
		checked    []string
		expected   []Checkbox
	}{
		{
			formatValues,
			[]string{"t20i"},
			[]Checkbox{
				Checkbox{"Test", "test", false},
				Checkbox{"ODI", "odi", false},
				Checkbox{"T20I", "t20i", true},
			},
		},
		{
			formatValues,
			[]string{"test", "t20i"},
			[]Checkbox{
				Checkbox{"Test", "test", true},
				Checkbox{"ODI", "odi", false},
				Checkbox{"T20I", "t20i", true},
			},
		},
		{
			formatValues,
			[]string{},
			[]Checkbox{
				Checkbox{"Test", "test", true},
				Checkbox{"ODI", "odi", true},
				Checkbox{"T20I", "t20i", true},
			},
		},
	}

	for _, c := range cases {
		result := checkboxValues(c.checkboxes, c.checked)
		if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", c.expected) {
			t.Errorf("checkboxValues(%v, %v) == %v, want %v", c.checkboxes, c.checked, result, c.expected)
		}
	}
}
