package watcher

import (
	"context"
	"log"
	"sync"
	"time"
)

// ReloaderFunc тип функции для перезагрузки конфигов
type ReloaderFunc func() error

// Reloader периодически перезагружает конфигурацию
type Reloader struct {
	interval time.Duration
	sources  []string
	reloadFn ReloaderFunc
	stopCh   chan struct{}
	wg       sync.WaitGroup
	ctx      context.Context
}

// NewReloader создаёт новый Reloader
func NewReloader(interval time.Duration, sources []string, reloadFn ReloaderFunc) *Reloader {
	return &Reloader{
		interval: interval,
		sources:  sources,
		reloadFn: reloadFn,
		stopCh:   make(chan struct{}),
		ctx:      context.Background(),
	}
}

// Start запускает периодическую перезагрузку
func (r *Reloader) Start(ctx context.Context) {
	if ctx != nil {
		r.ctx = ctx
	}
	r.wg.Add(1)
	go r.run()
	log.Printf("Reloader started (interval: %v, sources: %d)", r.interval, len(r.sources))
}

// Stop останавливает перезагрузку
func (r *Reloader) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	log.Println("Reloader stopped")
}

// ReloadNow выполняет немедленную перезагрузку
func (r *Reloader) ReloadNow() error {
	log.Println("Reloader: manual reload triggered")
	return r.reloadFn()
}

func (r *Reloader) run() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Reloader: scheduled reload started")
			if err := r.reloadFn(); err != nil {
				log.Printf("Reloader: reload failed: %v", err)
			} else {
				log.Println("Reloader: scheduled reload completed")
			}
		case <-r.stopCh:
			return
		case <-r.ctx.Done():
			return
		}
	}
}
