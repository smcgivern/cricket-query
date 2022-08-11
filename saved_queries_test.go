package main

import (
	"testing"
)

func TestSavedQueries(t *testing.T) {
	for key, query := range savedQueries {
		for _, lr := range projectQuery(query, rowsLimit) {
			if len(lr.Result.Messages) > 0 {
				t.Errorf("Saved query %s (%s) had messages: %v", key, lr.Id, lr.Result.Messages)
			}
		}
	}
}
