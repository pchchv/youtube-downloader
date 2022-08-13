package main

type stream map[string]string

type Video struct {
	Id     string
	Info   string
	stream []stream
}

func NewVideo() *Video {
	return new(Video)
}
