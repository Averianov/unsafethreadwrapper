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
	./master

test:
	go test -v ./...

build-worker:
	go build -o worker worker.go

build-logger:
	go build -o logger logger.go

worker_elf.h: worker
	xxd -i worker > internal/worker_elf.h

logger_elf.h: logger
	xxd -i logger > internal/logger_elf.h

# Новый раздел для автоматической сборки elf-процессов

PROCESS_JSON=processes.json
EXECDIR=cmd/executable
SRCDIR=cmd

# Получить список процессов из processes.json (jq должен быть установлен)
PROCESSES=$(shell jq -r '.processes[].name' $(PROCESS_JSON))

TMPDIR=build

.PHONY: all build-elves check-elves gen-headers build-master clean

all: build-elves check-elves gen-headers build-master

build-elves:
	@mkdir -p $(EXECDIR)
	@for proc in $(PROCESSES); do \
	  if [ -f $(SRCDIR)/$$proc/main.go ]; then \
	    echo "Building $$proc..."; \
	    go build -o $(EXECDIR)/$$proc $(SRCDIR)/$$proc/main.go; \
	  else \
	    echo "Source for $$proc not found!"; \
	    exit 1; \
	  fi; \
	done

check-elves:
	@for proc in $(PROCESSES); do \
	  if [ ! -f $(EXECDIR)/$$proc ]; then \
	    echo "Executable for $$proc not found!"; \
	    exit 1; \
	  fi; \
	done

# Генерация .h файлов для мастера во временную директорию
gen-headers:
	@mkdir -p $(TMPDIR)
	./scripts/gen_elf_payloads.sh $(PROCESS_JSON) $(EXECDIR) $(TMPDIR)

build-elf-payloads:
	gcc -c $(TMPDIR)/elf_payloads.c -o $(TMPDIR)/elf_payloads.o

build-master: build-elf-payloads
	gcc -o master internal/master.c $(TMPDIR)/elf_payloads.o -lcjson -lrt

clean:
	rm -rf $(EXECDIR)/* $(TMPDIR) master
