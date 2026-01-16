#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <string.h>
#include "elf_payload.h"

int main() {
    // Используем встроенный ELF как массив байт
    int memfd = memfd_create("payload", 0);
    if (memfd < 0) { perror("memfd_create"); return 1; }
    if (write(memfd, internal_elf_payload, internal_elf_payload_len) != internal_elf_payload_len) { perror("write"); return 1; }
    lseek(memfd, 0, SEEK_SET);

    // Запускаем процесс из памяти
    char* const argv[] = {"payload", NULL};
    char* const envp[] = {NULL};
    printf("[exec_from_mem] Запуск ELF из памяти через fexecve...\n");
    if (fexecve(memfd, argv, envp) < 0) {
        perror("fexecve");
        return 1;
    }
    return 0;
} 