package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type stream map[string]string

type Video struct {
	Id         string
	Info       string
	streamList []stream
}

func NewVideo() *Video {
	return new(Video)
}

func (v *Video) DecodeURL(url string) error {
	err := v.findVideoId(url)
	if err != nil {
		return fmt.Errorf("findvideoID error=%s", err)
	}
	err = v.getVideoInfo()
	if err != nil {
		return fmt.Errorf("getVideoInfo error=%s", err)
	}
	err = v.parseVidoInfo()
	if err != nil {
		return fmt.Errorf("parse video info failed, err=%s", err)
	}
	return nil
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

func (v *Video) parseVidoInfo() error {
	var streams []stream
	answer, err := url.ParseQuery(v.Id)
	if err != nil {
		return err
	}
	status, ok := answer["status"]
	if !ok {
		return errors.New("No response status found in the server's answer")
	}
	if status[0] == "fail" {
		reason, ok := answer["reason"]
		if ok {
			return fmt.Errorf("'fail' response status found in the server's answer, reason: %s" + reason[0])
		} else {
			return errors.New("'fail' response status found in the server's answer, no reason given")
		}
	}
	if status[0] != "ok" {
		return fmt.Errorf("non-success response status found in the server's answer (status: '%s')", status)
	}
	// Read the sreams map
	stream_map, ok := answer["url_encoded_fmt_stream_map"]
	if !ok {
		return errors.New("no stream map found in the server's answer")
	}
	// Read each stream
	streams_list := strings.Split(stream_map[0], ",")
	for stream_pos, stream_raw := range streams_list {
		stream_qry, err := url.ParseQuery(stream_raw)
		if err != nil {
			fmt.Errorf("An error occured while decoding one of the video's stream's information: stream %d: %s", stream_pos, err)
			continue
		}
		var sig string
		if _, exist := stream_qry["sig"]; exist {
			sig = stream_qry["sig"][0]
		}
		stream := stream{
			"quality": stream_qry["quality"][0],
			"type":    stream_qry["type"][0],
			"url":     stream_qry["url"][0],
			"sig":     sig,
			"title":   answer["title"][0],
			"author":  answer["author"][0],
		}
		streams = append(streams, stream)
		log.Printf("Stream found: quality '%s', format '%s'", stream_qry["quality"][0], stream_qry["type"][0])
	}
	v.streamList = streams
	return nil
}

func (v *Video) getVideoInfo() error {
	url := "http://youtube.com/get_video_info?video_id=" + v.Id
	log.Printf("url: %s/n", url)
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	v.Info = string(body)
	return nil
}
