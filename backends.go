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
	matchPlayingOK     string
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
	matchPlayingOK:     "Starting playback...",
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
	matchPlayingOK:     "[stream] ",
	matchPlayingPrefix: "Playing: ",
	matchPlayingSuffix: "",
	matchStartupFail:   "Error ",
	matchStartupOK:     "[input",

	cmdFullscreen:  "cycle fullscreen",
	cmdGetProp:     "", // set by init function
	cmdLoadfile:    "loadfile %s",
	cmdNoop:        "",
	cmdOSD:         "osd",
	cmdPause:       "cycle pause",
	cmdSeek0:       "seek %d relative",
	cmdSeek1:       "seek %d absolute-percent",
	cmdSeek2:       "seek %d absolute",
	cmdStop:        "stop",
	cmdSubSelect:   "cycle sid",
	cmdSwitchAudio: "cycle aid",
	cmdSwitchRatio: "set video-aspect %s",
	cmdVolume0:     "cycle volume %d",
	cmdVolume1:     "set volume %d",

	propAspect:     "video-aspect",
	propFilename:   "filename",
	propFullscreen: "fullscreen",
	propLength:     "", // set by init function
	propTimePos:    "time-pos",
	propVolume:     "volume",
}

// init sets a few strings in backendMPV that can only be determined
// by probing the exact version of MPV that is installed.
func init() {
	printText := "print-text"
	length := "duration"
	defer func() {
		backendMPV.cmdGetProp = printText + " ANS_%s=${%s}"
		backendMPV.propLength = length
	}()
	cmd := exec.Command(backendMPV.binary, backendMPV.startFlags...)
	out := new(bytes.Buffer)
	cmd.Stdin = strings.NewReader(
		"print-text ${duration}\nprint_text ${duration}\nquit\n")
	cmd.Stdout = out
	err := cmd.Run()
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		if strings.HasSuffix(
			scanner.Text(), "Command 'print-text' not found.") {
			printText = "print_text"
		}
		if scanner.Text() == "(error)" {
			length = "length"
		}
	}
}
