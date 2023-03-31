.PHONY: test
test: fmt testdata/innings.sqlite3 saved_queries.go
	@go test .

.PHONY: release
release: release/data/innings.sqlite3 release/cricket-query

.PHONY: clean
clean: clean-db clean-binary

.PHONY: clean-db
	rm -f release/data/innings.sqlite3

.PHONY: clean-binary
clean-binary:
	rm release/cricket-query

.PHONY: run
run: fmt data/innings.sqlite3 saved_queries.go
	@go run .

.PHONY: fmt
fmt:
	@go fmt

saved_queries.go: saved-queries/*.txt scripts/create-saved-queries
	scripts/create-saved-queries

data/innings.sqlite3: data/*.csv scripts/create-db
	scripts/create-db data

release/data/innings.sqlite3: data/innings.sqlite3
	mkdir -p release/data
	cp data/innings.sqlite3 release/data

release/cricket-query: *.go
	go build -o release/cricket-query

testdata/innings.sqlite3: scripts/create-db
	scripts/create-db testdata

-include *.mk
