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

// API information
//
// mplayer: http://www.mplayerhq.hu/DOCS/tech/slave.txt
//
//     vlc: https://wiki.videolan.org/VLC_HTTP_requests/
//          https://wiki.videolan.org/Documentation:Modules/http_intf/
//          https://raw.githubusercontent.com/videolan/vlc/master/share/lua/http/requests/README.txt

package main // import "xi2.org/x/mplayer-rc"

//go:generate genman
//go:generate go run genversion.go

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// version, the program version string, can be left blank or modified
// (in which case it is output on the license screen). go generate
// will create a version.go file from the output of "git show" which
// will set version via an init function to the current commit.
var version = ""

const license = `   Copyright 2015 The MPlayer-RC Authors. See the AUTHORS file at the
   top-level directory of this distribution and at
   <https://xi2.org/x/mplayer-rc/AUTHORS>.

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

   For further information about MPlayer-RC visit
   <https://xi2.org/x/mplayer-rc>.
`

var (
	flagUsage bool

	flagVersion       bool
	flagRemapCommands bool
	flagPassword      string
	flagPort          string
	flagMPlayerUsage  bool
)

// isParameterFlag returns true if flag is an MPlayer flag requiring a
// parameter, and that parameter is not provided following an "=".
//
// Examples:
//   -vf => true
//   --vf=pp => false
func isParameterFlag(flag string) bool {
	out, _ := exec.Command("mplayer", flag).CombinedOutput()
	scanner := bufio.NewScanner(bytes.NewBuffer(out))
	if !scanner.Scan() {
		return false
	}
	line1 := scanner.Text()
	if strings.HasSuffix(line1, "Required parameter for option missing") {
		// mplayer2
		return true
	}
	if strings.HasPrefix(line1, "Error parsing option on the command line:") {
		// mplayer
		return true
	}
	return false
}

