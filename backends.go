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

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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
	cmdQuit        string

	propAspect     string
	propFilename   string
	propFullscreen string
	propLength     string
	propTimePos    string
	propVolume     string
}

// MPlayer backend

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
	cmdNoop:        "pausing_keep_force loop -1",
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
	cmdQuit:        "quit",

	propAspect:     "aspect",
	propFilename:   "filename",
	propFullscreen: "fullscreen",
	propLength:     "length",
	propTimePos:    "time_pos",
	propVolume:     "volume",
}

// MPV backend

var backendMPV = backendData{
	binary:     "mpv",
	startFlags: mpvStartFlags,
	volumeMax:  mpvVolumeMax,

	matchNeedsParam:    "Error parsing ",
	matchPlayingOK:     []string{"[stream] ", " (+)"},
	matchPlayingPrefix: "Playing: ",
	matchPlayingSuffix: "",
	matchStartupFail:   "Error ",
	matchStartupOK:     "[input",
	matchCmdPrev:       "Backend: cmdPrev",
	matchCmdNext:       "Backend: cmdNext",

	cmdFullscreen:  "cycle fullscreen",
	cmdGetProp:     mpvCmdGetProp,
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
	cmdSwitchRatio: mpvCmdSwitchRatio,
	cmdVolumeAbs:   "set volume %d",
	cmdVolumeRel:   "add volume %d",
	cmdQuit:        "quit",

	propAspect:     mpvPropAspect,
	propFilename:   "filename",
	propFullscreen: "fullscreen",
	propLength:     mpvPropLength,
	propTimePos:    "time-pos",
	propVolume:     "volume",
}

// MPV backend helpers

func runMPV(in io.Reader, flags ...string) (*bufio.Scanner, error) {
	cmd := exec.Command("mpv", flags...)
	out := new(bytes.Buffer)
	cmd.Stdin = in
	cmd.Stdout = out
	err := cmd.Run()
	return bufio.NewScanner(out), err
}

func mpvFlags() map[string]struct{} {
	flags := map[string]struct{}{}
	scanner, err := runMPV(nil, "--list-options")
	if err != nil {
		return flags
	}
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), " -") {
			flags[strings.Split(scanner.Text(), " ")[1]] = struct{}{}
		}
	}
	return flags
}

func mpvProperties() map[string]struct{} {
	properties := map[string]struct{}{}
	scanner, err := runMPV(nil, "--list-properties")
	if err != nil {
		return properties
	}
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), " ") {
			properties[strings.Split(scanner.Text(), " ")[1]] = struct{}{}
		}
	}
	return properties
}

func mpvInputCmds() map[string]struct{} {
	inputCmds := map[string]struct{}{}
	scanner, err := runMPV(nil, "--input-cmdlist")
	if err != nil {
		return inputCmds
	}
	for scanner.Scan() {
		if scanner.Text() != "" {
			if scanner.Text()[0] >= 'a' && scanner.Text()[0] <= 'z' {
				inputCmds[strings.Split(scanner.Text(), " ")[0]] = struct{}{}
			}
		}
	}
	return inputCmds
}

var mpvData = func() struct {
	flags      map[string]struct{}
	properties map[string]struct{}
	inputCmds  map[string]struct{}
} {
	var data struct {
		flags      map[string]struct{}
		properties map[string]struct{}
		inputCmds  map[string]struct{}
	}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		data.flags = mpvFlags()
		wg.Done()
	}()
	go func() {
		data.properties = mpvProperties()
		wg.Done()
	}()
	go func() {
		data.inputCmds = mpvInputCmds()
		wg.Done()
	}()
	wg.Wait()
	return data
}()

// MPV backend computed fields

var mpvStartFlags = func() []string {
	startFlags := []string{
		"--idle", "--input-file=/dev/stdin", "--quiet",
		"--consolecontrols=no"}
	if _, ok := mpvData.flags["--input-console"]; ok {
		flags := startFlags[:len(startFlags)-1]
		startFlags = append(flags, "--input-console=no")
	}
	if _, ok := mpvData.flags["--input-terminal"]; ok {
		flags := startFlags[:len(startFlags)-1]
		startFlags = append(flags, "--input-terminal=no")
	}
	return startFlags
}()

var mpvVolumeMax = func() int {
	volumeMax := 100
	in := strings.NewReader(fmt.Sprintf(
		mpvCmdGetProp+"\nquit\n",
		"options/softvol-max", "options/softvol-max"))
	startFlags := append([]string{"--volume=101"}, mpvStartFlags...)
	scanner, err := runMPV(in, startFlags...)
	if err != nil {
		return volumeMax
	}
	for scanner.Scan() {
		// Note that --volume=101 purposely causes MPV < 0.10.x to
		// print an error and not the value of softvol-max. Hence for
		// MPV < 0.10.x, volumeMax remains at 100 as it should.
		if strings.HasPrefix(scanner.Text(), "ANS_options/softvol-max=") {
			max := scanner.Text()[len("ANS_options/softvol-max="):]
			if f, err := strconv.ParseFloat(max, 64); err == nil {
				if int(f) > 0 {
					volumeMax = int(f)
				}
			}
		}
	}
	return volumeMax
}()

var mpvCmdGetProp = func() string {
	cmdGetProp := "print_text ANS_%s=${%s}"
	if _, ok := mpvData.inputCmds["print-text"]; ok {
		cmdGetProp = "print-text ANS_%s=${%s}"
	}
	return cmdGetProp
}()

var mpvCmdSwitchRatio = "set " + mpvPropAspect + " %s"

var mpvPropAspect = func() string {
	propAspect := "aspect"
	if _, ok := mpvData.properties["video-aspect"]; ok {
		propAspect = "video-aspect"
	}
	return propAspect
}()

var mpvPropLength = func() string {
	propLength := "length"
	if _, ok := mpvData.properties["duration"]; ok {
		propLength = "duration"
	}
	return propLength
}()
