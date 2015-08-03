// This file was automatically generated using Genman.
// Do not edit.

// MPlayer-RC is an MPlayer/MPV wrapper enabling use of a VLC remote.
// 
// Synopsis
// 
// Usage:
// 
//   mplayer-rc [mplayer-rc or mplayer/mpv options] [files/URLs]
// 
// Description
// 
// MPlayer-RC enables remote control of the MPlayer, MPlayer2 and MPV
// command line media players from a VLC remote (i.e. an application
// written to use VLC's HTTP control interface). It is designed to work
// specifically with the Android application Android-VLC-Remote which can
// be obtained from F-Droid:
// 
//     https://f-droid.org/repository/browse/?fdid=org.peterbaldwin.client.android.vlcremote
// 
// or directly from the Android-VLC-Remote author:
// 
//     https://code.google.com/p/android-vlc-remote
// 
// Other applications speaking the VLC HTTP protocol may work but are not
// tested. VLC itself is not required since MPlayer-RC acts as a
// translator, forwarding VLC commands received from the remote to the
// backend player and returning responses back. The VLC HTTP protocol is
// described here:
// 
//     https://wiki.videolan.org/VLC_HTTP_requests
// 
// To use MPlayer-RC, invoke it in the same way you would invoke
// MPlayer. For example:
// 
//     mplayer-rc -ao alsa track1.mp3 track2.mp3
// 
// or
// 
//     mplayer-rc -playlist file
// 
// You can then control the player using Android-VLC-Remote on your
// Android device.
// 
// Android-VLC-Remote will prompt you for a password which you need to
// inform MPlayer-RC about beforehand. For this you can use the command
// line flag -password or put the line
// 
//     password=...
// 
// in the file ~/.mplayer-rc. Similarly, you can also use
// 
//     port=...
// 
// to change the default listening port from 8080.
// 
// By default, MPlayer-RC uses MPlayer/MPlayer2 as its backend player. To
// use MPV instead you can specify -backend mpv on the command line,
// rename the mplayer-rc binary to mpv-rc, or put
// 
//     backend=mpv
// 
// in ~/.mplayer-rc.
// 
// Options
// 
// The following flags are available:
// 
//   -V    show version, license and further information
//   -backend backend
//         set backend as the backend player (default mplayer)
//   -password pass
//         use pass as the VLC remote password
//   -port port
//         use port as the listening port for VLC commands (default 8080)
//   -remap-commands
//         use alternate actions for some VLC commands
// 
// Files
// 
// ~/.mplayer-rc - configuration file
// 
// Playlists
// 
// Files and URLs are not passed through to the backend player as command
// line arguments but are instead retained by MPlayer-RC since they are
// needed to implement shuffle. The backend is started by MPlayer-RC in
// slave mode without any files/URLs on its command line and then asked
// to play tracks one at a time via its slave mode protocol.
// 
// As a consequence of this there is currently a restriction on the
// format of a playlist file. It must be UTF-8 "one file/URL per line"
// format or a .m3u8 file. This is because it is not passed through using
// the -playlist flag and is parsed instead by MPlayer-RC, whose parsing
// is less sophisticated.
// 
// Signals
// 
// Since MPlayer-RC takes handling of the playlist away from the backend,
// the < and > keyboard keys (next/previous playlist entry) stop working
// since as far as the backend is concerned there is just one playlist
// entry. However, it is possible to work around this as follows:
// 
// When running on a Unix system, MPlayer-RC responds to SIGUSR1 and
// SIGUSR2 signals by executing previous track and next track
// commands. This allows you to put something like this in your
// input.conf (for MPV)
// 
//    < run killall -SIGUSR1 mplayer-rc
//    > run killall -SIGUSR2 mplayer-rc
// 
// or this (for MPlayer)
// 
//    < run "killall -SIGUSR1 mplayer-rc"
//    > run "killall -SIGUSR2 mplayer-rc"
// 
// and your < and > keys should then work again.
// 
// Command remapping
// 
// If the -remap-commands flag is given to MPlayer-RC or
// remap-commands=yes is set in ~/.mplayer-rc then some buttons within
// Android-VLC-Remote are repurposed to be more useful:
// 
//     • The "Audio track" button is repurposed to cycle through OSD modes.
// 
//     • The "Subtitle track" button is repurposed to rewind by 10 seconds.
// 
//     • The "Aspect ratio" button is repurposed to fast forward by 10 seconds.
// 
// The primary reason for remapping is to have an easy way to quickly
// fast forward and rewind when Android-VLC-Remote is used in portrait
// mode on a small screen. The alternative otherwise is
// forwarding/rewinding using the progress slider which is quite fiddly.
// 
// Status
// 
// The following features of Android-VLC-Remote are working:
// 
//     • Playing tab: All features (play, pause, stop, forward, back,
// loop, repeat, volume, shuffle, fullscreen, aspect toggle etc).
// 
//     • Playlist tab: Selecting tracks works as normal.
// 
// The following features of Android-VLC-Remote do not work:
// 
//     • Library tab.
// 
//     • DVD tab.
// 
//     • Metadata: The metadata passed through to the information box is
// just the filename (as "title").
// 
// See also
// 
// mplayer(1), mpv(1)
package main
