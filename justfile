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

check: fmt lint test build

run payload:
  go run . {{payload}}
