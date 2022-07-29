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
