package main

/*
#cgo CFLAGS: -I..
#cgo LDFLAGS: -lpthread
#include "cgo_wrappers.h"

// Обертки для работы с потоками
#include <pthread.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/types.h>
#include <signal.h>

typedef struct {
    pthread_t thread;
    int id;
    void (*func)(void*);
    void* arg;
    int running;
} ThreadData;

typedef struct {
    ThreadData* threads;
    int capacity;
    int count;
    pthread_mutex_t mutex;
} ThreadManager;

static ThreadManager* threadManager = NULL;

static ThreadManager* createThreadManager() {
    ThreadManager* manager = malloc(sizeof(ThreadManager));
    manager->capacity = 10;
    manager->count = 0;
    manager->threads = malloc(sizeof(ThreadData) * manager->capacity);
    pthread_mutex_init(&manager->mutex, NULL);
    return manager;
}

static int startThread(void (*func)(void*), void* arg) {
    if (!threadManager) {
        threadManager = createThreadManager();
    }

    pthread_mutex_lock(&threadManager->mutex);

    if (threadManager->count >= threadManager->capacity) {
        // Расширяем массив
        int newCapacity = threadManager->capacity * 2;
        ThreadData* newThreads = realloc(threadManager->threads, sizeof(ThreadData) * newCapacity);
        threadManager->threads = newThreads;
        threadManager->capacity = newCapacity;
    }

    int id = threadManager->count + 1;
    ThreadData* data = &threadManager->threads[threadManager->count];
    data->id = id;
    data->func = func;
    data->arg = arg;
    data->running = 1;

    pthread_create(&data->thread, NULL, (void* (*)(void*))func, arg);
    threadManager->count++;

    pthread_mutex_unlock(&threadManager->mutex);
    return id;
}

static void stopThread(int id) {
    if (!threadManager) return;

    pthread_mutex_lock(&threadManager->mutex);

    for (int i = 0; i < threadManager->count; i++) {
        if (threadManager->threads[i].id == id && threadManager->threads[i].running) {
            // Используем pthread_kill вместо pthread_cancel
            pthread_kill(threadManager->threads[i].thread, SIGTERM);
            threadManager->threads[i].running = 0;
            break;
        }
    }

    pthread_mutex_unlock(&threadManager->mutex);
}

static void cleanupThread(int id) {
    if (!threadManager) return;

    pthread_mutex_lock(&threadManager->mutex);

    for (int i = 0; i < threadManager->count; i++) {
        if (threadManager->threads[i].id == id) {
            if (threadManager->threads[i].running) {
                pthread_join(threadManager->threads[i].thread, NULL);
            }
            // Сдвигаем остальные элементы
            for (int j = i; j < threadManager->count - 1; j++) {
                threadManager->threads[j] = threadManager->threads[j + 1];
            }
            threadManager->count--;
            break;
        }
    }

    pthread_mutex_unlock(&threadManager->mutex);
}

// Go функции
void GoThreadFunc(void*);
void GoLoggerFunc(void*);
void GoWorkerFunc(void*);

typedef void (*void_func_ptr)(void*);

static void* getGoThreadFuncPtr() { return (void*)GoThreadFunc; }
static void* getGoLoggerFuncPtr() { return (void*)GoLoggerFunc; }
static void* getGoWorkerFuncPtr() { return (void*)GoWorkerFunc; }
*/
import "C"
import (
	"fmt"
	"os/signal"
	"syscall"
	"time"
	"unsafe"
)

//export GoThreadFunc
func GoThreadFunc(arg unsafe.Pointer) {
	data := (*struct {
		id  int
		msg *C.char
	})(arg)

	msg := C.GoString(data.msg)
	fmt.Printf("Thread %d started: %s\n", data.id, msg)

	for i := 1; i <= 15; i++ {
		fmt.Printf("Thread %d: working %d/15\n", data.id, i)
		time.Sleep(500 * time.Millisecond)
		// Точка отмены для pthread_cancel
		C.usleep(0)
	}

	fmt.Printf("Thread %d completed\n", data.id)
	C.free(unsafe.Pointer(data.msg))
}

//export GoLoggerFunc
func GoLoggerFunc(arg unsafe.Pointer) {
	fmt.Println("Logger started")

	// Эмуляция работы логгера
	for i := 1; i <= 20; i++ {
		fmt.Printf("Logger: processing message %d/20\n", i)
		time.Sleep(300 * time.Millisecond)
		// Точка отмены для pthread_cancel
		C.usleep(0)
	}

	fmt.Println("Logger completed")
}

//export GoWorkerFunc
func GoWorkerFunc(arg unsafe.Pointer) {
	data := (*struct {
		id   int
		name *C.char
	})(arg)

	name := C.GoString(data.name)
	fmt.Printf("Worker %d (%s) started\n", data.id, name)

	for i := 1; i <= 10; i++ {
		fmt.Printf("Worker %d (%s): task %d/10\n", data.id, name, i)
		time.Sleep(400 * time.Millisecond)
	}

	fmt.Printf("Worker %d (%s) completed\n", data.id, name)
	C.free(unsafe.Pointer(data.name))
}

func main() {
	// Игнорируем SIGPIPE для предотвращения крашей
	signal.Ignore(syscall.SIGPIPE)

	fmt.Println("Starting thread wrapper test...")

	var threadIds []int

	// Запуск логгера
	loggerArgs := &struct {
		id int
	}{
		id: 0,
	}
	loggerId := int(C.startThread((C.void_func_ptr)(C.getGoLoggerFuncPtr()), unsafe.Pointer(loggerArgs)))
	threadIds = append(threadIds, loggerId)
	time.Sleep(100 * time.Millisecond)

	// Запуск рабочих потоков
	for i := 1; i <= 3; i++ {
		workerArgs := &struct {
			id   int
			name *C.char
		}{
			id:   i,
			name: C.CString(fmt.Sprintf("worker_%d", i)),
		}

		threadId := int(C.startThread((C.void_func_ptr)(C.getGoWorkerFuncPtr()), unsafe.Pointer(workerArgs)))
		threadIds = append(threadIds, threadId)
		time.Sleep(100 * time.Millisecond)
	}

	// Даем поработать немного
	time.Sleep(2 * time.Second)

	// Принудительное завершение 2-го рабочего потока во время работы
	fmt.Println("Force stopping worker thread 2")
	C.stopThread(C.int(threadIds[2])) // threadIds[2] = worker 2

	// Ожидание завершения
	time.Sleep(3 * time.Second)

	// Очистка ресурсов
	for _, id := range threadIds {
		C.cleanupThread(C.int(id))
	}

	fmt.Println("All threads terminated (may have crashed due to unsafe termination)")
}
