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

// API information
//
// mplayer: http://www.mplayerhq.hu/DOCS/tech/slave.txt
//
//     mpv: http://mpv.io/manual/master/#command-interface
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
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
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
   <https://xi2.org/x/mplayer-rc/m/AUTHORS>.

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

// variables set by flag processing
var (
	flagUsage bool

	flagVersion       bool
	flagPassword      string
	flagPort          string
	flagRemapCommands bool
	flagFormat        string
)

// variables set by config file processing
var (
	confBackend       string
	confPassword      string
	confPort          string = "8080"
	confRemapCommands bool
	confFormat        string = "xml"
)

func trimTrailingSpace(s string) string {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

// processConfig parses the config file and sets the conf* variables
func processConfig() {
	home := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
	}
	b, err := ioutil.ReadFile(
		filepath.Join(home, ".mplayer-rc"))
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(bytes.NewBuffer(b))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "backend=") {
			p := scanner.Text()[len("backend="):]
			confBackend = trimTrailingSpace(p)
		}
		if strings.HasPrefix(scanner.Text(), "password=") {
			p := scanner.Text()[len("password="):]
			confPassword = trimTrailingSpace(p)
		}
		if strings.HasPrefix(scanner.Text(), "port=") {
			p := scanner.Text()[len("port="):]
			confPort = trimTrailingSpace(p)
		}
		if strings.HasPrefix(scanner.Text(), "remap-commands=") {
			p := scanner.Text()[len("remap-commands="):]
			p = strings.ToLower(trimTrailingSpace(p))
			switch p {
			case "yes", "1", "true":
				confRemapCommands = true
			}
		}
		if strings.HasPrefix(scanner.Text(), "format=") {
			p := scanner.Text()[len("format="):]
			confFormat = strings.ToLower(trimTrailingSpace(p))
		}
	}
}

// setBackend sets the backend by considering os.Args[0], the config
// file and command line flags. It returns the processed os.Args
func setBackend() []string {
	args := os.Args
	// set a default backend
	if _, err := exec.LookPath("mpv"); err == nil {
		backend = &backendMPV
	}
	if _, err := exec.LookPath("mplayer"); err == nil {
		backend = &backendMPlayer
	}
	if backend == nil {
		log.Fatalf("mplayer-rc: cannot find mpv or mplayer binaries")
	}
	// set using args[0] if the relevant binary exists
	switch strings.ToLower(filepath.Base(args[0])) {
	case "mpv-rc", "mpv-rc.exe":
		if _, err := exec.LookPath("mpv"); err == nil {
			backend = &backendMPV
		}
	case "mplayer-rc", "mplayer-rc.exe":
		if _, err := exec.LookPath("mplayer"); err == nil {
			backend = &backendMPlayer
		}
	}
	// set using config file
	switch confBackend {
	case "mplayer":
		backend = &backendMPlayer
	case "mpv":
		backend = &backendMPV
	}
	// set using flags
	for i := 1; i < len(args)-1; i++ {
		if args[i] == "--" {
			break
		}
		if args[i] == "-backend" {
			if args[i+1] == "mplayer" {
				backend = &backendMPlayer
				args = append(args[:i], args[i+2:]...)
				break
			}
			if args[i+1] == "mpv" {
				backend = &backendMPV
				args = append(args[:i], args[i+2:]...)
				break
			}
		}
	}
	return args
}

// needsParameter returns true if flag is a backend flag requiring a
// parameter.
//
// Examples:
//   -vf => true
//   -fs => false
func needsParameter(flag string) bool {
	out, _ := exec.Command(backend.binary, flag).CombinedOutput()
	scanner := bufio.NewScanner(bytes.NewBuffer(out))
	if !scanner.Scan() {
		return false
	}
	if strings.HasPrefix(scanner.Text(), backend.matchNeedsParam) {
		return true
	}
	return false
}

