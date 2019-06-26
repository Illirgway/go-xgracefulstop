package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type signalCh chan os.Signal
type StopCh chan struct{}

type GS struct {
	stopChQ []StopCh
	srv     *http.Server
	signal  signalCh
	done    StopCh
}

func NewGS(cap int) *GS {
	return &GS{
		// Package signal will not block sending to c: the caller must ensure
		// that c has sufficient buffer space to keep up with the expected
		// signal rate. For a channel used for notification of just one signal value,
		// a buffer of size 1 is sufficient.
		signal:  make(signalCh, 1),
		stopChQ: make([]StopCh, 0, cap),
		done:    make(StopCh),
	}
}

func (gs *GS) Add(ch StopCh) {
	gs.stopChQ = append(gs.stopChQ, ch)
}

func (gs *GS) Server(srv *http.Server) {
	gs.srv = srv
}

func (gs *GS) SetServerAndWatch(srv *http.Server) {
	gs.Server(srv)

	go gs.Watch()
}

func (gs *GS) Break() {
	close(gs.done)
}

// Watch should start/call in its own goroutine
func (gs *GS) Watch() {
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(gs.signal, syscall.SIGINT, syscall.SIGTERM)

	// wait for signal or break
	select {
	case <-gs.signal:
		// TODO synced close gs.done as in http.Server tracked listen socket done channel
		break
	case <-gs.done:
		return
	}

	// fast link
	srv := gs.srv

	if srv != nil {
		log.Println("Shutdown Server ...")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server Shutdown Error: %s", err.Error())

			if context.DeadlineExceeded == err {
				log.Println("Forces server shutdown...")
				// force close hangs up server connections
				// ATN! "Close does not attempt to close (and does not even know about)
				//       any hijacked connections, such as WebSockets."
				srv.Close()
			}
		}

		log.Println("Server stopped")

		// ATN! in any case freeing context
		cancel()
	}

	for _, ch := range gs.stopChQ {
		// https://dave.cheney.net/2014/03/19/channel-axioms
		// "A receive from a closed channel returns the zero value immediately"
		// need no to send empty value
		close(ch)
	}
}