// processFlags processes os.Args and creates the playlist state from
// the relevant command line arguments. It returns a list of flags to
// be passed to mplayer.
//
// The mplayer-rc specific flags are handled as appropriate.
//
// The mplayer flags --playlist/-playlist and --shuffle/-shuffle are
// handled by mplayer-rc and are not passed to mplayer.
func processFlags() []string {
	n := len(os.Args)

	printUsage := func() {
		fmt.Fprintf(os.Stderr,
			"usage: %s [mplayer-rc or mplayer options] [files/URLs]\n",
			filepath.Base(os.Args[0]))
		// Go 1.5+ package flag compatible format
		fmt.Fprintf(os.Stderr, "  -V\t")
		fmt.Fprintf(os.Stderr,
			"show version, license and further information\n")
		fmt.Fprintf(os.Stderr, "  --remap-commands\n")
		fmt.Fprintf(os.Stderr,
			"    \tuse alternate actions for some VLC commands\n")
		fmt.Fprintf(os.Stderr, "  --rc-password pass\n")
		fmt.Fprintf(os.Stderr,
			"    \tuse pass as the Android-VLC-Remote password\n")
		fmt.Fprintf(os.Stderr, "  --rc-port port\n")
		fmt.Fprintf(os.Stderr,
			"    \tuse port as the listening port for VLC commands (default 8080)\n")
		fmt.Fprintf(os.Stderr, "  --mplayer-help\n")
		fmt.Fprintf(os.Stderr,
			"    \tdisplay the MPlayer usage message\n")
	}
	printVersion := func() {
		if version != "" {
			fmt.Fprintf(os.Stderr, "   MPlayer-RC version %s\n\n", version)
		}
		fmt.Fprintf(os.Stderr, license)
	}
	printMPlayerUsage := func() {
		out, _ := exec.Command("mplayer", "--help").CombinedOutput()
		fmt.Fprint(os.Stderr, string(out))
	}

	// process flags
	doShuffle := false
	var flags, tracks []string
	for i := 1; i < n; i++ {
		a := os.Args[i]
		if a == "--" {
			tracks = append(tracks, os.Args[i+1:]...)
			break
		}
		if len(a) > 0 && a[0] != '-' {
			tracks = append(tracks, a)
			continue
		}
		if a == "--remap-commands" || a == "-remap-commands" {
			flagRemapCommands = true
			continue
		}
		if strings.HasPrefix(a, "--remap-commands=") {
			p := a[len("--remap-commands="):]
			switch p {
			case "1", "t", "T", "true", "TRUE", "True",
				"y", "Y", "yes", "YES", "Yes":
				flagRemapCommands = true
			}
			continue
		}
		if strings.HasPrefix(a, "--rc-password=") {
			flagPassword = a[len("--rc-password="):]
			continue
		}
		if i < n-1 && (a == "--rc-password" || a == "-rc-password") {
			flagPassword = os.Args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(a, "--rc-port=") {
			flagPort = a[len("--rc-port="):]
			continue
		}
		if i < n-1 && (a == "--rc-port" || a == "-rc-port") {
			flagPort = os.Args[i+1]
			i++
			continue
		}
		if a == "--shuffle" || a == "-shuffle" {
			doShuffle = true
			continue
		}
		if a == "-h" || a == "--help" || a == "-help" {
			flagUsage = true
			break
		}
		if a == "-V" {
			flagVersion = true
			break
		}
		if a == "--mplayer-help" || a == "-mplayer-help" {
			flagMPlayerUsage = true
			break
		}
		isPlaylist := false
		playlist := ""
		if strings.HasPrefix(a, "--playlist=") {
			playlist = a[len("--playlist="):]
			isPlaylist = true
		}
		if i < n-1 && (a == "--playlist" || a == "-playlist") {
			playlist = os.Args[i+1]
			isPlaylist = true
			i++
		}
		if isPlaylist {
			pl, err := ioutil.ReadFile(playlist)
			if err != nil {
				log.Fatal(err)
			}
			// only .m3u8 files are supported at present
			for _, s := range []struct {
				header string
				msg    string
			}{{
				header: "[playlist]",
				msg:    "PLS format playlists not yet supported",
			}, {
				header: "<asx ",
				msg:    "ASX format playlists not yet supported",
			}, {
				header: "<smil ",
				msg:    "SMIL format playlists not yet supported",
			}} {
				if len(pl) >= len(s.header) &&
					strings.ToLower(string(pl[:len(s.header)])) == s.header {
					log.Fatalf("mplayer-rc: %s\n", s.msg)
				}
			}
			scanner := bufio.NewScanner(bytes.NewBuffer(pl))
			tracks = []string{}
			for scanner.Scan() {
				if scanner.Text() != "" {
					if scanner.Text()[0] != '#' {
						tracks = append(tracks, scanner.Text())
					}
				}
			}
			continue
		}
		if i < n-1 && isParameterFlag(a) {
			flags = append(flags, a, os.Args[i+1])
			i++
			continue
		}
		flags = append(flags, a)
	}

	// handle mplayer-rc flags
	if flagVersion {
		printVersion()
		os.Exit(1)
	}
	if flagMPlayerUsage {
		printMPlayerUsage()
		os.Exit(1)
	}
	if flagUsage || len(tracks) == 0 {
		printUsage()
		os.Exit(2)
	}

	// create playlist state
	for _, f := range tracks {
		addPlaylistEntry(f)
	}
	if doShuffle == true {
		playpos = rand.Intn(len(playlist))
		funcShuffle()
	}
	return flags
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

var (
	// the playlist state
	idTrackMap = map[int]string{} // track id -> track (file/url)
	idPosMap   = map[int]int{}    // track id -> playlist pos
	playlist   []int              // playlist pos -> track id
	playpos    int                // current playlist pos
	// the shuffle state used by Next/Prev
	posToShuf []int // pos -> shufpos
	shufToPos []int // shufpos -> pos
	shuffle   bool
	// the loop/repeat state. loop is for playlist looping and repeat
	// is for track looping. They are never both true at once.
	loop   bool
	repeat bool
	// the stopped state. We need this since mplayer can briefly
	// transition into stopped state inbetween tracks when we are not
	// really stopped.
	stopped bool
	// whether we are using alternate actions
	remapCommands bool
)

// idCounter is incremented on each creation of a playlist id. id
// 1,2,3 are used in the playlist template, so we start at 4.
var idCounter int = 4

// addPlaylistEntry adds a track to the end of the playlist, taking
// care to update the playlist and shuffle state correctly.
func addPlaylistEntry(track string) {
	playlist = append(playlist, idCounter)
	idTrackMap[idCounter] = track
	idPosMap[idCounter] = len(playlist) - 1
	posToShuf = append(posToShuf, len(playlist)-1)
	shufToPos = append(shufToPos, len(playlist)-1)
	idCounter++
}

// launchMPlayer starts up mplayer with the provided flags in slave
// mode as a background process. It returns mplayer's stdin as an
// io.Writer and combined stdout/stderr as a <-chan string.
func launchMPlayer(flags []string) (io.Writer, <-chan string) {
	flags = append(
		[]string{"-idle", "-slave", "-quiet", "-noconsolecontrols"},
		flags...)
	cmd := exec.Command("mplayer", flags...)
	in, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	out, w := io.Pipe()
	cmd.Stdout = w
	cmd.Stderr = w
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	outChan := make(chan string, 1000)
	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			outChan <- scanner.Text()
		}
	}()
	// check for command line errors at mplayer startup
	for line := range outChan {
		if strings.HasPrefix(line, "Error ") {
			// mplayer has failed to parse it's command line or has
			// otherwise failed to start
			log.Fatal(errors.New("mplayer: " + line))
		}
		if strings.HasPrefix(line, "MPlayer") {
			// all good hopefully...
			break
		}
	}
	return in, outChan
}

