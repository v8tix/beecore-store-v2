include .envrc

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

## update-deps: update all deps
.PHONY: update/deps
update/deps:
	@go get -u all

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	@go fmt ./...
	@go mod tidy -v

## lint: run golangci-lint
.PHONY: lint
lint:
	@golangci-lint run

## audit: run quality control checks
.PHONY: audit
audit:
	@go clean -testcache
	@go mod verify
	@go vet ./...
	@go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	@golangci-lint run
	@go test -race -buildvcs -vet=off ./...

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## install/golangci-lint: install latest golangci-lint
.PHONY: install/golangci-lint
install/golangci-lint:
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@echo "golangci-lint v2 installed"

## update/tools: update tools
.PHONY: update/tools
update/tools:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@$(MAKE) install/golangci-lint

## test: run all tests
.PHONY: test
test:
	@go clean -testcache
	@go test -v -race -buildvcs ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	@go clean -testcache
	@go test -race ./... --coverprofile ${COVER_FILE_NAME} >> /dev/null
	@go tool cover -func ${COVER_FILE_NAME}
	@go tool cover -html=${COVER_FILE_NAME} -o ${COVER_FILE_NAME}.html

## vendor: tidy and vendor dependencies
.PHONY: vendor
vendor:
	@echo 'Tidying and verifying module dependencies...'
	@go mod tidy
	@go mod verify
	@echo 'Vendoring dependencies...'
	@go mod vendor
