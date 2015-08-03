// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

/*
   Copyright 2015 The MPlayer-RC Authors. See the AUTHORS file at the
   top-level directory of this distribution and at
   <https://xi2.org/x/mplayer-rc/AUTHORS>.

   This file is part of MPlayer-RC.

   MPlayer-RC is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published
   by the Free Software Foundation, either version 3 of the License,
   or (at your option) any later version.

   MPlayer-RC is distributed in the hope that it will be useful, but
   WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
   General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with MPlayer-RC.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

func startSignalHandler(commandChan chan interface{}) {
	sigChan := make(chan os.Signal, 100)
	signal.Notify(sigChan, unix.SIGCHLD, unix.SIGUSR1, unix.SIGUSR2)
	go func() {
		for sig := range sigChan {
			switch sig {
			case unix.SIGCHLD:
				os.Exit(0)
			case unix.SIGUSR1:
				commandChan <- cmdPrev{}
			case unix.SIGUSR2:
				commandChan <- cmdNext{}
			}
		}
	}()
}
