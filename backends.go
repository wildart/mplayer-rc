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
	"io"
	"os/exec"
	"strconv"
	"strings"
)

type backendData struct {
	binary     string
	startFlags []string
	volumeMax  int

	matchNeedsParam    string
	matchPlayingOK     []string
	matchPlayingPrefix string
	matchPlayingSuffix string
	matchStartupFail   string
	matchStartupOK     string
	matchCmdPrev       string
	matchCmdNext       string

	cmdFullscreen  string
	cmdGetProp     string
	cmdLoadfile    string
	cmdNoop        string
	cmdOSD         string
	cmdPause       string
	cmdSeekAbs     string
	cmdSeekPct     string
	cmdSeekRel     string
	cmdStop        string
	cmdSubSelect   string
	cmdSwitchAudio string
	cmdSwitchRatio string
	cmdVolumeAbs   string
	cmdVolumeRel   string

	propAspect     string
	propFilename   string
	propFullscreen string
	propLength     string
	propTimePos    string
	propVolume     string
}

var backendMPlayer = backendData{
	binary:     "mplayer",
	startFlags: []string{"-idle", "-slave", "-quiet", "-noconsolecontrols"},
	volumeMax:  100,

	matchNeedsParam:    "Error parsing ",
	matchPlayingOK:     []string{"Starting playback..."},
	matchPlayingPrefix: "Playing ",
	matchPlayingSuffix: ".",
	matchStartupFail:   "Error ",
	matchStartupOK:     "MPlayer",
	matchCmdPrev:       "ANS_stream_start=",
	matchCmdNext:       "ANS_stream_end=",

	cmdFullscreen:  "pausing_keep_force vo_fullscreen",
	cmdGetProp:     "pausing_keep_force get_property %s #%s",
	cmdLoadfile:    "loadfile %s",
	cmdNoop:        "mute 0",
	cmdOSD:         "pausing_keep_force osd",
	cmdPause:       "pause",
	cmdSeekAbs:     "pausing_keep_force seek %d 2",
	cmdSeekPct:     "pausing_keep_force seek %d 1",
	cmdSeekRel:     "pausing_keep_force seek %d 0",
	cmdStop:        "stop",
	cmdSubSelect:   "pausing_keep_force sub_select",
	cmdSwitchAudio: "pausing_keep_force switch_audio",
	cmdSwitchRatio: "pausing_keep_force switch_ratio %s",
	cmdVolumeAbs:   "pausing_keep_force volume %d 1",
	cmdVolumeRel:   "pausing_keep_force volume %d 0",

	propAspect:     "aspect",
	propFilename:   "filename",
	propFullscreen: "fullscreen",
	propLength:     "length",
	propTimePos:    "time_pos",
	propVolume:     "volume",
}

var backendMPV = backendData{
	binary: "mpv",
	startFlags: []string{
		"--idle", "--input-file=/dev/stdin", "--quiet", "--input-terminal=no"},
	volumeMax: 0, // set by init function

	matchNeedsParam:    "Error parsing ",
	matchPlayingOK:     []string{"[stream] ", " (+)"},
	matchPlayingPrefix: "Playing: ",
	matchPlayingSuffix: "",
	matchStartupFail:   "Error ",
	matchStartupOK:     "[input",
	matchCmdPrev:       "Backend: cmdPrev",
	matchCmdNext:       "Backend: cmdNext",

	cmdFullscreen:  "cycle fullscreen",
	cmdGetProp:     "", // set by init function
	cmdLoadfile:    "loadfile %s",
	cmdNoop:        "ignore",
	cmdOSD:         "osd",
	cmdPause:       "cycle pause",
	cmdSeekAbs:     "seek %d absolute",
	cmdSeekPct:     "seek %d absolute-percent",
	cmdSeekRel:     "seek %d relative",
	cmdStop:        "stop",
	cmdSubSelect:   "cycle sid",
	cmdSwitchAudio: "cycle aid",
	cmdSwitchRatio: "set video-aspect %s",
	cmdVolumeAbs:   "set volume %d",
	cmdVolumeRel:   "add volume %d",

	propAspect:     "video-aspect",
	propFilename:   "filename",
	propFullscreen: "fullscreen",
	propLength:     "", // set by init function
	propTimePos:    "time-pos",
	propVolume:     "volume",
}

// init sets a few fields in backendMPV that vary by MPV version. We
// query the MPV that is installed to determine which to use.
func init() {
	printText := "print_text"
	length := "length"
	volumeMax := 100
	defer func() {
		backendMPV.cmdGetProp = printText + " ANS_%s=${%s}"
		backendMPV.propLength = length
		backendMPV.volumeMax = volumeMax
	}()
	runMPV := func(in io.Reader, flags ...string) *bufio.Scanner {
		cmd := exec.Command(backendMPV.binary, flags...)
		out := new(bytes.Buffer)
		cmd.Stdin = in
		cmd.Stdout = out
		_ = cmd.Run()
		return bufio.NewScanner(out)
	}
	// return if no mpv binary
	_, err := exec.LookPath("mpv")
	if err != nil {
		return
	}
	// determine printText
	scanner := runMPV(nil, "--input-cmdlist")
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "print-text ") {
			printText = "print-text"
		}
	}
	// determine length
	scanner = runMPV(nil, "--list-properties")
	for scanner.Scan() {
		if scanner.Text() == " duration" {
			length = "duration"
		}
	}
	// determine volumeMax
	in := strings.NewReader(
		printText + " SOFTVOLMAX_${options/softvol-max}\nquit\n")
	flags := append([]string{"--volume=101"}, backendMPV.startFlags...)
	scanner = runMPV(in, flags...)
	for scanner.Scan() {
		// Note that --volume=101 purposely causes MPV < 0.10.x to
		// print an error and not the value of softvol-max. Hence for
		// MPV < 0.10.x, volumeMax remains at 100 as it should.
		if strings.HasPrefix(scanner.Text(), "SOFTVOLMAX_") {
			max := scanner.Text()[len("SOFTVOLMAX_"):]
			if f, err := strconv.ParseFloat(max, 64); err == nil {
				if int(f) > 0 {
					volumeMax = int(f)
				}
			}
		}
	}
}
