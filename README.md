# Cricket Query

A web interface to write SQL against Statsguru. Based on data from
[obrasier/cricketstats](https://github.com/obrasier/cricketstats).

## Development

1. Clone the repo.
2. `direnv allow .` if using direnv and Nix.
3. `make` to run tests.
4. `make run` to run on
   [localhost:8080/cricket-query](http://localhost:8080/cricket-query).
   This requires CSVs from cricketstats in `data/`.

### Saved queries

These are in [saved-queries](saved-queries) with the `.txt` extension.
The format is very barebones, being line-delimited:

1. Title.
2. Description (one paragraph, no HTML).
3. Formats (blank line for all; otherwise `"test", "odi", "t20i"`).
4. Genders (blank line for all; otherwise `"men", "women"`).
5. Query (all remaining lines; can span multiple lines).

After updating this, run `make` or `make run` (which will automatically
invoke `make saved_queries.go`) to update `saved_queries.go`. Do not
update this file manually.
