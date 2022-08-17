package main

type Video struct {
	ID   string
	Itag int  // Video format by Itag number
	mp3  bool // Extract MP3 audio using ffmpeg
}
