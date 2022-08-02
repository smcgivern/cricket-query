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
