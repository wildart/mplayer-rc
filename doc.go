// This file was automatically generated using Genman.
// Do not edit.

// MPlayer-RC is an MPlayer wrapper enabling remote control from a VLC remote.
// 
// Synopsis
// 
// Usage:
// 
//   mplayer-rc [mplayer-rc or mplayer options] [files/URLs]
// 
// Description
// 
// MPlayer-RC enables remote control of the MPlayer and MPlayer2 command
// line media players from a VLC remote (i.e. an application written to
// use VLC's HTTP control interface). It is designed to work specifically
// with the Android application Android-VLC-Remote which can be obtained
// from F-Droid:
// 
//     https://f-droid.org/repository/browse/?fdid=org.peterbaldwin.client.android.vlcremote
// 
// or directly from the Android-VLC-Remote author at
// 
//     https://code.google.com/p/android-vlc-remote
// 
// Other applications speaking the VLC HTTP protocol may work but are not
// tested. VLC itself is not required since MPlayer-RC acts as a
// translator, forwarding VLC HTTP commands received to MPlayer and
// returning responses back.
// 
// Invoke MPlayer-RC in the same way you would invoke MPlayer. For
// example:
// 
//     mplayer-rc -ao alsa track1.mp3 track2.mp3
// 
// or
// 
//     mplayer-rc -playlist file
// 
// You can then control MPlayer using Android-VLC-Remote on your Android
// device.
// 
// Android-VLC-Remote will prompt you for a password which you need to
// inform MPlayer-RC about beforehand. For this you can use the special
// command line flag -rc-password or put the line
// 
//     rc-password=...
// 
// in the file ~/.mplayer/mplayer-rc. Similarly, you can also use
// 
//     rc-port=...
// 
// to change the default listening port (8080) instead of using the
// -rc-port flag.
// 
// Options
// 
// The following flags are available:
// 
//   -V    show version, license and further information
//   -remap-commands
//         use alternate actions for some VLC commands
//   -rc-password pass
//         use pass as the Android-VLC-Remote password
//   -rc-port port
//         use port as the listening port for VLC commands (default 8080)
//   -mplayer-help
//         display the MPlayer usage message
// 
// Files
// 
// ~/.mplayer/mplayer-rc - configuration file
// 
// Notes
// 
// Files and URLs are not passed through to MPlayer as command line
// arguments and are instead retained by MPlayer-RC since they are
// needed to implement shuffle. MPlayer is started by MPlayer-RC in
// slave mode without any files/URLs on its command line and then asked
// to play tracks via its slave mode protocol.
// 
// As a consequence of this there is currently a restriction on the
// format of a playlist file. It must be UTF-8 "one file/URL per line"
// format or a .m3u8 file. This is because it is not passed through to
// MPlayer as a -playlist ... flag and is parsed instead by MPlayer-RC,
// whose parsing is less sophisticated than MPlayer's.
// 
// If the -remap-commands flag is given to MPlayer-RC or
// remap-commands=true is set in its config file then some buttons within
// Android-VLC-Remote are repurposed to be more useful:
// 
//     • The "Audio track" button is repurposed to cycle through OSD modes.
// 
//     • The "Subtitle track" button is repurposed to rewind by 10 seconds.
// 
//     • The "Aspect ratio" button is repurposed to fast forward by 10 seconds.
// 
// Having the subtitle track/aspect ratio buttons remapped is convenient
// when Android-VLC-Remote is used in portrait mode on a small
// screen. The alternative of fast forwarding/rewinding using the
// progress slider is otherwise quite fiddly.
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
// In testing, MPlayer2 seems to be more responsive than MPlayer with
// certain types of files. It is not known why this is.
// 
// See also
// 
// mplayer(1)
package main
