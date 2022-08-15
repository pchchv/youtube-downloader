package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func SetLogOutput(w io.Writer) {
	log.SetOutput(w)
}

func NewVideo(debug bool) *Video {
	return &Video{Debug: debug, DownloadPercent: make(chan int64, 100)}
}

type stream map[string]string

type Video struct {
	Debug             bool
	Id                string
	Info              string
	StreamList        []stream
	DownloadPercent   chan int64
	ContentLength     float64
	TotalWrittenBytes float64
	DownloadLevel     float64
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

func (v *Video) Download(dstDir string) error {
	//download highest resolution on [0]
	targetStream := v.StreamList[0]
	url := targetStream["url"] + "&signature=" + targetStream["sig"]
	v.log("Download url = " + url)
	v.log(fmt.Sprintf("Download to file=%s", dstDir))
	err := v.videoDownloadWorker(dstDir, url)
	if err != nil {
		return err
	}
	return nil
}

func (v *Video) findVideoId(url string) error {
	id := url
	if strings.Contains(id, "youtu") || strings.ContainsAny(id, "\"?&/<%=") {
		re_list := []*regexp.Regexp{
			regexp.MustCompile(`(?:v|embed|watch\?v)(?:=|/)([^"&?/=%]{11})`),
			regexp.MustCompile(`(?:=|/)([^"&?/=%]{11})`),
			regexp.MustCompile(`([^"&?/=%]{11})`),
		}
		for _, re := range re_list {
			if is_match := re.MatchString(id); is_match {
				subs := re.FindStringSubmatch(id)
				id = subs[1]
			}
		}
	}
	log.Println("Found video id — " + id)
	v.Id = id
	if strings.ContainsAny(id, "?&/<%=") {
		return errors.New("invalid characters in video id")
	}
	if len(id) < 10 {
		return errors.New("the video id must be at least 10 characters long")
	}
	return nil
}

func (v *Video) parseVidoInfo() error {
	var streams []stream
	answer, err := url.ParseQuery(v.Info) // ERROR: Empty answer!
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
			log.Println(fmt.Errorf("An error occured while decoding one of the video's stream's information: stream %d: %s", stream_pos, err))
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
		v.log(fmt.Sprintf("Stream found: quality '%s', format '%s'", stream_qry["quality"][0], stream_qry["type"][0]))
	}
	v.StreamList = streams
	return nil
}

func (v *Video) getVideoInfo() error {
	url := "http://youtube.com/get_video_info?video_id=" + v.Id
	v.log(fmt.Sprintf("url: %s", url))
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

func (v *Video) Write(p []byte) (n int, err error) {
	n = len(p)
	v.TotalWrittenBytes += float64(n)
	currentPercent := (v.TotalWrittenBytes / v.ContentLength) * 100
	if v.DownloadLevel <= currentPercent && v.DownloadLevel < 100 {
		v.DownloadLevel++
		v.DownloadPercent <- int64(v.DownloadLevel)
	}
	return
}

func (v *Video) videoDownloadWorker(dstFile string, target string) error {
	res, err := http.Get(target)
	if err != nil {
		log.Printf("Http.Get\nerror: %s\ntarget: %s\n", err, target)
		return err
	}
	defer res.Body.Close()
	v.ContentLength = float64(res.ContentLength)
	if res.StatusCode != http.StatusOK {
		log.Printf("reading answer: non 200[code=%v] status code received: '%s'", res.StatusCode, err)
		return errors.New("non 200 status code received")
	}
	err = os.MkdirAll(filepath.Dir(dstFile), 666)
	if err != nil {
		return err
	}
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	mw := io.MultiWriter(out, v)
	_, err = io.Copy(mw, res.Body)
	if err != nil {
		log.Println("download video error: ", err)
		return err
	}
	return nil
}

func (v *Video) log(logText string) {
	if v.Debug {
		log.Println(logText)
	}
}
