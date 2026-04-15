set shell := ["bash", "-euo", "pipefail", "-c"]

golangci_lint := "go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.2"

fmt:
	gofmt -w $(find . -name '*.go' -not -path './.git/*')

fmt_check:
	tmpfile="$$(mktemp)"; \
	trap 'rm -f "$$tmpfile"' EXIT; \
	gofmt -d $$(find . -name '*.go' -not -path './.git/*') > "$$tmpfile"; \
	test ! -s "$$tmpfile"

lint:
	{{golangci_lint}} run ./...

shell_test:
	zsh contrib/autosuggest/test_autosuggest.zsh
	zsh contrib/atc/test_atc.zsh

test:
	go test ./...
	zsh contrib/autosuggest/test_autosuggest.zsh
	zsh contrib/atc/test_atc.zsh

race:
	go test -race ./...

ci: fmt_check lint test race
