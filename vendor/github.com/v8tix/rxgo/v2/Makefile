# ====================================================================================
# HELPERS
# ====================================================================================

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

.PHONY: no-dirty
no-dirty:
	@test -z "$$(git status --porcelain)"

# ====================================================================================
# QUALITY CONTROL
# ====================================================================================

## audit: run all quality control checks (modules, format, vet, staticcheck, vulnerabilities, tests)
.PHONY: audit
audit: test
	go mod tidy -diff
	go mod verify
	test -z "$$(gofmt -l $$(git ls-files '*.go'))"
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## test: run all tests, with the race detector and the goroutine leak checks
.PHONY: test
test:
	go clean -testcache
	go test -race -timeout 60s ./... --tags=all
	go test -timeout 60s -run TestLeak

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	go test -race -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out

## vuln: scan the code and its dependencies for known vulnerabilities
.PHONY: vuln
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## lint: run golangci-lint (the same linter CI uses)
.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run

## upgradeable: list direct dependencies that have upgrades available
.PHONY: upgradeable
upgradeable:
	go run github.com/oligot/go-mod-upgrade@latest

# ====================================================================================
# DEVELOPMENT
# ====================================================================================

## tidy: tidy modfiles, modernize and format the .go files
.PHONY: tidy
tidy:
	go mod tidy -v
	go fix ./...
	go fmt ./...

# ====================================================================================
# OPERATIONS
# ====================================================================================

## push: audit, then push the changes to the remote Git repository
.PHONY: push
push: confirm audit no-dirty
	git push
