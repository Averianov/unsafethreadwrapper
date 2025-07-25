typedef struct {
    const char* name;
    const unsigned char* data;
    unsigned int len;
} ElfPayload;

extern ElfPayload elf_payloads[4];
void init_elf_payloads();