// escapeTrack escapes a file name/url so it is suitable to pass
// to mplayer with "loadfile"
func escapeTrack(track string) string {
	track = strings.Replace(track, "\\", "\\\\", -1)
	track = strings.Replace(track, " ", "\\ ", -1)
	track = strings.Replace(track, "#", "\\#", -1)
	return track
}

// "select loop" commands and their associated command functions.
// each cmdXXX has a corresponding funcXXX.

type cmdPlay struct {
	id int // track id
}
type cmdNext struct{}
type cmdPrev struct{}
type cmdPause struct{} // a toggle
type cmdStop struct{}
type cmdShuffle struct{} // a toggle
type cmdLoop struct{}    // a toggle
type cmdRepeat struct{}  // a toggle
type cmdAspect struct{}
type cmdAudio struct{}
type cmdSubtitle struct{}
type cmdFullscreen struct{} // a toggle
type cmdVolume struct {
	val  int // volume (0 -> 100 in absolute mode)
	mode int // 0 relative, 1 absolute
}
type cmdSeek struct {
	val  int // time in seconds (can be positive or negative value)
	mode int // 0 relative, 1 percent, 2 absolute
}
type cmdGetProp struct {
	prop      string
	replyChan chan<- string
}
type cmdGetPlaylistXML struct {
	replyChan chan<- string
}

// funcPlay plays the track given by id or plays the current playlist
// entry if id is invalid. By convention -1 is the invalid id used to
// mean play the current playlist entry.
//
// if id is valid then the current playlist position is updated to
// that of id's.
//
// funcPlay will do nothing if the playlist is empty.
//
// funcPlay will do nothing other than update the playlist position if
// it cannot find a playable track (even by repeated calls to
// funcNext).
func funcPlay(in io.Writer, outChan <-chan string, id int) {
	if len(playlist) == 0 {
		return
	}
	if _, ok := idTrackMap[id]; !ok {
		id = playlist[playpos]
	} else {
		playpos = idPosMap[id]
	}
	// if MPlayer could not play the previous track it will ignore the
	// next command so, in case this is true, send it an arbitrary
	// command first.
	io.WriteString(in, "mute 0\n")
	io.WriteString(in, "loadfile "+escapeTrack(idTrackMap[id])+"\n")
	var playing bool
	var playingTrack string
	for line := range outChan {
		if strings.HasPrefix(line, "Playing ") && len(line) > len("Playing ") {
			// len(line)-1 is to account for full stop
			playingTrack = line[len("Playing ") : len(line)-1]
			playing = true
		}
		if line == "" && playing {
			log.Println("mplayer: cannot play track: " + playingTrack)
			funcNext(in, outChan)
			return
		}
		if line == "Starting playback..." {
			// valid track found
			stopped = false
			return
		}
	}
}

