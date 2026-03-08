set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default:
  @just --list

fmt:
  gofmt -w ./*.go

lint:
  go vet ./...

build:
  go build .

test:
  go test ./...

coverage:
  @pkgs="$(go list ./... | rg -v '/chromeos_update_engine$$')"; \
    go test $pkgs -coverprofile=coverage-nopb.out; \
    go tool cover -func=coverage-nopb.out

check: fmt lint test build

run payload:
  go run . {{payload}}
