# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	@go fmt ./...
	@go mod tidy -v

## vuln: scan the code and its dependencies for known vulnerabilities
.PHONY: vuln
vuln:
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## lint: run golangci-lint (the same linter CI uses)
.PHONY: lint
lint:
	@go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run

## audit: run quality control checks
.PHONY: audit
audit:
	@go mod verify
	@go vet ./...
	@go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	@go test -race -buildvcs -vet=off ./...

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## test: run all tests
.PHONY: test
test:
	@go clean -testcache
	@go test -count=1 -race -timeout 60s ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	@go clean -testcache
	@go test -race ./... --coverprofile /tmp/coverage.out >> /dev/null
	@go tool cover -func /tmp/coverage.out
	@go tool cover -html=/tmp/coverage.out -o /tmp/coverage.html
