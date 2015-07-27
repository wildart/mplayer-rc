/*
   Copyright 2015 The MPlayer-ARC Authors. See the AUTHORS file at the
   top-level directory of this distribution and at
   <https://xi2.org/x/mplayer-arc/AUTHORS>.

   This file is part of MPlayer-ARC.

   MPlayer-ARC is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published
   by the Free Software Foundation, either version 3 of the License,
   or (at your option) any later version.

   MPlayer-ARC is distributed in the hope that it will be useful, but
   WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
   General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with MPlayer-ARC.  If not, see <https://www.gnu.org/licenses/>.
*/

// API information
//
// mplayer: http://www.mplayerhq.hu/DOCS/tech/slave.txt
//
//     vlc: https://wiki.videolan.org/VLC_HTTP_requests/
//          https://wiki.videolan.org/Documentation:Modules/http_intf/
//          https://raw.githubusercontent.com/videolan/vlc/master/share/lua/http/requests/README.txt

package main // import "xi2.org/x/mplayer-arc"

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

const license = `   Copyright 2015 The MPlayer-ARC Authors. See the AUTHORS file at the
   top-level directory of this distribution and at
   <https://xi2.org/x/mplayer-arc/AUTHORS>.

   MPlayer-ARC is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published
   by the Free Software Foundation, either version 3 of the License,
   or (at your option) any later version.

   MPlayer-ARC is distributed in the hope that it will be useful, but
   WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
   General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with MPlayer-ARC.  If not, see <https://www.gnu.org/licenses/>.

   For further information about MPlayer-ARC visit
   <https://xi2.org/x/mplayer-arc>.
`

