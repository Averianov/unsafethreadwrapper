#!/usr/bin/env bash
# Скрипт генерации build/elf_payloads.h и build/elf_payloads.c
# Usage: ./scripts/gen_elf_payloads.sh processes.json cmd/executable build

set -e

PROCESS_JSON="$1"
EXECDIR="$2"
OUTDIR="$3"

if [[ -z "$PROCESS_JSON" || -z "$EXECDIR" || -z "$OUTDIR" ]]; then
  echo "Usage: $0 processes.json cmd/executable build"
  exit 1
fi

mkdir -p "$OUTDIR"

# Получаем список процессов
PROCS=($(jq -r '.processes[].name' "$PROCESS_JSON"))
NPROCS=${#PROCS[@]}

# Генерируем .h для каждого elf
for proc in "${PROCS[@]}"; do
  xxd -i "$EXECDIR/$proc" > "$OUTDIR/${proc}_elf.h"
  # Делаем массивы extern const, чтобы их можно было использовать глобально
  sed -i 's/static unsigned char/extern const unsigned char/g' "$OUTDIR/${proc}_elf.h"
  sed -i 's/static unsigned int/extern const unsigned int/g' "$OUTDIR/${proc}_elf.h"
done

# Генерируем elf_payloads.h
cat > "$OUTDIR/elf_payloads.h" <<EOF
typedef struct {
    const char* name;
    const unsigned char* data;
    unsigned int len;
} ElfPayload;

extern ElfPayload elf_payloads[$((NPROCS+1))];
void init_elf_payloads();
EOF

# Генерируем elf_payloads.c
cat > "$OUTDIR/elf_payloads.c" <<EOF
#include <stddef.h>
#include "elf_payloads.h"
EOF
for proc in "${PROCS[@]}"; do
  echo "#include \"${proc}_elf.h\"" >> "$OUTDIR/elf_payloads.c"
done

tmpfile=$(mktemp)
echo "ElfPayload elf_payloads[$((NPROCS+1))];" > "$tmpfile"
echo "void init_elf_payloads() {" >> "$tmpfile"
echo "    int idx = 0;" >> "$tmpfile"
for proc in "${PROCS[@]}"; do
  echo "    elf_payloads[idx].name = \"$proc\";" >> "$tmpfile"
  echo "    elf_payloads[idx].data = cmd_executable_${proc};" >> "$tmpfile"
  echo "    elf_payloads[idx].len = cmd_executable_${proc}_len;" >> "$tmpfile"
  echo "    idx++;" >> "$tmpfile"
done
echo "    elf_payloads[idx].name = NULL;" >> "$tmpfile"
echo "    elf_payloads[idx].data = NULL;" >> "$tmpfile"
echo "    elf_payloads[idx].len = 0;" >> "$tmpfile"
echo "}" >> "$tmpfile"
cat "$tmpfile" >> "$OUTDIR/elf_payloads.c"
rm "$tmpfile" 