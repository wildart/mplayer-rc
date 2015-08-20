// +build ignore

/*
   Copyright 2015 The MPlayer-RC Authors. See the AUTHORS file at the
   top-level directory of this distribution and at
   <https://xi2.org/x/mplayer-rc/m/AUTHORS>.

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

// Generate version.go by executing git show.
//
// The generated code will set version to YYYYMMDD+HHHHHHH where
// YYYYMMDD is the date of the latest commit and HHHHHHH is the 7
// character hash.
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
)

func main() {
	cmd := exec.Command("git", "show", "--pretty=format:%ci %h")
	b, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	words := strings.Split(lines[0], " ")
	version := words[0][0:4] + words[0][5:7] + words[0][8:10] + "+" + words[3]
	versionGo := fmt.Sprintf(
		`// This file was automatically generated using genversion.go.
// Do not edit.

package main

func init() {
    version = "%s"
}
`, version)
	err = ioutil.WriteFile("version.go", []byte(versionGo), 0600)
	if err != nil {
		log.Fatal(err)
	}
}
