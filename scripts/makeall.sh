#!/usr/bin/env bash
# Новый раздел для автоматической сборки elf-процессов

set -e # остановиться при любой ошибке
set -x # режим вывода работы команд

PROCESS_JSON=processes.json
SRCDIR=cmd
EXECDIR=$SRCDIR/executable
TMPDIR=build

# Получить список процессов из processes.json (jq должен быть установлен)
PROCESSES=$(jq -r '.processes[].name' "$PROCESS_JSON")


# build-elves:
	mkdir -p $EXECDIR
	for proc in $PROCESSES; do 
	  if [ -f "$SRCDIR/$proc/main.go" ]; then 
	    echo "Building $proc..."
	    go build -o "$EXECDIR/$proc" "$SRCDIR/$proc/main.go"
	  else 
	    echo "Source for $proc not found!"
	    exit 1
	  fi
	done

# check-elves:
	for proc in $PROCESSES; do
	  if [ ! -f "$EXECDIR/$proc" ]; then
	    echo "Executable for $proc not found!"
	    exit 1
	  fi
	done

# gen-headers (Генерация .h файлов для мастера во временную директорию):
	mkdir -p "$TMPDIR"
	./scripts/gen_elf_payloads.sh "$PROCESS_JSON" "$EXECDIR" "$TMPDIR"

# build-elf-payloads:
	gcc -c "$TMPDIR/elf_payloads.c" -o "$TMPDIR/elf_payloads.o"

# build-master:
	gcc -o master internal/master.c "$TMPDIR/elf_payloads.o" -lcjson -lrt
