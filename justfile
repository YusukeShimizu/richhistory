set shell := ["bash", "-euo", "pipefail", "-c"]

golangci_lint := "go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.2"

fmt:
	gofmt -w $(find . -name '*.go' -not -path './.git/*')

fmt_check:
	tmpfile="$$(mktemp)"; \
	trap 'rm -f "$$tmpfile"' EXIT; \
	find . -name '*.go' -not -path './.git/*' -print0 | xargs -0 gofmt -d > "$$tmpfile"; \
	test ! -s "$$tmpfile"

lint:
	{{golangci_lint}} run ./...

shell_test:
	zsh contrib/atc/test_atc.zsh
	zsh contrib/suggest/test_suggest.zsh

test:
	go test ./...
	zsh contrib/atc/test_atc.zsh
	zsh contrib/suggest/test_suggest.zsh

race:
	go test -race ./...

ci: fmt_check lint test race
