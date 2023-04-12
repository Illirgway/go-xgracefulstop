//  Copyright (C) 2019 - 2023 Illirgway
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <https://www.gnu.org/licenses/>.

package xgracefulstop

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

const DefaultTimeout = 5 * time.Second

type signalCh chan os.Signal
type StopCh chan struct{}

type GS struct {
	timeout time.Duration
	stopChQ []StopCh
	srv     *http.Server
	signal  signalCh
	close   StopCh
	started	uint32
	done    StopCh
}

func NewGS(cap int, timeout time.Duration) *GS {
	return &GS{
		timeout: timeout,
		// Package signal will not block sending to c: the caller must ensure
		// that c has sufficient buffer space to keep up with the expected
		// signal rate. For a channel used for notification of just one signal value,
		// a buffer of size 1 is sufficient.
		signal:  make(signalCh, 1),
		stopChQ: make([]StopCh, 0, cap),
		close:   make(StopCh),
		started: 0,
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

	gs.Watch()
}

// @see http.Server.closeDoneChanLocked
func (gs *GS) Break() {
	select {
	case <-gs.close:
		// already closed, pass
	default:
		close(gs.close)
	}
}

func (gs *GS) Wait() {
	// only if already started
	if atomic.LoadUint32(&gs.started) != 0 {
		<-gs.done
	}
}

// Watch should start/call in its own goroutine
func (gs *GS) Watch() {
	// single-shot guard and "started" flag
	if atomic.CompareAndSwapUint32(&gs.started, 0, 1) {
		go gs.watch()
	}
}

func (gs *GS) watch() {
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(gs.signal, syscall.SIGINT, syscall.SIGTERM)

	// wait for signal or break
	select {
	case <-gs.signal:
		// TODO synced close gs.close as in http.Server tracked listen socket done channel
		break
	case <-gs.close:
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

	if (gs.timeout > 0) && (len(gs.stopChQ) > 0) {
		time.Sleep(gs.timeout)
	}

	// send signal "gs is finished"
	close(gs.done)
}