.PHONY: test
test: fmt testdata/innings.sqlite3
	@go test .

.PHONY: release
release: release/data/innings.sqlite3 release/cricket-query

.PHONY: clean
clean:
	rm -f release/data/innings.sqlite3
	rm release/cricket-query

.PHONY: run
run: fmt data/innings.sqlite3
	@go run .

.PHONY: fmt
fmt:
	@go fmt

data/innings.sqlite3: data/*.csv scripts/create-db
	scripts/create-db data

release/data/innings.sqlite3: data/innings.sqlite3
	mkdir -p release/data
	cp data/innings.sqlite3 release/data

release/cricket-query: main.go
	go build -o release/cricket-query

testdata/innings.sqlite3: scripts/create-db
	scripts/create-db testdata

-include *.mk
