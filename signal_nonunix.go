// +build !darwin,!dragonfly,!freebsd,!linux,!nacl,!netbsd,!openbsd,!solaris

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

func startSignalHandler(commandChan chan<- interface{}) {
	// currently a noop on non-unix
}
