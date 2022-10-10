package main

import (
	"context"
	"testing"
)

func TestSavedQueries(t *testing.T) {
	ctx := context.Background()

	for key, query := range savedQueries {
		for _, lr := range projectQuery(ctx, query, rowsLimit, defaultTimeout) {
			if len(lr.Result.Messages) > 0 {
				t.Errorf("Saved query %s (%s) had messages: %v", key, lr.Id, lr.Result.Messages)
			}
		}
	}
}