// funcNext will try to play the next track. This includes playing the
// current track again if repeat is true.
//
// It does this by updating the playlist position and then calling
// funcPlay.
//
// funcNext will do nothing if the playlist is empty.
//
// If it is at the end of the playlist and loop is false, then if
// funcNext has been called as a result of a track finishing then it
// will set playpos to 0 and stopped to true, otherwise it will do
// nothing.
func funcNext(in io.Writer, outChan <-chan string) {
	if len(playlist) == 0 {
		return
	}
	if repeat {
		funcPlay(in, outChan, -1)
	} else {
		if posToShuf[playpos] != len(playlist)-1 || loop {
			shufpos := posToShuf[playpos]
			if shufpos == len(playlist)-1 {
				shufpos = 0
			} else {
				shufpos++
			}
			playpos = shufToPos[shufpos]
			funcPlay(in, outChan, -1)
		} else {
			if !stopped && funcGetProp(in, outChan, "state") == "stopped" {
				stopped = true
				playpos = 0
			}
		}
	}
}

// funcPrev acts like funcNext but will try to play the previous
// track.
func funcPrev(in io.Writer, outChan <-chan string) {
	if len(playlist) == 0 {
		return
	}
	if posToShuf[playpos] != 0 || loop {
		shufpos := posToShuf[playpos]
		if shufpos == 0 {
			shufpos = len(playlist) - 1
		} else {
			shufpos--
		}
		playpos = shufToPos[shufpos]
		funcPlay(in, outChan, -1)
	}
}

func funcPause(in io.Writer, outChan <-chan string) {
	if stopped {
		funcPlay(in, outChan, -1)
		return
	}
	io.WriteString(in, "pause\n")
}

func funcStop(in io.Writer, outChan <-chan string) {
	if !stopped {
		if funcGetProp(in, outChan, "state") == "paused" {
			funcPause(in, outChan) // un-pause before stop
		}
		io.WriteString(in, "stop\n")
		// wait for mplayer to confirm stop before updating the
		// stopped state
		ticker := time.NewTicker(250 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				if funcGetProp(in, outChan, "state") == "stopped" {
					ticker.Stop()
					stopped = true
					return
				}
			}
		}
	}
}

// funcShuffle toggles shuffling on/off and recreates the shuffle state
// accordingly.
func funcShuffle() {
	if shuffle {
		shuffle = false
		for i := range playlist {
			posToShuf[i] = i
			shufToPos[i] = i
		}
		return
	}
	shuffle = true
	// the set of positions to be shuffled. Position zero is not
	// included since it will be used for the current track.
	shufSet := make([]int, len(playlist)-1)
	for i := range shufSet {
		shufSet[i] = i + 1
	}
	for i := range playlist {
		if i == playpos {
			// always put current track at top of newly shuffled playlist
			posToShuf[i] = 0
			shufToPos[0] = i
		} else {
			j := rand.Intn(len(shufSet))
			// remove a random element from shufSet and put track i at
			// this position
			posToShuf[i] = shufSet[j]
			shufToPos[shufSet[j]] = i
			shufSet = append(shufSet[0:j], shufSet[j+1:]...)
		}
	}
}

func funcLoop() {
	loop = !loop
	repeat = false
}

func funcRepeat() {
	repeat = !repeat
	loop = false
}

func funcAspect(in io.Writer, outChan <-chan string) {
	if remapCommands {
		// repurpose to fast forward by 10 seconds
		funcSeek(in, +10, 0)
	} else {
		if f, err := strconv.ParseFloat(
			funcGetProp(in, outChan, "aspect"), 64); err == nil {
			// flip between 16:9 and 4:3
			if f < 1.5555 {
				io.WriteString(in, "pausing_keep_force switch_ratio 1.7777\n")
			} else {
				io.WriteString(in, "pausing_keep_force switch_ratio 1.3333\n")
			}
		}
	}
}

func funcAudio(in io.Writer) {
	if remapCommands {
		// repurpose this as OSD toggle
		io.WriteString(in, "pausing_keep_force osd\n")
	} else {
		io.WriteString(in, "pausing_keep_force switch_audio\n")
	}
}
func funcSubtitle(in io.Writer) {
	if remapCommands {
		// repurpose to rewind by 10 seconds
		funcSeek(in, -10, 0)
	} else {
		io.WriteString(in, "pausing_keep_force sub_select\n")
	}
}

func funcFullscreen(in io.Writer) {
	io.WriteString(in, "pausing_keep_force vo_fullscreen\n")
}

