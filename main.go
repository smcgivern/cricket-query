package main

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func main() {
	db := sqlx.MustConnect("sqlite", ":memory:")
	defer db.Close()

	rows, err := db.Queryx("SELECT 'a' AS first, 'b' AS second, 1 AS third")
	if err != nil {
		fmt.Println(err)
	}
	columns, err := rows.Columns()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(columns)
	for rows.Next() {
		// cols is an []interface{} of all of the column results
		cols, err := rows.SliceScan()
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(cols)
	}
}
