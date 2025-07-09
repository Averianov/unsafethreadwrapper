// Package threadmanager обеспечивает безкомпромиссное управление потоками
package cgo

/*
#include "threads.h"
#include <pthread.h>

// Настройка для потоков, чтобы они могли быть отменены в любой момент
static void setup_thread_for_cancellation() {
    // Устанавливаем тип отмены - немедленный
    pthread_setcanceltype(PTHREAD_CANCEL_ASYNCHRONOUS, NULL);

    // Устанавливаем состояние отмены - разрешено
    pthread_setcancelstate(PTHREAD_CANCEL_ENABLE, NULL);
}
*/
import "C"
import (
	"fmt"
	"runtime/cgo"
	"unsafe"
)

// Инициализируем систему потоков при загрузке пакета
func init() {
	C.init_thread_system()
}

// Thread представляет управляемый поток
type Thread struct {
	id int
}

// Status представляет статус потока
type Status struct {
	ID           int
	IsRunning    bool
	IsCompleted  bool
	IsTerminated bool
}

// ThreadFunc представляет функцию для выполнения в потоке
type ThreadFunc func()

//export go_thread_callback
func go_thread_callback(arg unsafe.Pointer) {
	// Настраиваем поток для возможности отмены
	C.setup_thread_for_cancellation()

	// Извлекаем Go-функцию из хендла
	h := *(*cgo.Handle)(arg)
	fn, ok := h.Value().(ThreadFunc)
	if ok && fn != nil {
		// Выполняем функцию в потоке
		fn()
	}
	// Освобождаем ресурсы хендла
	h.Delete()
}

// Create создает и запускает новый поток
func Create(fn ThreadFunc) (*Thread, error) {
	if fn == nil {
		return nil, fmt.Errorf("функция не может быть nil")
	}

	// Создаем хендл для Go-функции
	h := cgo.NewHandle(fn)
	ptr := unsafe.Pointer(&h)

	// Создаем поток в C
	id := int(C.create_thread(ptr))
	if id < 0 {
		// Освобождаем хендл при ошибке
		h.Delete()
		return nil, fmt.Errorf("не удалось создать поток")
	}

	return &Thread{id: id}, nil
}

// Terminate принудительно завершает поток
func (t *Thread) Terminate() error {
	result := C.terminate_thread(C.int(t.id))
	if result < 0 {
		return fmt.Errorf("не удалось завершить поток %d", t.id)
	}
	return nil
}

// GetStatus возвращает статус потока
func (t *Thread) GetStatus() Status {
	cStatus := C.get_thread_status(C.int(t.id))
	return Status{
		ID:           t.id,
		IsRunning:    cStatus.is_running != 0,
		IsCompleted:  cStatus.is_completed != 0,
		IsTerminated: cStatus.is_terminated != 0,
	}
}

// GetID возвращает ID потока
func (t *Thread) GetID() int {
	return t.id
}

// Cleanup освобождает ресурсы завершенных потоков
func Cleanup() {
	C.cleanup_threads()
}

// CountActive возвращает количество активных потоков
func CountActive() int {
	return int(C.count_active_threads())
}
