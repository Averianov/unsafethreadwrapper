#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <sys/syscall.h>
#include <pthread.h>
#include "threads.h"

// Максимальное количество потоков
#define MAX_THREADS 1024

// Информация о потоке
typedef struct {
    pthread_t thread;
    int valid;
    int running;
    int completed;
    int terminated;
    pid_t tid;
    void* arg;
    volatile int should_exit; // Флаг для завершения
} thread_info;

// Глобальный массив потоков
static thread_info threads[MAX_THREADS];
static pthread_mutex_t global_mutex = PTHREAD_MUTEX_INITIALIZER;
static int next_thread_id = 0;

// Объявление Go-функции, которая будет вызывать Go-колбэк
extern void go_thread_callback(void* arg);

// Функция выполнения потока
static void* thread_function(void* arg) {
    int id = *((int*)arg);
    free(arg); // Освобождаем аргумент сразу
    
    void* go_arg = NULL;
    
    // Запоминаем системный ID потока
    pid_t tid = syscall(SYS_gettid);
    
    // Блокируем мьютекс
    pthread_mutex_lock(&global_mutex);
    
    // Отмечаем, что поток запущен и сохраняем его TID
    threads[id].running = 1;
    threads[id].tid = tid;
    go_arg = threads[id].arg;
    
    // Разблокируем мьютекс
    pthread_mutex_unlock(&global_mutex);
    
    // Вызываем Go-колбэк
    if (go_arg != NULL) {
        go_thread_callback(go_arg);
    }
    
    // Блокируем мьютекс
    pthread_mutex_lock(&global_mutex);
    
    // Отмечаем, что поток завершен
    threads[id].running = 0;
    threads[id].completed = 1;
    
    // Разблокируем мьютекс
    pthread_mutex_unlock(&global_mutex);
    
    return NULL;
}

// Инициализация системы потоков
void init_thread_system() {
    // Блокируем мьютекс
    pthread_mutex_lock(&global_mutex);
    
    // Инициализируем массив потоков
    for (int i = 0; i < MAX_THREADS; i++) {
        threads[i].valid = 0;
        threads[i].should_exit = 0;
    }
    next_thread_id = 0;
    
    // Разблокируем мьютекс
    pthread_mutex_unlock(&global_mutex);
}

// Создание потока
int create_thread(void* go_arg) {
    int id = -1;
    
    // Блокируем мьютекс
    pthread_mutex_lock(&global_mutex);
    
    // Ищем свободный слот
    for (int i = 0; i < MAX_THREADS; i++) {
        if (!threads[i].valid) {
            id = i;
            break;
        }
    }
    
    // Если свободного слота нет, используем новый ID
    if (id == -1) {
        if (next_thread_id >= MAX_THREADS) {
            // Слишком много потоков
            pthread_mutex_unlock(&global_mutex);
            return -1;
        }
        id = next_thread_id++;
    }
    
    // Инициализируем информацию о потоке
    threads[id].valid = 1;
    threads[id].running = 0;
    threads[id].completed = 0;
    threads[id].terminated = 0;
    threads[id].should_exit = 0;
    threads[id].arg = go_arg;
    
    // Создаем аргумент для потока - копируем ID
    int* arg = (int*)malloc(sizeof(int));
    if (arg == NULL) {
        threads[id].valid = 0;
        pthread_mutex_unlock(&global_mutex);
        return -1;
    }
    *arg = id;
    
    // Создаем поток
    if (pthread_create(&threads[id].thread, NULL, thread_function, arg) != 0) {
        free(arg);
        threads[id].valid = 0;
        pthread_mutex_unlock(&global_mutex);
        return -1;
    }
    
    // Разблокируем мьютекс
    pthread_mutex_unlock(&global_mutex);
    
    return id;
}

// Принудительное завершение потока
int terminate_thread(int id) {
    if (id < 0 || id >= MAX_THREADS) {
        return -1;
    }
    
    // Блокируем мьютекс
    pthread_mutex_lock(&global_mutex);
    
    // Проверяем, существует ли поток и активен ли он
    if (!threads[id].valid || !threads[id].running) {
        pthread_mutex_unlock(&global_mutex);
        return 0; // Поток уже завершен или не существует
    }
    
    // Запоминаем информацию о потоке
    pthread_t thread = threads[id].thread;
    
    // Отмечаем поток как завершенный принудительно
    threads[id].running = 0;
    threads[id].terminated = 1;
    threads[id].should_exit = 1;
    
    // Разблокируем мьютекс
    pthread_mutex_unlock(&global_mutex);
    
    // Пробуем отменить поток через pthread_cancel (безопасный способ)
    int result = pthread_cancel(thread);
    
    // Отсоединяем поток, чтобы избежать утечек ресурсов
    pthread_detach(thread);
    
    return 0;
}

// Получение статуса потока
struct thread_status get_thread_status(int id) {
    struct thread_status status;
    status.id = id;
    status.is_running = 0;
    status.is_completed = 0;
    status.is_terminated = 0;
    
    if (id < 0 || id >= MAX_THREADS) {
        return status;
    }
    
    // Блокируем мьютекс
    pthread_mutex_lock(&global_mutex);
    
    // Если поток существует, получаем его статус
    if (threads[id].valid) {
        status.is_running = threads[id].running;
        status.is_completed = threads[id].completed;
        status.is_terminated = threads[id].terminated;
    }
    
    // Разблокируем мьютекс
    pthread_mutex_unlock(&global_mutex);
    
    return status;
}

// Очистка ресурсов завершенных потоков
void cleanup_threads() {
    // Блокируем мьютекс
    pthread_mutex_lock(&global_mutex);
    
    // Проходим по всем потокам
    for (int i = 0; i < MAX_THREADS; i++) {
        if (threads[i].valid && (threads[i].completed || threads[i].terminated)) {
            // Отмечаем поток как недействительный
            threads[i].valid = 0;
        }
    }
    
    // Разблокируем мьютекс
    pthread_mutex_unlock(&global_mutex);
}

// Подсчет количества активных потоков
int count_active_threads() {
    int count = 0;
    
    // Блокируем мьютекс
    pthread_mutex_lock(&global_mutex);
    
    // Считаем активные потоки
    for (int i = 0; i < MAX_THREADS; i++) {
        if (threads[i].valid && threads[i].running) {
            count++;
        }
    }
    
    // Разблокируем мьютекс
    pthread_mutex_unlock(&global_mutex);
    
    return count;
} 