func funcVolume(in io.Writer, val, mode int) {
	io.WriteString(in,
		"pausing_keep_force volume "+
			strconv.Itoa(val)+" "+strconv.Itoa(mode)+"\n")
}

func funcSeek(in io.Writer, val, mode int) {
	io.WriteString(in,
		"pausing_keep_force seek "+
			strconv.Itoa(val)+" "+strconv.Itoa(mode)+"\n")
}

// funcGetProp gets a property value. It also handles the
// pseudo-property, "state".
func funcGetProp(in io.Writer, outChan <-chan string, prop string) string {
	if prop == "state" {
		// first deal with the pseudo-property, "state"
		trackname := funcGetProp(in, outChan, "filename")
		if trackname == "ANS_ERROR=PROPERTY_UNAVAILABLE" {
			return "stopped"
		}
		if funcGetProp(in, outChan, "pause") == "yes" {
			return "paused"
		}
		return "playing"
	}
	// now deal with real properties.
	io.WriteString(in, "pausing_keep_force get_property "+prop+"\n")
	var ans string
	for line := range outChan {
		if strings.HasPrefix(line, "ANS_ERROR=") {
			ans = line
			break
		}
		if strings.HasPrefix(line, "ANS_"+prop+"=") {
			ans = line[len("ANS_"+prop+"="):]
			break
		}
	}
	return ans
}

// playlist.xml

const playlistTmplTxt = `
<node ro="rw" name="Undefined" id="1">
<node ro="ro" name="Playlist" id="2">
{{range .}}
<leaf duration="-1" ro="rw" name="{{.Name}}"
 id="{{.Id}}" {{if .Current}}current="current"{{end}}></leaf>
{{end}}
</node>
<node ro="ro" name="Media Library" id="3"></node>
</node>
`

var playlistTmpl = template.Must(
	template.New("playlist").Parse(playlistTmplTxt))

// funcGetPlaylistXML constructs playlist.xml.
func funcGetPlaylistXML() string {
	data := []struct {
		Name    string
		Id      int
		Current bool
	}{}
	for i := range playlist {
		id := playlist[shufToPos[i]]
		name := filepath.Base(idTrackMap[id])
		var current bool
		if id == playlist[playpos] {
			current = true
		}
		data = append(data, struct {
			Name    string
			Id      int
			Current bool
		}{Name: name, Id: id, Current: current})
	}
	buf := new(bytes.Buffer)
	buf.WriteString(`<?xml version="1.0" encoding="utf-8" standalone="yes" ?>`)
	err := playlistTmpl.Execute(buf, data)
	if err != nil {
		log.Fatal(err)
	}
	return buf.String()
}

// startSelectLoop returns a command channel whose purpose is to
// serialize the execution of commands sent to mplayer. In a goroutine
// it uses select to wait on either a command over the command
// channel, output from mplayer (which is discarded) or a ticker
// firing (which causes it to check whether the current track has
// stopped playing). It also sets up a channel to receive SIGCHILDs
// from mplayer and ensures the program exits when receiving one.
func startSelectLoop(in io.Writer, outChan <-chan string) chan<- interface{} {
	commandChan := make(chan interface{})
	sigChan := make(chan os.Signal, 1)
	ticker := time.NewTicker(250 * time.Millisecond)
	setupSIGCHLD(sigChan)
	go func() {
		<-sigChan
		os.Exit(0)
	}()
	go func() {
		for {
			select {
			case cmdIn := <-commandChan:
				switch cmd := cmdIn.(type) {
				case cmdPlay:
					funcPlay(in, outChan, cmd.id)
				case cmdNext:
					funcNext(in, outChan)
				case cmdPrev:
					funcPrev(in, outChan)
				case cmdPause:
					funcPause(in, outChan)
				case cmdStop:
					funcStop(in, outChan)
				case cmdShuffle:
					funcShuffle()
				case cmdLoop:
					funcLoop()
				case cmdRepeat:
					funcRepeat()
				case cmdAspect:
					funcAspect(in, outChan)
				case cmdAudio:
					funcAudio(in)
				case cmdSubtitle:
					funcSubtitle(in)
				case cmdFullscreen:
					funcFullscreen(in)
				case cmdVolume:
					funcVolume(in, cmd.val, cmd.mode)
				case cmdSeek:
					funcSeek(in, cmd.val, cmd.mode)
				case cmdGetProp:
					cmd.replyChan <- funcGetProp(in, outChan, cmd.prop)
				case cmdGetPlaylistXML:
					cmd.replyChan <- funcGetPlaylistXML()
				}
			case <-outChan:
				// discard unused output from mplayer
			case <-ticker.C:
				if !stopped && funcGetProp(in, outChan, "state") == "stopped" {
					funcNext(in, outChan)
				}
			}
		}
	}()
	return commandChan
}

