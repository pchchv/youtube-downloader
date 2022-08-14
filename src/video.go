package main

import (
	"errors"
	"log"
	"regexp"
	"strings"
)

type stream map[string]string

type Video struct {
	Id     string
	Info   string
	stream []stream
}

func NewVideo() *Video {
	return new(Video)
}

func (v *Video) findVideoId(url string) error {
	var videoId string
	if strings.Contains(url, "youtu") || strings.ContainsAny(url, "\"?&/<%=") {
		re_list := []*regexp.Regexp{
			regexp.MustCompile(`(?:v|embed|watch\?v)(?:=|/)([^"&?/=%]{11})`),
			regexp.MustCompile(`(?:=|/)([^"&?/=%]{11})`),
			regexp.MustCompile(`([^"&?/=%]{11})`),
		}
		for _, re := range re_list {
			if is_match := re.MatchString(url); is_match {
				subs := re.FindStringSubmatch(url)
				videoId = subs[1]
			}
		}
	}
	log.Println("Found video id — " + videoId)
	v.Id = videoId
	if strings.ContainsAny(videoId, "?&/<%=") {
		return errors.New("invalid characters in video id")
	}
	if len(videoId) < 10 {
		return errors.New("the video id must be at least 10 characters long")
	}
	return nil
}
