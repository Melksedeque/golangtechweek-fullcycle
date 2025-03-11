package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/Melksedeque/golangtechweek-fullcycle/pkg/workerpool"
)

type NumeroJob struct {
	Numero int
}

type ResultadoNumero struct {
	Numero    int
	workerID  int
	Timestamp time.Time
}

func processarNumero(ctx context.Context, job workerpool.Job) workerpool.Result {
	numero := job.(NumeroJob).Numero
	workerID := numero % 3
	sleepTime := time.Duration(800 + rand.Intn(400)) * time.Millisecond
	time.Sleep(sleepTime)

	return ResultadoNumero {
		Numero: numero,
		workerID: workerID,
		Timestamp: time.Now(),
	}
} 

func main() {
	valorMaximo := 20
	bufferSize := 10
	var wg sync.WaitGroup

	pool := workerpool.New(processarNumero, workerpool.Config{
		NumWorkers: 3,
	})

	inputCh := make(chan workerpool.Job, bufferSize)
	ctx := context.Background()

	resultCh, erro := pool.Start(ctx, inputCh)
	if erro != nil {
		panic(erro)
	}

	wg.Add(valorMaximo)

	fmt.Println("Iniciando o pool de workers com conta de ", valorMaximo, "numeros")

	go func() {
		for i := 0; i < valorMaximo; i ++ {
			inputCh <- NumeroJob{Numero: i}
		}
		close(inputCh)
	}()

	go func() {
		for resultado := range resultCh {
			r := resultado.(ResultadoNumero)
			fmt.Printf("Resultado: %d, WorkerID: %d, Timestamp: %s\n", r.Numero, r.workerID, r.Timestamp)
			wg.Done()
		}
	}()

	wg.Wait()
	fmt.Printf("Pool de Workers finalizados\nTodos os %d números foram processados\n", valorMaximo)
}