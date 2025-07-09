package main

import (
	"fmt"
	"sync/atomic"
	"time"

	tm "github.com/Averianov/unsafethreadwrapper"
)

// Флаги для принудительного завершения
var (
	stopSimpleFunction int32
	stopParamFunction  int32
	stopWorkerProcess  int32
)

func main() {
	fmt.Println("Запуск демонстрации работы с нативными потоками...")
	RunDemo()
	fmt.Println("Демонстрация завершена.")
}

// #########################################################################################

// Глобальный канал для передачи сообщений в логгер
var logChannel = make(chan string, 100)

// Logger - функция-логгер, выполняется в нативном pthread БЕЗ горутин
func Logger() {
	fmt.Println("   [LOGGER] Запущен в нативном pthread")
	for i := 0; i < 1000; i++ {
		select {
		case msg := <-logChannel:
			fmt.Printf("   [LOGGER] %s\n", msg)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	fmt.Println("   [LOGGER] Завершен по истечению времени")
}

// SimpleFunction - простая функция без параметров
// Проверяет флаг завершения
func SimpleFunction() {
	for i := 1; i <= 30; i++ {
		// Проверяем флаг завершения
		if atomic.LoadInt32(&stopSimpleFunction) != 0 {
			logChannel <- "SimpleFunction: принудительно завершена"
			return
		}

		logChannel <- fmt.Sprintf("SimpleFunction: итерация %d (pthread)", i)
		time.Sleep(200 * time.Millisecond)
	}
	logChannel <- "SimpleFunction: завершена по истечению времени"
}

// ParametrizedFunction - функция с параметрами
// Проверяет флаг завершения
func ParametrizedFunction(name string, count int) func() {
	return func() {
		for i := 1; i <= count; i++ {
			// Проверяем флаг завершения
			if atomic.LoadInt32(&stopParamFunction) != 0 {
				logChannel <- fmt.Sprintf("%s: принудительно завершена", name)
				return
			}

			logChannel <- fmt.Sprintf("%s: итерация %d из %d (pthread)", name, i, count)
			time.Sleep(300 * time.Millisecond)
		}
		logChannel <- fmt.Sprintf("%s: завершена по истечению времени", name)
	}
}

// Worker - структура с методом
type Worker struct {
	Name string
}

// Process - метод структуры Worker
// Проверяет флаг завершения
func (w *Worker) Process() {
	for i := 1; i <= 30; i++ {
		// Проверяем флаг завершения
		if atomic.LoadInt32(&stopWorkerProcess) != 0 {
			logChannel <- fmt.Sprintf("Worker[%s]: принудительно завершен", w.Name)
			return
		}

		logChannel <- fmt.Sprintf("Worker[%s]: обработка элемента %d (pthread)", w.Name, i)
		time.Sleep(250 * time.Millisecond)
	}
	logChannel <- fmt.Sprintf("Worker[%s]: завершен по истечению времени", w.Name)
}

// RunDemo запускает демонстрацию работы с потоками
func RunDemo() {
	fmt.Println("=== ДЕМОНСТРАЦИЯ РАБОТЫ С НАТИВНЫМИ ПОТОКАМИ ===")

	// Сбрасываем флаги завершения
	atomic.StoreInt32(&stopSimpleFunction, 0)
	atomic.StoreInt32(&stopParamFunction, 0)
	atomic.StoreInt32(&stopWorkerProcess, 0)

	// 1. Запускаем функцию-логгер первой
	fmt.Println("1. Запуск логгера в нативном pthread:")
	_, err := tm.Create(Logger)
	if err != nil {
		fmt.Println("Ошибка создания логгера:", err)
		return
	}

	time.Sleep(200 * time.Millisecond)

	// 2. Запускаем простую функцию
	fmt.Println("2. Запуск простой функции в pthread:")
	simpleThread, err := tm.Create(SimpleFunction)
	if err != nil {
		fmt.Println("Ошибка создания простого потока:", err)
		return
	}

	// 3. Запускаем функцию с параметрами
	fmt.Println("3. Запуск функции с параметрами в pthread:")
	paramFunc := ParametrizedFunction("ParamFunc", 30)
	paramThread, err := tm.Create(paramFunc)
	if err != nil {
		fmt.Println("Ошибка создания параметризованного потока:", err)
		return
	}

	// 4. Запускаем метод структуры
	fmt.Println("4. Запуск метода структуры в pthread:")
	worker := &Worker{Name: "DataProcessor"}
	methodThread, err := tm.Create(worker.Process)
	if err != nil {
		fmt.Println("Ошибка создания потока метода:", err)
		return
	}

	fmt.Printf("\nАктивных нативных pthread: %d\n", tm.CountActive())

	// Даем потокам поработать
	time.Sleep(3 * time.Second)

	// Проверяем статусы ДО завершения
	fmt.Println("\n6. Статусы ДО принудительного завершения:")
	fmt.Printf("   SimpleFunction: %+v\n", simpleThread.GetStatus())
	fmt.Printf("   ParamFunction: %+v\n", paramThread.GetStatus())
	fmt.Printf("   Worker.Process: %+v\n", methodThread.GetStatus())

	// ПРИНУДИТЕЛЬНО завершаем потоки снаружи
	fmt.Println("\n7. ПРИНУДИТЕЛЬНОЕ завершение снаружи:")
	fmt.Println("   SimpleFunction - жёсткое завершение...")
	atomic.StoreInt32(&stopSimpleFunction, 1)

	time.Sleep(100 * time.Millisecond)

	fmt.Println("   ParamFunction - жёсткое завершение...")
	atomic.StoreInt32(&stopParamFunction, 1)

	time.Sleep(100 * time.Millisecond)

	fmt.Println("   Worker.Process - жёсткое завершение...")
	atomic.StoreInt32(&stopWorkerProcess, 1)

	// Даем время на обработку сигналов
	time.Sleep(500 * time.Millisecond)

	// Проверяем статусы ПОСЛЕ завершения
	fmt.Println("\n8. Статусы ПОСЛЕ принудительного завершения:")
	fmt.Printf("   SimpleFunction: %+v\n", simpleThread.GetStatus())
	fmt.Printf("   ParamFunction: %+v\n", paramThread.GetStatus())
	fmt.Printf("   Worker.Process: %+v\n", methodThread.GetStatus())

	fmt.Printf("\nАктивных pthread: %d\n", tm.CountActive())

	// Завершаем логгер последним
	fmt.Println("\n9. Завершаем логгер последним:")
	// Не используем Terminate для логгера, просто ждем его завершения
	time.Sleep(500 * time.Millisecond)

	// Очистка
	fmt.Println("\n10. Очистка ресурсов завершенных потоков:")
	tm.Cleanup()

	// Ждем, чтобы увидеть результаты
	time.Sleep(5 * time.Second)

	fmt.Println("\n=== ДЕМОНСТРАЦИЯ ЗАВЕРШЕНА ===")
	fmt.Printf("Осталось активных pthread: %d\n", tm.CountActive())
}
