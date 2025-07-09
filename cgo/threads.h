#ifndef THREADS_H
#define THREADS_H

#include <pthread.h>
#include <signal.h>
#include <unistd.h>
#include <sys/syscall.h>
#include <time.h>

// Определение pthread_timedjoin_np для систем, где оно отсутствует
#ifndef __USE_GNU
#define __USE_GNU
#endif

#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif

#ifndef PTHREAD_TIMEDJOIN_NP_DEFINED
#define PTHREAD_TIMEDJOIN_NP_DEFINED
// Если функция pthread_timedjoin_np не определена, определяем её сами
#ifndef __linux__
static int pthread_timedjoin_np(pthread_t thread, void **retval, const struct timespec *abstime) {
    // Пытаемся присоединиться к потоку без таймаута
    return pthread_join(thread, retval);
}
#endif
#endif

// Статус потока
struct thread_status {
    int id;
    int is_running;
    int is_completed;
    int is_terminated;
};

// Инициализация системы потоков
void init_thread_system();

// Создание потока
int create_thread(void* go_arg);

// Принудительное завершение потока
int terminate_thread(int id);

// Получение статуса потока
struct thread_status get_thread_status(int id);

// Очистка ресурсов завершенных потоков
void cleanup_threads();

// Подсчет количества активных потоков
int count_active_threads();

#endif // THREADS_H 