var (
	flagUsage bool

	flagVersion      bool
	flagPassword     string
	flagPort         string
	flagMPlayerUsage bool
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
// The mplayer-arc specific flags are handled as appropriate.
//
// The mplayer flags --playlist/-playlist and --shuffle/-shuffle are
// handled by mplayer-arc and are not passed to mplayer.
func processFlags() []string {
	n := len(os.Args)

	printUsage := func() {
		fmt.Fprintf(os.Stderr,
			"usage: %s [mplayer-arc or mplayer options] [files/URLs]\n",
			filepath.Base(os.Args[0]))
		// Go 1.5+ package flag compatible format
		fmt.Fprintf(os.Stderr, "  -V\t")
		fmt.Fprintf(os.Stderr,
			"show version, license and further information\n")
		fmt.Fprintf(os.Stderr, "  --arc-password pass\n")
		fmt.Fprintf(os.Stderr,
			"    \tuse pass as the Android-VLC-Remote password\n")
		fmt.Fprintf(os.Stderr, "  --arc-port port\n")
		fmt.Fprintf(os.Stderr,
			"    \tuse port as the listening port for VLC commands (default 8080)\n")
		fmt.Fprintf(os.Stderr, "  --mplayer-help\n")
		fmt.Fprintf(os.Stderr,
			"    \tdisplay the MPlayer usage message\n")
	}
	printVersion := func() {
		if version != "" {
			fmt.Fprintf(os.Stderr, "   MPlayer-ARC version %s\n\n", version)
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
		if strings.HasPrefix(a, "--arc-password=") {
			flagPassword = a[len("--arc-password="):]
			continue
		}
		if i < n-1 && (a == "--arc-password" || a == "-arc-password") {
			flagPassword = os.Args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(a, "--arc-port=") {
			flagPort = a[len("--arc-port="):]
			continue
		}
		if i < n-1 && (a == "--arc-port" || a == "-arc-port") {
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
			scanner := bufio.NewScanner(bytes.NewBuffer(pl))
			tracks = []string{}
			// only .m3u8 files are supported at present
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

	// handle mplayer-arc flags
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
	// the stopped state
	stopped bool
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
// mode as a background process. It returns mplayer's stdin, stdout
// and stderr as io.Writer, io.Reader and io.Reader.
func launchMPlayer(flags []string) (io.Writer, io.Reader, io.Reader) {
	flags = append(
		[]string{"-idle", "-slave", "-quiet", "-noconsolecontrols"},
		flags...)
	cmd := exec.Command("mplayer", flags...)
	in, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	outerr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	// check for command line errors at mplayer startup
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "Error ") {
			// mplayer has failed to parse it's command line or has
			// otherwise failed to start
			log.Fatal(errors.New("mplayer: " + scanner.Text()))
		}
		if strings.HasPrefix(scanner.Text(), "MPlayer") {
			// all good hopefully...
			break
		}
	}
	return in, out, outerr
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

// command list.
const (
	cmdPlay = iota // input: track_id int
	cmdNext
	cmdPrev
	cmdPause // a toggle
	cmdStop
	cmdShuffle // a toggle
	cmdLoop    // a toggle
	cmdRepeat  // a toggle
	cmdAspect
	cmdAudio
	cmdSubtitle
	cmdFullscreen // a toggle
	cmdVolume     // input: volume int (0 -> 320?)
	cmdSeek       // input: position seekVal (seconds and mode)
	cmdGetProp    // input: prop string - reply: a string
)

// command is the type sent to the select loop.
type command struct {
	kind      int // cmdXXX
	input     interface{}
	replyChan chan<- interface{}
}

// seekVal is the type used as input for cmdSeek.
type seekVal struct {
	val  int // time in seconds (can be positive or negative value)
	mode int // 0 relative, 2 absolute
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
	io.WriteString(in, "loadfile "+escapeTrack(idTrackMap[id])+"\n")
	var playing bool
	var playingFile string
	for line := range outChan {
		if strings.HasPrefix(line, "Playing ") && len(line) > len("Playing ") {
			// len(line)-1 is to account for full stop
			playingFile = line[len("Playing ") : len(line)-1]
			playing = true
		}
		if line == "" && playing {
			log.Println("mplayer: cannot play file: " + playingFile)
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
// It will do nothing if playlist is empty or it is at the end of the
// playlist and loop is false.
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
			stopped = true
			playpos = 0
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
	if funcGetProp(in, outChan, "state") == "stopped" {
		funcPlay(in, outChan, -1)
		return
	}
	io.WriteString(in, "pause\n")
}

func funcStop(in io.Writer, outChan <-chan string) {
	if funcGetProp(in, outChan, "state") != "stopped" {
		if funcGetProp(in, outChan, "state") == "paused" {
			funcPause(in, outChan) // un-pause before stop
		}
		io.WriteString(in, "stop\n")
		// wait for mplayer to confirm stop
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
	shufSet := make([]int, len(playlist)-1)
	for i := range shufSet {
		shufSet[i] = i + 1 // positions in shuffled playlist
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

func funcAudio(in io.Writer) {
	// repurpose this as OSD toggle
	io.WriteString(in, "pausing_keep_force osd\n")
}
func funcSubtitle(in io.Writer, outChan <-chan string) {
	// repurpose to rewind by 10 seconds
	funcSeek(in, seekVal{val: -10, mode: 0})
}

func funcFullscreen(in io.Writer) {
	io.WriteString(in, "pausing_keep_force vo_fullscreen\n")
}

func funcVolume(in io.Writer, volume int) {
	io.WriteString(in,
		"pausing_keep_force volume "+strconv.Itoa(volume*100/320)+" 1\n")
}

func funcSeek(in io.Writer, sv seekVal) {
	io.WriteString(in,
		"pausing_keep_force seek "+
			strconv.Itoa(sv.val)+" "+strconv.Itoa(sv.mode)+"\n")
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

// startSelectLoop returns a command channel whose purpose is to
// serialize the execution of commands sent to mplayer. In a goroutine
// it uses select to wait on either a command over the command channel
// or a ticker firing which causes it to check whether the current track
// has stopped playing and act accordingly. It also sets up a channel
// to receive SIGCHILDs from mplayer and ensures the program exits
// when receiving one.
func startSelectLoop(in io.Writer, out, outerr io.Reader) chan<- command {
	commandChan := make(chan command)
	outChan := make(chan string, 1000)
	sigChan := make(chan os.Signal, 1)
	ticker := time.NewTicker(250 * time.Millisecond)
	setupSIGCHLD(sigChan)
	go func() {
		<-sigChan
		os.Exit(0)
	}()
	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			outChan <- scanner.Text()
		}
	}()
	go func() {
		scanner := bufio.NewScanner(outerr)
		for scanner.Scan() {
			outChan <- scanner.Text()
		}
	}()
	go func() {
		for {
			select {
			case cmd := <-commandChan:
				switch cmd.kind {
				case cmdPlay:
					funcPlay(in, outChan, cmd.input.(int))
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
					funcSubtitle(in, outChan)
				case cmdFullscreen:
					funcFullscreen(in)
				case cmdVolume:
					funcVolume(in, cmd.input.(int))
				case cmdSeek:
					funcSeek(in, cmd.input.(seekVal))
				case cmdGetProp:
					cmd.replyChan <- funcGetProp(
						in, outChan, cmd.input.(string))
				}
			case <-ticker.C:
				if !stopped {
					if funcGetProp(in, outChan, "state") == "stopped" {
						funcNext(in, outChan)
					}
				}
			}
		}
	}()
	return commandChan
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

// playlistXML constructs playlist.xml.
func playlistXML() string {
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
func statusXML(commandChan chan<- command) string {
	data := &statusTmplData{}
	replyChan := make(chan interface{})
	getProp := func(prop string) string {
		commandChan <- command{
			kind: cmdGetProp, input: prop, replyChan: replyChan}
		return (<-replyChan).(string)
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

func startWebServer(commandChan chan<- command, password, port string) {
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
				commandChan <- command{kind: cmdPlay, input: id}
			case "pl_next":
				commandChan <- command{kind: cmdNext}
			case "pl_previous":
				commandChan <- command{kind: cmdPrev}
			case "pl_pause":
				commandChan <- command{kind: cmdPause}
			case "pl_stop":
				commandChan <- command{kind: cmdStop}
			case "pl_random":
				commandChan <- command{kind: cmdShuffle}
			case "pl_loop":
				commandChan <- command{kind: cmdLoop}
			case "pl_repeat":
				commandChan <- command{kind: cmdRepeat}
			case "key":
				switch r.FormValue("val") {
				case "aspect-ratio":
					commandChan <- command{kind: cmdAspect}
				case "audio-track":
					commandChan <- command{kind: cmdAudio}
				case "subtitle-track":
					commandChan <- command{kind: cmdSubtitle}
				}
			case "fullscreen":
				commandChan <- command{kind: cmdFullscreen}
			case "volume":
				if i, err := strconv.Atoi(r.FormValue("val")); err == nil {
					commandChan <- command{kind: cmdVolume, input: i}
				}
			case "seek":
				val := r.FormValue("val")
				off := 0
				var sv seekVal
				if len(val) > 0 {
					switch val[0] {
					case '+', '-':
						off = 1
					default:
						sv.mode = 2
					}
					if i, err := strconv.Atoi(val[off:]); err == nil {
						if val[0] == '-' {
							sv.val = -i
						} else {
							sv.val = i
						}
						commandChan <- command{kind: cmdSeek, input: sv}
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
			io.WriteString(w, playlistXML())
		})
	if http.ListenAndServe(":"+port, nil) != nil {
		log.Fatal(errors.New("failed to start http server"))
	}
}

// main

func main() {
	flags := processFlags()
	// set default password/port
	password, port := "", "8080"
	// try to set password/port from config file
	home := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
	}
	b, err := ioutil.ReadFile(
		filepath.Join(home, ".mplayer", "mplayer-arc"))
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewBuffer(b))
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "arc-password=") {
				password = scanner.Text()[len("arc-password="):]
			}
			if strings.HasPrefix(scanner.Text(), "arc-port=") {
				port = scanner.Text()[len("arc-port="):]
			}
			// trim trailing spaces
			for len(password) > 0 && password[len(password)-1] == ' ' {
				password = password[:len(password)-1]
			}
			for len(port) > 0 && port[len(port)-1] == ' ' {
				port = port[:len(port)-1]
			}
		}
	}
	// try to set password/port from flags
	if flagPassword != "" {
		password = flagPassword
	}
	if flagPort != "" {
		port = flagPort
	}
	// if password not set, exit
	if password == "" {
		fmt.Fprint(os.Stderr,
			`MPlayer-ARC needs to have a password which is used to authorize
Android-VLC-Remote. You can specify the password using the command
line flag --arc-password=<pass> or by putting the line

  arc-password=<pass>

in the file ~/.mplayer/mplayer-arc.
`)
		os.Exit(1)
	}
	// start mplayer, select loop and web server
	in, out, outerr := launchMPlayer(flags)
	commandChan := startSelectLoop(in, out, outerr)
	commandChan <- command{kind: cmdPlay, input: -1} // initial play cmd
	startWebServer(commandChan, password, port)
}
