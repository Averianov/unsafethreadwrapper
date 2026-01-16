#!/usr/bin/make

GOCMD=$(shell which go)
GOMOD=$(shell which go) mod
GOLINT=$(shell which golint)
GODOC=$(shell which doc)
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOLIST=$(GOCMD) list
GOVET=$(GOCMD) vet
GORUN=$(GOCMD) run
CGO_ENABLED=1

help:
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@echo '    build                    Preparing www content && Build executable file.'
	@echo '    run                      Start test wrapper.'
	@echo ''
	@echo 'Targets run by default are: fmt deps vet lint build test-unit.'
	@echo ''


build:
	go mod tidy
	go build -o demo ./cmd/main.go

run:
	#go clean -cache
	#go mod tidy
	#CGO_ENABLED=1 go run ./cmd/main.go
	#./master
	go run ./cmd/main.go

test:
	go test -v ./...

build-worker:
	go build -o worker worker.go

build-logger:
	go build -o logger logger.go


# Новый раздел для автоматической сборки elf-процессов

.PHONY: all build-elves check-elves gen-headers build-master clean
# all: build-elves check-elves gen-headers build-master
all: 
	./privat/makeall.sh