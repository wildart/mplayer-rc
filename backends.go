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
	"bufio"
	"bytes"
	"os/exec"
	"strings"
)

type backendStrings struct {
	binary     string
	startFlags []string

	matchNeedsParam    string
	matchPlayingOK     []string
	matchPlayingPrefix string
	matchPlayingSuffix string
	matchStartupFail   string
	matchStartupOK     string

	cmdFullscreen  string
	cmdGetProp     string
	cmdLoadfile    string
	cmdNoop        string
	cmdOSD         string
	cmdPause       string
	cmdSeek0       string
	cmdSeek1       string
	cmdSeek2       string
	cmdStop        string
	cmdSubSelect   string
	cmdSwitchAudio string
	cmdSwitchRatio string
	cmdVolume0     string
	cmdVolume1     string

	propAspect     string
	propFilename   string
	propFullscreen string
	propLength     string
	propTimePos    string
	propVolume     string
}

var backendMPlayer = backendStrings{
	binary:     "mplayer",
	startFlags: []string{"-idle", "-slave", "-quiet", "-noconsolecontrols"},

	matchNeedsParam:    "Error parsing ",
	matchPlayingOK:     []string{"Starting playback..."},
	matchPlayingPrefix: "Playing ",
	matchPlayingSuffix: ".",
	matchStartupFail:   "Error ",
	matchStartupOK:     "MPlayer",

	cmdFullscreen:  "pausing_keep_force vo_fullscreen",
	cmdGetProp:     "pausing_keep_force get_property %s #%s",
	cmdLoadfile:    "loadfile %s",
	cmdNoop:        "mute 0",
	cmdOSD:         "pausing_keep_force osd",
	cmdPause:       "pause",
	cmdSeek0:       "pausing_keep_force seek %d 0",
	cmdSeek1:       "pausing_keep_force seek %d 1",
	cmdSeek2:       "pausing_keep_force seek %d 2",
	cmdStop:        "stop",
	cmdSubSelect:   "pausing_keep_force sub_select",
	cmdSwitchAudio: "pausing_keep_force switch_audio",
	cmdSwitchRatio: "pausing_keep_force switch_ratio %s",
	cmdVolume0:     "pausing_keep_force volume %d 0",
	cmdVolume1:     "pausing_keep_force volume %d 1",

	propAspect:     "aspect",
	propFilename:   "filename",
	propFullscreen: "fullscreen",
	propLength:     "length",
	propTimePos:    "time_pos",
	propVolume:     "volume",
}

var backendMPV = backendStrings{
	binary: "mpv",
	startFlags: []string{
		"--idle", "--input-file=/dev/stdin", "--quiet", "--input-terminal=no"},

	matchNeedsParam:    "Error parsing ",
	matchPlayingOK:     []string{"[stream] ", " (+)"},
	matchPlayingPrefix: "Playing: ",
	matchPlayingSuffix: "",
	matchStartupFail:   "Error ",
	matchStartupOK:     "[input",

	cmdFullscreen:  "cycle fullscreen",
	cmdGetProp:     "", // set by init function
	cmdLoadfile:    "loadfile %s",
	cmdNoop:        "ignore",
	cmdOSD:         "osd",
	cmdPause:       "cycle pause",
	cmdSeek0:       "seek %d relative",
	cmdSeek1:       "seek %d absolute-percent",
	cmdSeek2:       "seek %d absolute",
	cmdStop:        "stop",
	cmdSubSelect:   "cycle sid",
	cmdSwitchAudio: "cycle aid",
	cmdSwitchRatio: "set video-aspect %s",
	cmdVolume0:     "add volume %d",
	cmdVolume1:     "set volume %d",

	propAspect:     "video-aspect",
	propFilename:   "filename",
	propFullscreen: "fullscreen",
	propLength:     "", // set by init function
	propTimePos:    "time-pos",
	propVolume:     "volume",
}

// init sets a few strings in backendMPV that vary by MPV version. We
// query the MPV that is installed to determine which to use.
func init() {
	printText := "print_text"
	length := "length"
	defer func() {
		backendMPV.cmdGetProp = printText + " ANS_%s=${%s}"
		backendMPV.propLength = length
	}()
	cmd := exec.Command(backendMPV.binary, "--input-cmdlist")
	out := new(bytes.Buffer)
	cmd.Stdout = out
	err := cmd.Run()
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "print-text ") {
			printText = "print-text"
		}
	}
	cmd = exec.Command(backendMPV.binary, "--list-properties")
	out = new(bytes.Buffer)
	cmd.Stdout = out
	err = cmd.Run()
	if err != nil {
		return
	}
	scanner = bufio.NewScanner(out)
	for scanner.Scan() {
		if scanner.Text() == " duration" {
			length = "duration"
		}
	}
}
