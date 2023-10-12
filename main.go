package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/eduardocfalcao/delayed-funcs/task"
)

func Info(info string) {
	log.Printf("[Info] %s", info)
}

func Task(ctx context.Context) {
	Info("[Task] Doing some task...")
}

func LongTask(ctx context.Context) {
	d, _ := time.ParseDuration("5s")
	Info("[Long Task] Start running some long task...")

	time.Sleep(d)

	Info("[Long Task] Finished running the long task...")
}

func main() {

	address := fmt.Sprintf(":%d", 8081)
	mux := http.NewServeMux()

	delayedTask := task.NewDelayedTaskChan(Task)
	mux.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		delayedTask.Run()
	})

	longDelayedTask := task.NewDelayedTaskChan(LongTask)
	mux.HandleFunc("/long-process", func(w http.ResponseWriter, r *http.Request) {
		longDelayedTask.Run()
	})

	server := &http.Server{
		Addr:         address,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		log.Printf("Starting http server. Listening port %s", address)
		if err := server.ListenAndServe(); err != nil {
			log.Print(err)
			cancel()
		}
	}()

	select {
	case <-c:
		log.Print("Stopping server...")
		server.Shutdown(ctx)
		os.Exit(0)
	case <-ctx.Done():
	}
}

// Initial State
// Stopped ->
// Delaying ->
// Delaying ->
// Running -> Stopped
type TaskStatus int

const (
	Stopped TaskStatus = iota
	Running
	Delaying
)

type DelayedTask struct {
	status TaskStatus
	Task   func(ctx context.Context)

	safMu            sync.Mutex
	startAfterFinish bool

	lastCallRequest time.Time
	lcrMu           sync.Mutex
}

func NewDelayedTask(fn func(ctx context.Context)) *DelayedTask {
	return &DelayedTask{
		Task:            fn,
		lastCallRequest: time.Now(),
	}
}

func (d *DelayedTask) Run() {
	switch d.status {
	case Stopped:
		d.startDelayedCall()
	case Delaying:
		d.increaseDelay()
	case Running:
		d.safMu.Lock()
		defer d.safMu.Unlock()
		Info("Setting start after finish to true")
		d.startAfterFinish = true
	}
}

func (d *DelayedTask) startDelayedCall() {
	Info("Starting delayed task")
	d.status = Delaying

	go func() {
		duration, _ := time.ParseDuration("5s")
		ticker := time.NewTicker(duration)
		<-ticker.C

		for {
			d := time.Since(d.lastCallRequest)
			if d.Seconds() > 5 {
				break
			}
			ticker.Reset(duration)
			<-ticker.C
		}
		d.status = Running

		Info("Starting to run the delayed task")
		d.Task(context.Background())
		Info("Finished to run the delayed task")
		d.status = Stopped

		if d.startAfterFinish {
			d.startAfterFinish = false
			Info("Restart delayed task")
			d.startDelayedCall()
		}
	}()
}

func (d *DelayedTask) increaseDelay() {
	Info("Increasing delay")
	d.status = Delaying
	d.lcrMu.Lock()
	defer d.lcrMu.Unlock()

	d.lastCallRequest = time.Now()
}
