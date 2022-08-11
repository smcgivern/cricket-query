package main

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"testing"
)

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
		{"6996", "6,996"},
		{"6996.01", "6,996.01"},
		{"6996.015", "6,996.02"},
		{"99.9400000", "99.94"},
		{"1877-03-15 00:00:00 +0000 UTC", "15 March 1877"},
		{"1877-03-15", "15 March 1877"},
		{"1 string with 2 numbers", "1 string with 2 numbers"},
		{nil, ""},
		{6996, "6,996"},
		{6996.015, "6,996.02"},
	}

	for _, c := range cases {
		if format(c.input) != c.expected {
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
	rows[0][0] = "25"

	cases := []struct {
		formats  []Checkbox
		genders  []Checkbox
		query    string
		limit    int
		expected []LabelledResult
	}{
		{
			checkboxValues(formatValues, []string{"odi", "t20i"}),
			checkboxValues(genderValues, []string{"men"}),
			// Using full table names means that the projected
			// tables don't apply.
			"SELECT runs FROM women_test_batting_innings WHERE runs IS NOT NULL ORDER BY runs DESC LIMIT 1;",
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
			checkboxValues(formatValues, []string{"test"}),
			checkboxValues(genderValues, []string{"women"}),
			"SELECT runs FROM innings WHERE runs IS NOT NULL ORDER BY runs DESC LIMIT 1;",
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
		result := projectQuery(c.formats, c.genders, c.query, c.limit)
		if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", c.expected) {
			t.Errorf("projectQuery(%v, %v, %q, %d) == %v, want %v", c.formats, c.genders, c.query, c.limit, result, c.expected)
		}
	}
}

func TestRunQuery(t *testing.T) {
	rows := make([][]interface{}, 1)
	rows[0] = make([]interface{}, 1)
	rows[0][0] = "0"

	cases := []struct {
		query    string
		limit    int
		expected Result
	}{
		{
			"SELECT runs FROM women_test_batting_innings WHERE runs IS NOT NULL ORDER BY runs ASC;",
			1,
			Result{
				Columns:  []string{"runs"},
				Rows:     rows,
				Messages: []string{"Too many rows returned; stopping at 1"},
			},
		},
		{
			"SELECT runs FROM women_test_batting_innings WHERE runs IS NOT NULL ORDER BY runs ASC LIMIT 1;",
			2,
			Result{
				Columns: []string{"runs"},
				Rows:    rows,
			},
		},
		{
			"UPDATE women_test_batting_innings SET runs = 100;",
			1,
			Result{Messages: []string{"attempt to write a readonly database (8)"}},
		},
	}

	for _, c := range cases {
		result := runQuery(c.query, c.limit)
		if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", c.expected) {
			t.Errorf("runQuery(%q, %d) == %v, want %v", c.query, c.limit, result, c.expected)
		}
	}
}

func TestAddAliases(t *testing.T) {
	cases := []struct {
		gender   string
		format   string
		query    string
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
		if addAliases(c.gender, c.format, c.query) != c.expected {
			t.Errorf("addAliases(%q, %q, %q) == %v, want %v", c.gender, c.format, c.query, addAliases(c.gender, c.format, c.query), c.expected)
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