// processFlags processes args received from setBackend and creates
// the playlist state from the relevant command line arguments. It
// returns a list of flags to be passed to the backend.
//
// The mplayer-rc specific flags are handled as appropriate.
//
// The flags -playlist and -shuffle are handled by mplayer-rc and are
// not passed to the backend.
func processFlags(args []string) []string {
	n := len(args)

	printUsage := func() {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [mplayer-rc or mplayer/mpv flags] [files/URLs]\n\n",
			filepath.Base(args[0]))
		// Go 1.5+ package flag compatible format
		fmt.Fprintf(os.Stderr, "  -V\t")
		fmt.Fprintf(os.Stderr,
			"show version, license and further information\n")
		fmt.Fprintf(os.Stderr, "  -backend backend\n")
		fmt.Fprintf(os.Stderr,
			"    \tset backend as the backend player (default mplayer)\n")
		fmt.Fprintf(os.Stderr, "  -password pass\n")
		fmt.Fprintf(os.Stderr,
			"    \tuse pass as the VLC remote password\n")
		fmt.Fprintf(os.Stderr, "  -port port\n")
		fmt.Fprintf(os.Stderr,
			"    \tuse port as the listening port for VLC commands (default 8080)\n")
		fmt.Fprintf(os.Stderr, "  -remap-commands\n")
		fmt.Fprintf(os.Stderr,
			"    \tuse alternate actions for some VLC commands\n")
	}
	printVersion := func() {
		if version != "" {
			fmt.Fprintf(os.Stderr, "   MPlayer-RC version %s\n\n", version)
		}
		fmt.Fprintf(os.Stderr, license)
	}

	// process flags
	doShuffle := false
	var flags, tracks []string
	for i := 1; i < n; i++ {
		a := args[i]
		if a == "--" {
			tracks = append(tracks, args[i+1:]...)
			break
		}
		if len(a) > 0 && a[0] != '-' {
			tracks = append(tracks, a)
			continue
		}
		if a == "-remap-commands" {
			flagRemapCommands = true
			continue
		}
		if a == "-format" {
			flagFormat = args[i+1]
			i++
			continue
		}
		if i < n-1 && a == "-password" {
			flagPassword = args[i+1]
			i++
			continue
		}
		if i < n-1 && a == "-port" {
			flagPort = args[i+1]
			i++
			continue
		}
		if a == "-shuffle" || a == "--shuffle" {
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
		isPlaylist := false
		playlist := ""
		if strings.HasPrefix(a, "--playlist=") {
			playlist = a[len("--playlist="):]
			isPlaylist = true
		}
		if i < n-1 && a == "-playlist" {
			playlist = args[i+1]
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
					log.Fatalf("mplayer-rc: %s", s.msg)
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
		if i < n-1 && needsParameter(a) {
			flags = append(flags, a, args[i+1])
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
	if flagUsage || len(tracks) == 0 {
		printUsage()
		os.Exit(2)
	}

	// create playlist state
	for _, f := range tracks {
		addPlaylistEntry(f)
	}
	if doShuffle {
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
	// the stopped state. The backend can briefly transition into
	// "stopped" state in-between tracks when we are not really
	// stopped, so this variable allows us to keep a true idea of
	// whether the backend is stopped or not.
	stopped bool
	// whether we remap some VLC commands to perform alternate actions
	remapCommands bool
	// the response format (XML or JSON)
	responseFormat string
	// the backend, set by setBackend
	backend *backendData
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

// launchBackend starts up the backend with the provided flags in
// slave mode. It returns the backend's stdin as an io.Writer, and the
// combined stdout/stderr as a <-chan string.
//
// The stdout/stderr is prefiltered by a goroutine that looks for
// matchCmdPrev/matchCmdNext strings. If it sees them it puts
// cmdPrev{}/cmdNext{} into commandChan.
func launchBackend(commandChan chan<- interface{}, flags []string) (io.Writer, <-chan string) {
	startFlags := append([]string{}, backend.startFlags...)
	flags = append(startFlags, flags...)
	cmd := exec.Command(backend.binary, flags...)
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
			switch {
			case strings.HasPrefix(scanner.Text(), backend.matchCmdPrev):
				go func() {
					commandChan <- cmdPrev{}
				}()
			case strings.HasPrefix(scanner.Text(), backend.matchCmdNext):
				go func() {
					commandChan <- cmdNext{}
				}()
			default:
				outChan <- scanner.Text()
			}
		}
	}()
	// give a bad command to force MPV to give some output at startup
	io.WriteString(in, "XXXX\n")
	// check for command line errors at backend startup
	for line := range outChan {
		if strings.HasPrefix(line, backend.matchStartupFail) {
			// backend has failed to parse it's command line or has
			// otherwise failed to start
			log.Fatalf("%s: %s", backend.binary, line)
		}
		if strings.HasPrefix(line, backend.matchStartupOK) {
			// all good hopefully...
			break
		}
	}
	return in, outChan
}

// escapeTrack escapes a filename/URL so it is suitable to pass
// to the backend with backend.cmdLoadfile
func escapeTrack(track string) string {
	track = strings.Replace(track, `\`, `\\`, -1)
	track = strings.Replace(track, `"`, `\"`, -1)
	return `"` + track + `"`
}

// getProp gets a property value from the backend. It also handles the
// pseudo-property, "state", and harmonizes backend responses.
func getProp(in io.Writer, outChan <-chan string, prop string) string {
	if prop == "state" {
		// first deal with the pseudo-property, "state"
		trackname := getProp(in, outChan, "filename")
		if trackname == "(unavailable)" {
			return "stopped"
		}
		if getProp(in, outChan, "pause") == "yes" {
			return "paused"
		}
		return "playing"
	}
	// now deal with real properties.
	fmt.Fprintf(in, backend.cmdGetProp+"\n", prop, prop)
	var ans string
	for line := range outChan {
		if line == "ANS_ERROR=PROPERTY_UNAVAILABLE" {
			// convert MPlayer response to MPV response
			ans = "(unavailable)"
			break
		}
		if strings.HasPrefix(line, "ANS_ERROR=") {
			// convert MPlayer response to MPV response
			ans = "(error)"
			break
		}
		if strings.HasPrefix(line, "ANS_"+prop+"=") {
			ans = line[len("ANS_"+prop+"="):]
			break
		}
	}
	switch ans {
	case "(unavailable)", "(error)":
		return ans
	}
	// do some conversions to harmonize backend responses
	switch prop {
	case backend.propLength, backend.propTimePos:
		// convert float to int (MPlayer)
		if strings.Contains(ans, ".") {
			if f, err := strconv.ParseFloat(ans, 64); err == nil {
				ans = strconv.Itoa(int(f))
			}
		}
		// convert HH:MM:SS to seconds (MPV)
		if strings.Contains(ans, ":") {
			var result int
			for _, s := range strings.Split(ans, ":") {
				if i, err := strconv.Atoi(s); err == nil {
					result = 60*result + i
				}
			}
			ans = strconv.Itoa(result)
		}
	case backend.propVolume:
		// convert volume to integer in range 0->320
		var vol int
		if f, err := strconv.ParseFloat(ans, 64); err == nil {
			vol = int(f)
		}
		ans = strconv.Itoa(vol * 320 / backend.volumeMax)
	}
	return ans
}

// "select loop" commands and their associated command functions.
// each cmdXXX has a corresponding funcXXX.

const (
	volAbs  = 1 // volume mode absolute
	volRel  = 0 // volume mode relative
	seekAbs = 2 // seek mode absolute
	seekPct = 1 // seek mode percent
	seekRel = 0 // seek mode relative
)

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
	val  int // volume (0 -> 320 in absolute mode)
	mode int // mode: absolute/relative
}
type cmdSeek struct {
	val  int // time in seconds/percent (can be positive or negative value)
	mode int // mode: absolute/percent/relative
}
type cmdGetPlaylist struct {
	replyChan chan<- string
}
type cmdGetStatus struct {
	replyChan chan<- string
}
type cmdGetBrowse struct {
	replyChan chan<- string
	uri       string
}
type cmdQuit struct{}
type cmdSetPlaylist struct {
	uri string
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
	// if backend could not play the previous track it will ignore the
	// next command sometimes (MPlayer at least does this). In case
	// this is true, send it a Noop command first.
	fmt.Fprintf(in, backend.cmdNoop+"\n")
	fmt.Fprintf(in, backend.cmdLoadfile+"\n", escapeTrack(idTrackMap[id]))
	var playing bool
	var playingTrack string
	for line := range outChan {
		if strings.HasPrefix(line, backend.matchPlayingPrefix) &&
			strings.HasSuffix(line, backend.matchPlayingSuffix) &&
			len(line) >= len(backend.matchPlayingPrefix)+
				len(backend.matchPlayingSuffix) {
			playingTrack = line[len(backend.matchPlayingPrefix) : len(line)-
				len(backend.matchPlayingSuffix)]
			playing = true
		}
		if line == "" && playing {
			log.Println(
				backend.binary + ": cannot play track: " + playingTrack)
			funcNext(in, outChan)
			return
		}
		for _, match := range backend.matchPlayingOK {
			if strings.HasPrefix(line, match) {
				// valid track found
				stopped = false
				return
			}
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
// funcNext has been called as a result of a track finishing it will
// set playpos to 0 and stopped to true, otherwise it will do nothing.
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
			if !stopped && getProp(in, outChan, "state") == "stopped" {
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
	fmt.Fprintf(in, backend.cmdPause+"\n")
}

func funcStop(in io.Writer, outChan <-chan string) {
	if !stopped {
		if getProp(in, outChan, "state") == "paused" {
			funcPause(in, outChan) // un-pause before stop
		}
		fmt.Fprintf(in, backend.cmdStop+"\n")
		// wait for backend to confirm stop before updating the
		// stopped state
		ticker := time.NewTicker(250 * time.Millisecond)
		for _ = range ticker.C {
			if getProp(in, outChan, "state") == "stopped" {
				ticker.Stop()
				stopped = true
				return
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
	// the set of shuffled positions. Position zero is not included
	// since the current track will be shuffled to position zero
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
			strings.Split(getProp(in, outChan, backend.propAspect), " ")[0],
			64); err == nil {
			// cycle between 4:3, 16:9 and 2.35:1
			switch {
			case f < 1.5555:
				fmt.Fprintf(in, backend.cmdSwitchRatio+"\n", "1.7777")
			case f < 2.05:
				fmt.Fprintf(in, backend.cmdSwitchRatio+"\n", "2.35")
			default:
				fmt.Fprintf(in, backend.cmdSwitchRatio+"\n", "1.3333")
			}
		}
	}
}

func funcAudio(in io.Writer) {
	if remapCommands {
		// repurpose this as OSD toggle
		fmt.Fprintf(in, backend.cmdOSD+"\n")
	} else {
		fmt.Fprintf(in, backend.cmdSwitchAudio+"\n")
	}
}

func funcSubtitle(in io.Writer) {
	if remapCommands {
		// repurpose to rewind by 10 seconds
		funcSeek(in, -10, 0)
	} else {
		fmt.Fprintf(in, backend.cmdSubSelect+"\n")
	}
}

func funcFullscreen(in io.Writer) {
	fmt.Fprintf(in, backend.cmdFullscreen+"\n")
}

func funcVolume(in io.Writer, val, mode int) {
	val = val * backend.volumeMax / 320
	switch mode {
	case volAbs: // absolute
		fmt.Fprintf(in, backend.cmdVolumeAbs+"\n", val)
	case volRel: // relative
		fmt.Fprintf(in, backend.cmdVolumeRel+"\n", val)
	}
}

func funcSeek(in io.Writer, val, mode int) {
	switch mode {
	case seekAbs: // absolute
		fmt.Fprintf(in, backend.cmdSeekAbs+"\n", val)
	case seekPct: // percent
		fmt.Fprintf(in, backend.cmdSeekPct+"\n", val)
	case seekRel: // relative
		fmt.Fprintf(in, backend.cmdSeekRel+"\n", val)
	}
}

// playlist.xml

const playlistTmplTxt = `
<node ro="rw" name="Undefined" id="1">
<node ro="ro" name="Playlist" id="2">
{{range .}}
<leaf duration="-1" ro="rw" name="{{.Name}}"
 id="{{.ID}}" {{if .Current}}current="current"{{end}}></leaf>
{{end}}
</node>
<node ro="ro" name="Media Library" id="3"></node>
</node>
`

type playlistChild interface {
}

type playlistNode struct {
	Type     string          `json:"type"`
	RO       string          `json:"ro"`
	Name     string          `json:"name"`
	ID       string          `json:"id"`
	Children []playlistChild `json:"children"`
}

func NewPlaylistNode(name string, id int, rw bool) playlistNode {
	pln := playlistNode{}
	pln.Type = "node"
	if rw {
		pln.RO = "rw"
	} else {
		pln.RO = "ro"
	}
	pln.Name = name
	pln.ID = strconv.Itoa(id)
	pln.Children = []playlistChild{}
	return pln
}

var playlistTmpl = template.Must(
	template.New("playlist").Parse(playlistTmplTxt))

// funcGetPlaylistXML constructs playlist.xml.
func funcGetPlaylistXML() string {
	data := []struct {
		Name    string
		ID      int
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
			ID      int
			Current bool
		}{Name: name, ID: id, Current: current})
	}
	buf := new(bytes.Buffer)
	buf.WriteString(`<?xml version="1.0" encoding="utf-8" standalone="yes" ?>`)
	err := playlistTmpl.Execute(buf, data)
	if err != nil {
		log.Fatal(err)
	}
	return buf.String()
}

func funcGetPlaylistJSON() string {
	pl := NewPlaylistNode("Undefined", 1, true)
	pl.Children = append(pl.Children, NewPlaylistNode("Playlist", 2, false))
	pl.Children = append(pl.Children, NewPlaylistNode("Media Library", 3, false))
	plc := pl.Children[0].(playlistNode)
	for i := range playlist {
		id := playlist[shufToPos[i]]
		name := filepath.Base(idTrackMap[id])
		cur := ""
		if id == playlist[playpos] {
			cur = "current"
		}
		ch := NewPlaylistNode(name, id, true)
		leaf := struct {
			playlistNode
			URI      string `json:"uri"`
			Duration int    `json:"duration"`
			Type     string `json:"type"`
			Current  string `json:"current,omitempty"`
		}{
			playlistNode: ch,
			URI:          idTrackMap[id],
			Duration:     3630,
			Type:         "leaf",
			Current:      cur,
		}
		plc.Children = append(plc.Children, leaf)
	}
	pl.Children[0] = plc
	buf, _ := json.Marshal(pl)
	return string(buf)
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
	Fullscreen bool   `json:"fullscreen"`
	Volume     int    `json:"volume"`
	Loop       bool   `json:"loop"`
	Random     bool   `json:"random"`
	Length     int    `json:"length"`
	Repeat     bool   `json:"repeat"`
	State      string `json:"state"`
	Time       int    `json:"time"`
	Title      string `json:"title,omitempty"`
	Filename   string `json:"filename,omitempty"`
}

var statusTmpl = template.Must(template.New("status").Parse(statusTmplTxt))

func getInt(prop string) int {
	if i, err := strconv.Atoi(prop); err == nil {
		return i
	}
	return 0
}

func getBool(prop string) bool {
	if prop == "yes" {
		return true
	}
	return false
}

// funcGetStatusXML constructs status.xml.
func funcGetStatusXML(in io.Writer, outChan <-chan string) string {
	data := &statusTmplData{}
	get := func(prop string) string {
		return getProp(in, outChan, prop)
	}
	data.Fullscreen = getBool(get(backend.propFullscreen))
	data.Volume = getInt(get(backend.propVolume))
	data.Loop = loop
	data.Random = shuffle
	data.Length = getInt(get(backend.propLength))
	data.Repeat = repeat
	data.State = get("state")
	data.Time = getInt(get(backend.propTimePos))
	filename := get(backend.propFilename)
	if filename != "(unavailable)" {
		data.Title = filename
		data.Filename = filename
	}
	buf := new(bytes.Buffer)
	buf.WriteString(`<?xml version="1.0" encoding="utf-8" standalone="yes" ?>`)
	err := statusTmpl.Execute(buf, data)
	if err != nil {
		log.Fatal(err)
	}
	return buf.String()
}

// funcGetStatusJSON constructs status.json.
func funcGetStatusJSON(in io.Writer, outChan <-chan string) string {
	get := func(prop string) string {
		return getProp(in, outChan, prop)
	}
	filename := get(backend.propFilename)
	if filename == "(unavailable)" {
		filename = ""
	}
	status := map[string]interface{}{
		"audiodelay":    0,
		"subtitledelay": 0,
		"aspectratio":   "default",
		"rate":          1,
		"version":       "2.2.2 Weatherwax",
		"repeat":        repeat,
		"time":          getInt(get(backend.propTimePos)),
		"currentplid":   playlist[playpos],
		"information": map[string]interface{}{
			"chapters": []string{},
			"titles":   []string{},
			"chapter":  0,
			"title":    0,
			"category": map[string]interface{}{
				"meta": map[string]interface{}{
					"filename": filename,
					"album":    "",
					"artist":   "",
				},
			},
		},
		"loop":       loop,
		"volume":     getInt(get(backend.propVolume)),
		"state":      get("state"),
		"length":     getInt(get(backend.propLength)),
		"random":     shuffle,
		"fullscreen": getBool(get(backend.propFullscreen)),
	}

	buf, _ := json.Marshal(status)
	// log.Println(string(buf))
	return string(buf)
}

func funcGetBrowseXML(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.Fatal(err)
		return "<root></root>"
	}
	log.Println(u)
	return "<root></root>"
}

func funcGetBrowseJSON(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.Fatal(err)
		return "{\"element\":[]}"
	}

	elements := [](map[string]interface{}){}
	// if u.Path != "/" {
	elements = append(elements, map[string]interface{}{
		"uri":  uri + "/..",
		"type": "dir",
		"size": 4096,
		"name": "..",
		"path": u.Path + "/..",
	})
	// }
	files, _ := ioutil.ReadDir(u.Path)
	for _, f := range files {
		ftype, fsize := "dir", int64(4096)
		if !f.IsDir() {
			ftype = "file"
			fsize = f.Size()
		}
		fpath := path.Join(u.Path, f.Name())
		elements = append(elements, map[string]interface{}{
			"uri":  u.Scheme + "://" + fpath,
			"type": ftype,
			"size": fsize,
			"name": f.Name(),
			"path": fpath,
		})
	}

	buf, _ := json.Marshal(map[string]interface{}{
		"element": elements,
	})
	// log.Println(string(buf))
	return string(buf)
}

func funcSetPlaylist(in io.Writer, uri string) {
	u, err := url.Parse(uri)
	if err != nil {
		log.Fatal(err)
		return
	}
	// Add to internal playlist
	addPlaylistEntry(u.Path)
	// Add to player playlist
	fmt.Fprintf(in, backend.cmdNoop+"\n")
	fmt.Fprintf(in, backend.cmdLoadfile+"\n", escapeTrack(idTrackMap[idCounter-1]))
}

// startSelectLoop starts the select loop whose purpose is to
// serialize the execution of commands sent to the backend. In a
// goroutine it uses select to wait on either a command over the
// command channel, output from the backend (which is discarded) or a
// ticker firing (which causes it to check whether the current track
// has stopped playing). All interactions with the backend (using the
// funcXXX or getProp functions) or manipulations of global state are
// performed from the select loop goroutine.
//
// When using Unix, startSelectLoop also starts up a signal handler in
// a goroutine to handle SIGCHLD.
func startSelectLoop(commandChan <-chan interface{}, in io.Writer, outChan <-chan string) {
	ticker := time.NewTicker(250 * time.Millisecond)
	startSignalHandler()
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
				case cmdGetPlaylist:
					var playlist string = ""
					if responseFormat == "xml" {
						playlist = funcGetPlaylistXML()
					} else if responseFormat == "json" {
						playlist = funcGetPlaylistJSON()
					}
					cmd.replyChan <- playlist
				case cmdGetStatus:
					var status string = ""
					if responseFormat == "xml" {
						status = funcGetStatusXML(in, outChan)
					} else if responseFormat == "json" {
						status = funcGetStatusJSON(in, outChan)
					}
					cmd.replyChan <- status
				case cmdGetBrowse:
					var browsefiles string = ""
					if responseFormat == "xml" {
						browsefiles = funcGetBrowseXML(cmd.uri)
					} else if responseFormat == "json" {
						browsefiles = funcGetBrowseJSON(cmd.uri)
					}
					cmd.replyChan <- browsefiles
				case cmdSetPlaylist:
					funcSetPlaylist(in, cmd.uri)
				case cmdQuit:
					fmt.Fprintf(in, backend.cmdQuit+"\n")
					os.Exit(0)
				}
			case <-outChan:
				// discard unused output from the backend
			case <-ticker.C:
				if !stopped && getProp(in, outChan, "state") == "stopped" {
					funcNext(in, outChan)
				}
			}
		}
	}()
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
	staturl := "/requests/status." + responseFormat
	http.HandleFunc(
		staturl, func(w http.ResponseWriter, r *http.Request) {
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
				case "quit":
					commandChan <- cmdQuit{}
				}
			case "fullscreen":
				commandChan <- cmdFullscreen{}
			case "volume":
				val := r.FormValue("val")
				var off int
				mode := volAbs
				percent := false
				if len(val) > 0 && val[len(val)-1] == '%' {
					val = val[:len(val)-1]
					percent = true
				}
				if len(val) > 0 {
					switch val[0] {
					// note: we get ' ' when + is not URL-encoded
					case '+', '-', ' ':
						// relative mode
						mode = volRel
						off = 1
					default:
						// absolute mode
					}
					if i, err := strconv.Atoi(val[off:]); err == nil {
						if percent {
							i = i * 320 / 100
						}
						if val[0] == '-' {
							i = -i
						}
						commandChan <- cmdVolume{val: i, mode: mode}
					}
				}
			case "seek":
				val := r.FormValue("val")
				var off int
				mode := seekAbs
				if len(val) > 0 && val[len(val)-1] == '%' {
					// percent mode
					val = val[:len(val)-1]
					mode = seekPct
				}
				if len(val) > 0 &&
					(val[len(val)-1] == 's' || val[len(val)-1] == 'S') {
					val = val[:len(val)-1]
				}
				if len(val) > 0 {
					switch val[0] {
					// note: we get ' ' when + is not URL-encoded
					case '+', '-', ' ':
						// relative mode
						mode = seekRel
						off = 1
					default:
						// absolute mode
					}
					if i, err := strconv.Atoi(val[off:]); err == nil {
						if val[0] == '-' {
							i = -i
						}
						commandChan <- cmdSeek{val: i, mode: mode}
					}
				}
			case "in_play":
				if inPath := r.FormValue("input"); inPath != "" {
					commandChan <- cmdSetPlaylist{uri: inPath}
				}
			}
			// allways output status after operation
			replyChan := make(chan string, 1)
			commandChan <- cmdGetStatus{replyChan: replyChan}
			io.WriteString(w, <-replyChan)
		})
	plurl := "/requests/playlist." + responseFormat
	http.HandleFunc(
		plurl,
		func(w http.ResponseWriter, r *http.Request) {
			if !authorized(w, r, "", password) {
				return
			}
			// output playlist
			replyChan := make(chan string, 1)
			commandChan <- cmdGetPlaylist{replyChan: replyChan}
			io.WriteString(w, <-replyChan)
		})
	brwurl := "/requests/browse." + responseFormat
	http.HandleFunc(
		brwurl,
		func(w http.ResponseWriter, r *http.Request) {
			if !authorized(w, r, "", password) {
				return
			}
			// output browse data
			replyChan := make(chan string, 1)
			commandChan <- cmdGetBrowse{replyChan: replyChan, uri: r.URL.Query().Get("uri")}
			io.WriteString(w, <-replyChan)
		})
	if http.ListenAndServe(":"+port, nil) != nil {
		log.Fatalf("mplayer-rc: failed to start http server")
	}
}

// main

func main() {
	processConfig()
	args := setBackend()
	flags := processFlags(args)
	// set some variables from config file
	remapCommands = confRemapCommands
	responseFormat = confFormat
	password, port := confPassword, confPort
	// override with flags if appropriate
	if flagRemapCommands {
		remapCommands = true
	}
	if flagFormat != "" {
		responseFormat = flagFormat
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
the VLC Remote. You can specify the password using the command
line flag -password <pass> or by putting the line

  password=<pass>

in the file ~/.mplayer-rc.
`)
		os.Exit(1)
	}
	// create command channel
	commandChan := make(chan interface{}, 1000)
	// start backend, select loop and web server
	in, outChan := launchBackend(commandChan, flags)
	startSelectLoop(commandChan, in, outChan)
	commandChan <- cmdPlay{id: -1} // initial play cmd
	startWebServer(commandChan, password, port)
}
