package workerpool

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type Job interface {}

type Result interface {}

type ProcessFunc func(ctx context.Context, job Job) Result

type WorkerPool interface {
	Start(ctx context.Context, inputCh <- chan Job)
	Stop() error
	IsRunning() bool
}

type State int 

const (
	StateIdle State = iota
	StateRunning
	StateStopping
)

type Config struct {
	NumWorkers int
	QueueSize int
	Logger *slog.Logger
}

func DefaultConfig() Config {
	return Config{
		NumWorkers: 1,
		QueueSize: 100,
		Logger: slog.Default(),
	}
}

type workerPool struct {
	NumWorkers int
	processFunc ProcessFunc
	state State
	stateMutex sync.Mutex
	logger *slog.Logger
	stopCh chan struct{}
	stopWg sync.WaitGroup
}

func New(ProcessFunc ProcessFunc, config Config) *workerPool{
	// validate config
	if config.NumWorkers <= 0 {
		config.NumWorkers = 1
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 1
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &workerPool{
		NumWorkers: config.NumWorkers,
		processFunc: ProcessFunc,
		state: StateIdle,
		logger: config.Logger,
		stopCh: make(chan struct{}),
	}
}

func (wp *workerPool) Start(ctx context.Context, inputCh <- chan Job) (<- chan Result, error) {
	wp.stateMutex.Lock()
	defer wp.stateMutex.Unlock()

	if wp.state != StateIdle {
		return nil, fmt.Errorf("Worker pool is not idle")
	}

	resultCh := make(chan Result, wp.NumWorkers)

	wp.state = StateRunning
	wp.stopCh = make(chan struct{})

	wp.stopWg.Add(wp.NumWorkers)

	for i := 0; i < wp.NumWorkers; i++ {
		go wp.worker(ctx, i, inputCh, resultCh)
	}

	go func () {
		wp.stopWg.Wait()
		close(resultCh)

		wp.stateMutex.Lock()
		wp.state = StateIdle
		wp.stateMutex.Unlock()
	}()

	return resultCh, nil
}

func (wp *workerPool) Stop() error {
	wp.stateMutex.Lock()
	defer wp.stateMutex.Unlock()

	if wp.state != StateRunning {
		return fmt.Errorf("Worker pool is not running")
	}

	wp.state = StateStopping
	close(wp.stopCh)

	wp.stopWg.Wait()

	wp.state = StateIdle

	return nil
}

func (wp *workerPool) IsRunning() bool {
	wp.stateMutex.Lock()
	defer wp.stateMutex.Unlock()

	return wp.state == StateRunning
}

func (wp *workerPool) worker(ctx context.Context, workerID int, inputCh <- chan Job, resultCh chan <- Result) {
	wp.logger.Info("Worker started", "workerID", workerID)

	for {
		select {
		case <- wp.stopCh:
			wp.logger.Info("Worker stopped", "workerID", workerID)
			return
		case <- ctx.Done():
			wp.logger.Info("Context canceled, interrupting worker", "workerID", workerID)
		case job, ok := <- inputCh:
			if !ok {
				wp.logger.Info("Input channel closed", "workerID", workerID)
				return
			}

			result := wp.processFunc(ctx, job)
			wp.logger.Info("Job processed", "workerID", workerID, "job", job, "result", result)
			select {
				case resultCh <- result:
				case <- wp.stopCh:
					wp.logger.Info("Worker stopped", "workerID", workerID)
				case <- ctx.Done():
					wp.logger.Info("Context canceled, interrupting worker", "workerID", workerID)
			}
		}
	}
}