#include <stddef.h>
#include "elf_payloads.h"
#include "logger_elf.h"
#include "worker1_elf.h"
#include "worker2_elf.h"
ElfPayload elf_payloads[4];
void init_elf_payloads() {
    int idx = 0;
    elf_payloads[idx].name = "logger";
    elf_payloads[idx].data = cmd_executable_logger;
    elf_payloads[idx].len = cmd_executable_logger_len;
    idx++;
    elf_payloads[idx].name = "worker1";
    elf_payloads[idx].data = cmd_executable_worker1;
    elf_payloads[idx].len = cmd_executable_worker1_len;
    idx++;
    elf_payloads[idx].name = "worker2";
    elf_payloads[idx].data = cmd_executable_worker2;
    elf_payloads[idx].len = cmd_executable_worker2_len;
    idx++;
    elf_payloads[idx].name = NULL;
    elf_payloads[idx].data = NULL;
    elf_payloads[idx].len = 0;
}
