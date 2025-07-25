#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <signal.h>
#include <string.h>
#include <stdbool.h>
#include <errno.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <dirent.h>
#include <stdint.h>
#include <limits.h>
#include <ctype.h>
#include <cjson/cJSON.h>
#include <sys/un.h>
#include <sys/socket.h>
#include <sys/poll.h>
#include <time.h>

// --- ELF payloads ---
#include "../build/elf_payloads.h"

// --- Task struct ---
typedef struct Task {
    char name[64];
    bool must_start;
    bool st_in_progress;
    bool st_launched;
    char **required; // массив имён зависимостей
    int required_count;
    pid_t pid;
    int wait_counter;  // Добавляем новое поле для отслеживания времени ожидания
} Task;

#define MAX_TASKS 16
Task tasks[MAX_TASKS];
int task_count = 0;

#define MAX_SOCKETS 16
struct {
    char name[64];
    char path[256];
    int fd;
} sockets[MAX_SOCKETS];
int socket_count = 0;

#define MAX_CLIENTS 32

// --- Объявления функций ---
void set_nonblocking(int fd);
void command_loop();
void handle_master_command(const char* cmd, int to_stdout, int client_fd);

void set_nonblocking(int fd) {
    int flags = fcntl(fd, F_GETFL, 0);
    if (flags == -1) flags = 0;
    fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

void create_sockets_from_json(cJSON* sockets_array) {
    printf("[MASTER] create_sockets_from_json called\n");
    if (sockets_array && cJSON_IsArray(sockets_array)) {
        printf("[MASTER] sockets_array is valid array\n");
        int sockn = cJSON_GetArraySize(sockets_array);
        printf("[MASTER] sockets array size: %d\n", sockn);
        for (int j = 0; j < sockn && j < MAX_SOCKETS; ++j) {
            printf("[MASTER] Processing socket %d\n", j);
            cJSON* jitem = cJSON_GetArrayItem(sockets_array, j);
            printf("[MASTER] Got socket item\n");
            cJSON* jname = cJSON_GetObjectItem(jitem, "name");
            cJSON* jpath = cJSON_GetObjectItem(jitem, "path");
            printf("[MASTER] Got name and path objects\n");
            if (jname && jname->valuestring) {
                strncpy(sockets[j].name, jname->valuestring, 63);
                printf("[MASTER] Socket name copied: %s\n", sockets[j].name);
            }
            if (jpath && jpath->valuestring) {
                strncpy(sockets[j].path, jpath->valuestring, 255);
                printf("[MASTER] Socket path: %s\n", sockets[j].path);
            } else {
                snprintf(sockets[j].path, sizeof(sockets[j].path), "/tmp/%s.sock", sockets[j].name);
                printf("[MASTER] Generated socket path: %s\n", sockets[j].path);
            }
            unlink(sockets[j].path);
            sockets[j].fd = socket(AF_UNIX, SOCK_STREAM, 0);
            if (sockets[j].fd < 0) { perror("socket"); continue; }
            struct sockaddr_un addr = {0};
            addr.sun_family = AF_UNIX;
            strncpy(addr.sun_path, sockets[j].path, sizeof(addr.sun_path)-1);
            if (bind(sockets[j].fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) { perror("bind"); close(sockets[j].fd); sockets[j].fd = -1; continue; }
            listen(sockets[j].fd, 8);
            printf("[MASTER] Created socket: %s (fd=%d)\n", sockets[j].name, sockets[j].fd);
            socket_count++;
        }
    } else {
        printf("[MASTER] sockets_array is NULL or not an array\n");
    }
    printf("[MASTER] create_sockets_from_json completed\n");
}

// --- Заготовка для парсинга processes.json ---
void load_tasks_from_json(const char* filename) {
    printf("[MASTER] Loading tasks from %s\n", filename);
    FILE* f = fopen(filename, "r");
    if (!f) { perror("fopen json"); exit(1); }
    printf("[MASTER] File opened successfully\n");
    fseek(f, 0, SEEK_END);
    long len = ftell(f);
    fseek(f, 0, SEEK_SET);
    char* data = malloc(len+1);
    fread(data, 1, len, f);
    data[len] = 0;
    fclose(f);
    printf("[MASTER] File read, length: %ld\n", len);

    cJSON* root = cJSON_Parse(data);
    if (!root) { fprintf(stderr, "JSON parse error\n"); exit(1); }
    printf("[MASTER] JSON parsed successfully\n");
    cJSON* sockets_array = cJSON_GetObjectItem(root, "sockets");
    printf("[MASTER] Getting sockets array\n");
    create_sockets_from_json(sockets_array);  // Вызов функции для создания сокетов
    printf("[MASTER] Sockets created\n");
    cJSON* arr = cJSON_GetObjectItem(root, "processes");
    if (!arr || !cJSON_IsArray(arr)) { fprintf(stderr, "No 'processes' array\n"); exit(1); }
    printf("[MASTER] Processes array found\n");
    int n = cJSON_GetArraySize(arr);
    printf("[MASTER] Processing %d processes\n", n);
    for (int i = 0; i < n && i < MAX_TASKS; ++i) {
        printf("[MASTER] Processing process %d\n", i);
        cJSON* obj = cJSON_GetArrayItem(arr, i);
        cJSON* jname = cJSON_GetObjectItem(obj, "name");
        cJSON* jmust = cJSON_GetObjectItem(obj, "must_start");
        cJSON* jinprog = cJSON_GetObjectItem(obj, "st_in_progress");
        cJSON* jlaunch = cJSON_GetObjectItem(obj, "st_launched");
        cJSON* jreq = cJSON_GetObjectItem(obj, "required");
        Task* t = &tasks[task_count++];
        strncpy(t->name, jname ? jname->valuestring : "", 63);
        t->must_start = jmust ? cJSON_IsTrue(jmust) : false;
        t->st_in_progress = jinprog ? cJSON_IsTrue(jinprog) : false;
        t->st_launched = jlaunch ? cJSON_IsTrue(jlaunch) : false;
        t->pid = 0;
        t->required_count = 0;
        t->required = NULL;
        t->wait_counter = 0;  // Инициализация
        printf("[MASTER] Task %s initialized\n", t->name);
        if (jreq && cJSON_IsArray(jreq)) {
            int reqn = cJSON_GetArraySize(jreq);
            t->required = malloc(sizeof(char*) * reqn);
            t->required_count = reqn;
            for (int j = 0; j < reqn; ++j) {
                cJSON* jitem = cJSON_GetArrayItem(jreq, j);
                t->required[j] = strdup(jitem->valuestring);
            }
        }
    }
    printf("[MASTER] All processes processed\n");
    cJSON_Delete(root);
    free(data);
    printf("[MASTER] JSON cleanup completed\n");
}

// Все функции работают с Task* и массивом tasks[]
const ElfPayload* find_elf(const char* name) {
    for (int i = 0; elf_payloads[i].name; ++i) {
        if (strcmp(elf_payloads[i].name, name) == 0)
            return &elf_payloads[i];
    }
    return NULL;
}

// Запуск процесса по Task
int start_task(Task* t, const char* arg1) {
    const ElfPayload* elf = find_elf(t->name);
    if (!elf) {
        fprintf(stderr, "[MASTER] ELF for %s not found!\n", t->name);
        return -1;
    }
    int memfd = memfd_create(t->name, 0);
    if (memfd < 0) { perror("memfd_create"); return -1; }
    if (write(memfd, elf->data, elf->len) != elf->len) { perror("write"); close(memfd); return -1; }
    lseek(memfd, 0, SEEK_SET);
    pid_t pid = fork();
    if (pid == 0) {
        char* const argv[] = {(char*)t->name, (char*)arg1, NULL};
        char* const envp[] = {NULL};
        fexecve(memfd, argv, envp);
        perror("fexecve");
        exit(1);
    }
    close(memfd);
    t->pid = pid;
    t->st_launched = true;
    t->st_in_progress = false;
    printf("[MASTER] Started %s pid=%d\n", t->name, pid);
    return 0;
}

// Завершение процесса по Task
void stop_task(Task* t) {
    if (t->pid > 0) {
        printf("[MASTER] Shutdown %s pid=%d (SIGTERM)\n", t->name, t->pid);
        kill(t->pid, SIGTERM);
        int waited = 0;
        while (waited < 10) {
            int status;
            pid_t res = waitpid(t->pid, &status, WNOHANG);
            if (res == t->pid) break;
            sleep(1);
            waited++;
        }
        if (waited >= 10) {
            printf("[MASTER] %s do not shutdown by SIGTERM, send SIGKILL\n", t->name);
            kill(t->pid, SIGKILL);
            waitpid(t->pid, NULL, 0);
        }
        t->pid = 0;
        t->st_launched = false;
        t->st_in_progress = false;
    }
}

// Проверка, все ли required процессы запущены
int required_ready(Task* t) {
    for (int i = 0; i < t->required_count; ++i) {
        for (int j = 0; j < task_count; ++j) {
            if (strcmp(tasks[j].name, t->required[i]) == 0) {
                if (!tasks[j].st_launched) return 0;
            }
        }
    }
    return 1;
}

// Add new function to handle task logic
void task_dispatcher() {
    // Сначала проверяем и устанавливаем must_start для зависимостей
    for (int i = 0; i < task_count; ++i) {
        Task* t = &tasks[i];
        if (t->must_start && !t->st_launched) {
            // Если процесс должен запуститься, но не запущен, проверяем его зависимости
            for (int j = 0; j < t->required_count; ++j) {
                for (int k = 0; k < task_count; ++k) {
                    if (strcmp(tasks[k].name, t->required[j]) == 0) {
                        // Нашли зависимость, устанавливаем must_start=true
                        if (!tasks[k].must_start) {
                            printf("[MASTER] Auto-starting dependency %s for %s\n", tasks[k].name, t->name);
                            tasks[k].must_start = true;
                        }
                        break;
                    }
                }
            }
        }
    }
    
    // Проверяем, все ли процессы завершены для graceful shutdown
    int all_finished = 1;
    int any_was_started = 0;
    int active_processes = 0;
    for (int i = 0; i < task_count; ++i) {
        Task* t = &tasks[i];
        if (t->must_start || t->st_launched || t->st_in_progress) {
            all_finished = 0;
        }
        if (t->pid > 0) {
            any_was_started = 1;
            // Проверяем, действительно ли процесс ещё жив
            int status;
            pid_t res = waitpid(t->pid, &status, WNOHANG);
            if (res == 0) {
                // Процесс ещё жив
                active_processes++;
            } else if (res == t->pid) {
                // Процесс завершился
                t->pid = 0;
            } else {
                // Ошибка или процесс не существует
                t->pid = 0;
            }
        }
    }
    // Graceful shutdown только если нет активных процессов и были запущенные процессы
    if (all_finished && any_was_started && active_processes == 0) {
        printf("[MASTER] Graceful shutdown!\n");
        exit(0);
    }
    
    // Теперь обрабатываем каждый таск согласно логике диспетчера
    for (int i = 0; i < task_count; ++i) {
        Task* t = &tasks[i];
        if (t->must_start == false && t->st_in_progress == false && t->st_launched == false) {
            // Игнорируем
            continue;
        } else if (t->must_start == false && t->st_in_progress == false && t->st_launched == true) {
            // Отправляем сигнал на остановку и помечаем st_in_progress=true
            stop_task(t);
            t->st_in_progress = true;
            t->wait_counter = 0;
        } else if (t->must_start == false && t->st_in_progress == true && t->st_launched == true) {
            // Ожидаем счетчик и по превышению убиваем жестко
            if (t->wait_counter++ > 10) {  // Порог 10, настройте по необходимости
                if (t->pid > 0) {
                    kill(t->pid, SIGKILL);
                    waitpid(t->pid, NULL, 0);
                }
                t->st_in_progress = false;
                t->st_launched = false;
                t->wait_counter = 0;
            }
        } else if (t->must_start == true && t->st_in_progress == false && t->st_launched == false) {
            // Запускаем процесс, помечаем st_in_progress=true
            if (required_ready(t)) {
                start_task(t, NULL);
                t->st_in_progress = true;
                t->wait_counter = 0;
            }
        } else if (t->must_start == true && t->st_in_progress == true && t->st_launched == false) {
            // Ожидаем счетчик и по превышению убиваем и пытаемся перезапустить
            if (t->wait_counter++ > 10) {
                if (t->pid > 0) kill(t->pid, SIGKILL);
                waitpid(t->pid, NULL, 0);
                start_task(t, NULL);  // Пытаемся перезапустить
                t->wait_counter = 0;  // Обнуляем счетчик
            }
        } else if (t->must_start == true && t->st_in_progress == true && t->st_launched == true) {
            // Получен сигнал от дочернего процесса, меняем st_in_progress=false
            t->st_in_progress = false;
            t->wait_counter = 0;
        } else if (t->must_start == true && t->st_in_progress == false && t->st_launched == true) {
            // Игнорируем
            continue;
        }
    }
}

// --- Новый unified цикл обработки stdin и master_sock через select() ---
void command_loop() {
    static int initialized = 0;
    static int client_fds[MAX_CLIENTS] = {0};
    static int client_count = 0;
    
    int master_sock_fd = -1;
    for (int i = 0; i < socket_count; ++i) {
        if (strcmp(sockets[i].name, "master_sock") == 0) {
            master_sock_fd = sockets[i].fd;
            break;
        }
    }
    if (master_sock_fd < 0) {
        fprintf(stderr, "[MASTER] master_sock not found!\n");
        exit(1);
    }
    
    if (!initialized) {
        set_nonblocking(master_sock_fd);
        set_nonblocking(0); // stdin
        printf("[MASTER] Listening for commands on master_sock and stdin (non-blocking)...\n");
        initialized = 1;
    }

    struct pollfd pfds[1 + MAX_CLIENTS + 1]; // master_sock + clients + stdin
    
    // Обрабатываем команды с таймаутом 1 секунда
    int nfds = 0;
    pfds[nfds].fd = master_sock_fd;
    pfds[nfds].events = POLLIN;
    nfds++;
    pfds[nfds].fd = 0; // stdin
    pfds[nfds].events = POLLIN;
    nfds++;
    for (int i = 0; i < client_count; ++i) {
        pfds[nfds].fd = client_fds[i];
        pfds[nfds].events = POLLIN;
        nfds++;
    }
    int ret = poll(pfds, nfds, 1000); // Таймаут 1 секунда
    if (ret < 0) { perror("poll"); return; }
    if (ret == 0) { return; } // Таймаут, возвращаемся в main для вызова диспетчера
    
    int idx = 0;
    // master_sock
    if (pfds[idx].revents & POLLIN) {
        int client_fd = accept(master_sock_fd, NULL, NULL);
        if (client_fd >= 0) {
            set_nonblocking(client_fd);
            if (client_count < MAX_CLIENTS) {
                client_fds[client_count++] = client_fd;
            } else {
                close(client_fd);
            }
        }
    }
    idx++;
    // stdin
    if (pfds[idx].revents & POLLIN) {
        char cmd[256];
        if (fgets(cmd, sizeof(cmd), stdin)) {
            handle_master_command(cmd, 1, 1);
        }
    }
    idx++;
    // clients
    for (int i = 0; i < client_count; ++i) {
        if (pfds[idx + i].revents & POLLIN) {
            char buf[1024]; // Увеличиваем буфер
            ssize_t n = read(client_fds[i], buf, sizeof(buf)-1);
            if (n > 0) {
                buf[n] = 0;
                printf("[MASTER] Received: %s\n", buf);
                
                // Разбираем команды по строкам
                char *line = strtok(buf, "\n");
                while (line != NULL) {
                    // Пропускаем пустые строки
                    if (strlen(line) > 0) {
                        handle_master_command(line, 0, client_fds[i]);
                    }
                    line = strtok(NULL, "\n");
                }
            } else if (n <= 0) {
                // Закрываем соединение только при ошибке или закрытии клиента
                close(client_fds[i]);
                for (int j = i+1; j < client_count; ++j) client_fds[j-1] = client_fds[j];
                client_count--;
                i--;
            }
        }
    }
}
// --- Универсальный обработчик команд ---
void handle_master_command(const char* cmd, int to_stdout, int client_fd) {
    if (strncmp(cmd, "tasks", 5) == 0) {
        char out[1024] = "";
        for (int i = 0; i < task_count; ++i) {
            char line[128];
            snprintf(line, sizeof(line), "%s: pid=%d, must_start=%d, in_progress=%d, launched=%d\n", tasks[i].name, tasks[i].pid, tasks[i].must_start, tasks[i].st_in_progress, tasks[i].st_launched);
            strncat(out, line, sizeof(out)-strlen(out)-1);
        }
        if (to_stdout) printf("%s", out); else write(client_fd, out, strlen(out));
    } else if (strncmp(cmd, "exit", 4) == 0) {
        for (int i = 0; i < task_count; ++i) tasks[i].must_start = false;
        if (!to_stdout) close(client_fd);
        exit(0);
    } else if (strncmp(cmd, "start ", 6) == 0) {
        char name[64];
        strncpy(name, cmd+6, 63);
        name[strcspn(name, "\n")] = 0;
        int found = 0;
        for (int i = 0; i < task_count; ++i) {
            if (strcmp(tasks[i].name, name) == 0) {
                found = 1;
                tasks[i].must_start = true;
                if (!to_stdout) write(client_fd, "STATUS_UPDATED\n", 15);
            }
        }
        if (!found && !to_stdout) write(client_fd, "NOTFOUND\n", 9);
    } else if (strncmp(cmd, "stop ", 5) == 0) {
        char name[64];
        strncpy(name, cmd+5, 63);
        name[strcspn(name, "\n")] = 0;
        int found = 0;
        for (int i = 0; i < task_count; ++i) {
            if (strcmp(tasks[i].name, name) == 0) {
                found = 1;
                tasks[i].must_start = false;
                if (!to_stdout) write(client_fd, "STATUS_UPDATED\n", 15);
            }
        }
        if (!found && !to_stdout) write(client_fd, "NOTFOUND\n", 9);
    } else if (strncmp(cmd, "launched ", 9) == 0) {
        char name[64];
        strncpy(name, cmd+9, 63);
        name[strcspn(name, "\n")] = 0;
        int found = 0;
        for (int i = 0; i < task_count; ++i) {
            if (strcmp(tasks[i].name, name) == 0) {
                found = 1;
                tasks[i].st_launched = true;  // Set st_launched to true
                tasks[i].st_in_progress = false;  // Set st_in_progress to false
                if (!to_stdout) write(client_fd, "STATUS_UPDATED\n", 15);
            }
        }
        if (!found && !to_stdout) write(client_fd, "NOTFOUND\n", 9);
    } else if (strncmp(cmd, "stopped ", 8) == 0) {
        char name[64];
        strncpy(name, cmd+8, 63);
        name[strcspn(name, "\n")] = 0;
        int found = 0;
        for (int i = 0; i < task_count; ++i) {
            if (strcmp(tasks[i].name, name) == 0) {
                found = 1;
                tasks[i].st_launched = false;  // Set st_launched to false
                tasks[i].st_in_progress = false;  // Set st_in_progress to false
                if (!to_stdout) write(client_fd, "STATUS_UPDATED\n", 15);
            }
        }
        if (!found && !to_stdout) write(client_fd, "NOTFOUND\n", 9);
    } else {
        if (!to_stdout) write(client_fd, "UNKNOWN\n", 8);
        else printf("[MASTER] Unknown command\n");
    }
} 

int main() {
    printf("[MASTER] Starting...\n");
    
    // Игнорируем SIGPIPE, чтобы не завершаться при закрытии соединений дочерними процессами
    signal(SIGPIPE, SIG_IGN);
    
    init_elf_payloads();
    printf("[MASTER] ELF payloads initialized\n");
    load_tasks_from_json("./processes.json");
    printf("[MASTER] Tasks loaded from JSON\n");
    
    // Периодический бесконечный цикл с вызовом диспетчера и обработкой команд
    time_t last_dispatch = time(NULL);
    
    while (1) {
        // Вызываем диспетчер каждые 5 секунд
        if (time(NULL) - last_dispatch >= 5) {
            printf("[MASTER] Calling task_dispatcher...\n");
            task_dispatcher();
            last_dispatch = time(NULL);
        }
        
        // Обрабатываем команды с коротким таймаутом
        command_loop();
    }
    return 0;
}