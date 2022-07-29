.PHONY: test
test: fmt
	@go test .

.PHONY: release
release: release/data/innings.sqlite3 release/cricket-query

.PHONY: clean
clean:
	rm -f release/data/innings.sqlite3
	rm release/cricket-query

.PHONY: run
run: fmt
	@go run .

.PHONY: fmt
fmt:
	@go fmt

data/innings.sqlite3: data/*.csv scripts/create-db
	scripts/create-db data
	chmod -w data/innings.sqlite3

release/data/innings.sqlite3: data/innings.sqlite3
	mkdir -p release/data
	cp data/innings.sqlite3 release/data

release/cricket-query: main.go
	go build -o release/cricket-query

-include *.mk