// status.xml

const statusTmplTxt = `
<root>

<fullscreen>{{.Fullscreen}}</fullscreen>
<volume>{{.Volume}}</volume>
<loop>{{.Loop}}</loop>
<random>{{.Random}}</random>
<length>{{.Length}}</length>
<repeat>{{.Repeat}}</repeat>
<state>{{.State}}</state>
<time>{{.Time}}</time>

<information>
<category name="meta">
<info name='title'>{{.Title}}</info>
<info name='filename'>{{.Filename}}</info>
</category>
</information>

</root>
`

type statusTmplData struct {
	Fullscreen bool
	Volume     int
	Loop       bool
	Random     bool
	Length     int
	Repeat     bool
	State      string
	Time       int
	Title      string
	Filename   string
}

var statusTmpl = template.Must(template.New("status").Parse(statusTmplTxt))

// statusXML constructs status.xml.
func statusXML(commandChan chan<- interface{}) string {
	data := &statusTmplData{}
	replyChan := make(chan string)
	getProp := func(prop string) string {
		commandChan <- cmdGetProp{prop: prop, replyChan: replyChan}
		return <-replyChan
	}
	getFloat := func(prop string) float64 {
		if f, err := strconv.ParseFloat(getProp(prop), 64); err == nil {
			return f
		} else {
			return 0
		}
	}
	getBool := func(prop string) bool {
		if getProp(prop) == "yes" {
			return true
		} else {
			return false
		}
	}
	data.Fullscreen = getBool("fullscreen")
	data.Volume = int(getFloat("volume")) * 320 / 100
	data.Loop = loop
	data.Random = shuffle
	data.Length = int(getFloat("length"))
	data.Repeat = repeat
	data.State = getProp("state")
	data.Time = int(getFloat("time_pos"))
	if data.State != "stopped" {
		filename := getProp("filename")
		data.Title = filename
		data.Filename = filename
	} else {
		data.Title = ""
		data.Filename = ""
	}
	buf := new(bytes.Buffer)
	buf.WriteString(`<?xml version="1.0" encoding="utf-8" standalone="yes" ?>`)
	err := statusTmpl.Execute(buf, data)
	if err != nil {
		log.Fatal(err)
	}
	return buf.String()
}

// the http server

func authorized(
	w http.ResponseWriter, r *http.Request, username, password string) bool {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Basic ") {
		b64 := base64.StdEncoding.EncodeToString(
			[]byte(username + ":" + password))
		if auth[6:] == b64 {
			return true
		}
	}
	w.Header().Add("WWW-Authenticate", "Basic realm=\"authenticate\"")
	w.WriteHeader(401)
	return false
}

