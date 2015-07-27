// This file was automatically generated using Genman.
// Do not edit.

// MPlayer-ARC is a wrapper for MPlayer enabling Android Remote Control.
// 
// Synopsis
// 
// Usage:
// 
//   mplayer-arc [mplayer-arc options] [mplayer options] [files/URLs]
// 
// Description
// 
// MPlayer-ARC enables remote control of the MPlayer and MPlayer2 command
// line media players using an Android device (ARC meaning Android Remote
// Control). It is designed to work specifically with the Android
// application Android-VLC-Remote which can be obtained from
// 
//   https://f-droid.org
// 
// or direct from the Android-VLC-Remote author at
// 
//   https://code.google.com/p/android-vlc-remote
// 
// VLC itself is not required since MPlayer-ARC acts as a translator,
// conveying "VLC commands" received from the Android client to MPlayer
// and returning reponses back.
// 
// Invoke MPlayer-ARC in the same way you would invoke MPlayer. For
// example:
// 
//   mplayer-arc -ao alsa track1.mp3 track2.mp3
// 
// or
// 
//   mplayer-arc --playlist=file
// 
// You can then control MPlayer using Android-VLC-Remote on your Android
// device.
// 
// Android-VLC-Remote will prompt you for a password which you need to
// inform MPlayer-ARC about beforehand. For this you can use the special
// command line flag --arc-password or put the line
// 
//   arc-password=...
// 
// in the file ~/.mplayer/mplayer-arc. Similarly, you can also use
// 
//   arc-port=...
// 
// to change the default listening port instead of using the --arc-port
// flag.
// 
// Options
// 
// The following flags are available:
// 
//   -V    show version, license and further information
//   --arc-password pass
//         use pass as the Android-VLC-Remote password
//   --arc-port port
//         set the listening port for VLC commands (default 8080)
//   --mplayer-help
//         display the MPlayer usage message
// 
// Files
// 
// ~/.mplayer/mplayer-arc - configuration file
// 
// Notes
// 
// Files and URLs are not passed through to MPlayer as command line
// arguments and are instead retained by MPlayer-ARC since they are
// needed to implement shuffle. MPlayer is started by MPlayer-ARC in
// slave mode without any files/URLs on its command line and then asked
// to play tracks via its slave mode protocol.
// 
// As a consequence of this there is currently a restriction on the
// format of a playlist file. It must be UTF-8 "one file/URL per line"
// format or a .m3u8 file. This is because it is not passed through to
// MPlayer as a --playlist=... flag and is parsed instead by MPlayer-ARC,
// whose parsing is less sophisticated than MPlayer's.
// 
// The "Audio track" button within Android-VLC-Remote is repurposed to
// cycle through OSD modes.
// 
// The "Subtitle track" button is repurposed to rewind by 10
// seconds. This is convenient when somebody talks over a part of a film
// and you need to quickly rewind. The alternative of rewinding using the
// Android-VLC-Remote progress slider is quite fiddly.
// 
// The following features of Android-VLC-Remote are working:
// 
//   * Playing tab - All features (play, pause, stop, forward, back,
//     loop, repeat, volume, shuffle, fullscreen, aspect toggle etc).
// 
//   * Playlist tab - Selecting tracks works as normal.
// 
// The following features of Android-VLC-Remote do not work:
// 
//   * Library tab
// 
//   * DVD tab.
// 
//   * Metadata - The metadata passed through to the information box is
//     just the filename (as "title").
// 
// See also
// 
// mplayer(1)
package main
