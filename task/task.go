package task

import (
	"context"
	"log"
	"sync"
	"time"
)

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

type DelayedTaskChan struct {
	status TaskStatus
	Task   func(ctx context.Context)

	safMu            sync.Mutex
	startAfterFinish bool

	delayC chan any
}

func NewDelayedTaskChan(fn func(ctx context.Context)) *DelayedTaskChan {
	return &DelayedTaskChan{
		Task:   fn,
		delayC: make(chan any),
	}
}

func (d *DelayedTaskChan) Run() {
	switch d.status {
	case Stopped:
		d.startDelayedCall()
	case Delaying:
		d.increaseDelay()
	case Running:
		d.safMu.Lock()
		defer d.safMu.Unlock()
		log.Print("Setting start after finish to true")
		d.startAfterFinish = true
	}
}

func (d *DelayedTaskChan) startDelayedCall() {
	log.Print("Starting delayed task")
	d.status = Delaying

	go func() {
		duration, _ := time.ParseDuration("5s")
		ticker := time.NewTicker(duration)

		func() {
			for {
				select {
				case <-ticker.C:
					return
				case <-d.delayC:
					ticker.Reset(duration)
				}
			}
		}()
		d.status = Running

		log.Print("Starting to run the delayed task")
		d.Task(context.Background())
		log.Print("Finished to run the delayed task")
		d.status = Stopped

		if d.startAfterFinish {
			d.startAfterFinish = false
			log.Print("Restart delayed task")
			d.startDelayedCall()
		}
	}()
}

func (d *DelayedTaskChan) increaseDelay() {
	log.Print("Increasing delay")
	d.delayC <- true
}