func startWebServer(commandChan chan<- interface{}, password, port string) {
	http.HandleFunc(
		"/requests/status.xml", func(w http.ResponseWriter, r *http.Request) {
			if !authorized(w, r, "", password) {
				return
			}
			switch r.FormValue("command") {
			case "pl_play":
				id := -1
				if idStr := r.FormValue("id"); idStr != "" {
					if idVal, err := strconv.Atoi(idStr); err == nil {
						id = idVal
					}
				}
				commandChan <- cmdPlay{id: id}
			case "pl_next":
				commandChan <- cmdNext{}
			case "pl_previous":
				commandChan <- cmdPrev{}
			case "pl_pause":
				commandChan <- cmdPause{}
			case "pl_stop":
				commandChan <- cmdStop{}
			case "pl_random":
				commandChan <- cmdShuffle{}
			case "pl_loop":
				commandChan <- cmdLoop{}
			case "pl_repeat":
				commandChan <- cmdRepeat{}
			case "key":
				switch r.FormValue("val") {
				case "aspect-ratio":
					commandChan <- cmdAspect{}
				case "audio-track":
					commandChan <- cmdAudio{}
				case "subtitle-track":
					commandChan <- cmdSubtitle{}
				}
			case "fullscreen":
				commandChan <- cmdFullscreen{}
			case "volume":
				val := r.FormValue("val")
				var mode, off int
				percent := false
				if len(val) > 0 && val[len(val)-1] == '%' {
					val = val[:len(val)-1]
					percent = true
				}
				if len(val) > 0 {
					switch val[0] {
					case '+', '-', ' ':
						off = 1
					default:
						mode = 1
					}
					if i, err := strconv.Atoi(val[off:]); err == nil {
						if !percent {
							i = i * 100 / 320
						}
						if val[0] == '-' {
							i = -i
						}
						commandChan <- cmdVolume{val: i, mode: mode}
					}
				}
			case "seek":
				val := r.FormValue("val")
				var mode, off int
				if len(val) > 0 && val[len(val)-1] == '%' {
					val = val[:len(val)-1]
					mode = 1
				}
				if len(val) > 0 &&
					(val[len(val)-1] == 's' || val[len(val)-1] == 'S') {
					val = val[:len(val)-1]
				}
				if len(val) > 0 {
					switch val[0] {
					case '+', '-', ' ':
						off = 1
					default:
						if mode == 0 {
							mode = 2
						}
					}
					if i, err := strconv.Atoi(val[off:]); err == nil {
						if val[0] == '-' {
							i = -i
						}
						commandChan <- cmdSeek{val: i, mode: mode}
					}
				}
			default:
				// output status.xml
				io.WriteString(w, statusXML(commandChan))
			}
		})
	http.HandleFunc(
		"/requests/playlist.xml",
		func(w http.ResponseWriter, r *http.Request) {
			if !authorized(w, r, "", password) {
				return
			}
			// output playlist.xml
			replyChan := make(chan string)
			commandChan <- cmdGetPlaylistXML{replyChan: replyChan}
			io.WriteString(w, <-replyChan)
		})
	if http.ListenAndServe(":"+port, nil) != nil {
		log.Fatal(errors.New("failed to start http server"))
	}
}

// main

func trimTrailingSpace(s string) string {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func main() {
	flags := processFlags()
	// set defaults for password/port
	password, port := "", "8080"
	// try to set remapCommands/password/port from config file
	home := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
	}
	b, err := ioutil.ReadFile(
		filepath.Join(home, ".mplayer", "mplayer-rc"))
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewBuffer(b))
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "remap-commands") {
				p := trimTrailingSpace(scanner.Text())
				if p == "remap-commands" {
					remapCommands = true
				}
			}
			if strings.HasPrefix(scanner.Text(), "remap-commands=") {
				p := scanner.Text()[len("remap-commands="):]
				p = trimTrailingSpace(p)
				switch p {
				case "1", "t", "T", "true", "TRUE", "True",
					"y", "Y", "yes", "YES", "Yes":
					remapCommands = true
				}
			}
			if strings.HasPrefix(scanner.Text(), "rc-password=") {
				p := scanner.Text()[len("rc-password="):]
				password = trimTrailingSpace(p)
			}
			if strings.HasPrefix(scanner.Text(), "rc-port=") {
				p := scanner.Text()[len("rc-port="):]
				port = trimTrailingSpace(p)
			}
		}
	}
	// try to set them from flags
	if flagRemapCommands == true {
		remapCommands = true
	}
	if flagPassword != "" {
		password = flagPassword
	}
	if flagPort != "" {
		port = flagPort
	}
	// if password not set, exit
	if password == "" {
		fmt.Fprint(os.Stderr,
			`MPlayer-RC needs to have a password which is used to authorize
Android-VLC-Remote. You can specify the password using the command
line flag --rc-password=<pass> or by putting the line

  rc-password=<pass>

in the file ~/.mplayer/mplayer-rc.
`)
		os.Exit(1)
	}
	// start mplayer, select loop and web server
	in, outChan := launchMPlayer(flags)
	commandChan := startSelectLoop(in, outChan)
	commandChan <- cmdPlay{id: -1} // initial play cmd
	startWebServer(commandChan, password, port)
}
