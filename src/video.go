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
	info              string
	StreamList        []stream
	DownloadPercent   chan int64
	contentLength     float64
	totalWrittenBytes float64
	downloadLevel     float64
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

func (v *Video) Download(destDir string) error {
	//download highest resolution on [0]
	targetStream := v.StreamList[0]
	url := targetStream["url"] + "&signature=" + targetStream["sig"]
	v.log("Download url = " + url)
	v.log(fmt.Sprintf("Download to file=%s", destDir))
	err := v.videoDownloadWorker(destDir, url)
	if err != nil {
		return err
	}
	return nil
}

func (v *Video) findVideoId(url string) error {
	id := url
	if strings.Contains(id, "youtu") || strings.ContainsAny(id, "\"?&/<%=") {
		reList := []*regexp.Regexp{
			regexp.MustCompile(`(?:v|embed|watch\?v)(?:=|/)([^"&?/=%]{11})`),
			regexp.MustCompile(`(?:=|/)([^"&?/=%]{11})`),
			regexp.MustCompile(`([^"&?/=%]{11})`),
		}
		for _, re := range reList {
			if isMatch := re.MatchString(id); isMatch {
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
	answer, err := url.ParseQuery(v.info) // ERROR: Empty answer!
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
	streamMap, ok := answer["url_encoded_fmt_stream_map"]
	if !ok {
		return errors.New("no stream map found in the server's answer")
	}
	// Read each stream
	streamsList := strings.Split(streamMap[0], ",")
	for streamPos, streamRaw := range streamsList {
		streamQry, err := url.ParseQuery(streamRaw)
		if err != nil {
			log.Println(fmt.Errorf("An error occured while decoding one of the video's stream's information: stream %d: %s", streamPos, err))
			continue
		}
		var sig string
		if _, exist := streamQry["sig"]; exist {
			sig = streamQry["sig"][0]
		}
		stream := stream{
			"quality": streamQry["quality"][0],
			"type":    streamQry["type"][0],
			"url":     streamQry["url"][0],
			"sig":     sig,
			"title":   answer["title"][0],
			"author":  answer["author"][0],
		}
		streams = append(streams, stream)
		v.log(fmt.Sprintf("Stream found: quality '%s', format '%s'", streamQry["quality"][0], streamQry["type"][0]))
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
	v.info = string(body)
	return nil
}

func (v *Video) Write(p []byte) (n int, err error) {
	n = len(p)
	v.totalWrittenBytes += float64(n)
	currentPercent := (v.totalWrittenBytes / v.contentLength) * 100
	if v.downloadLevel <= currentPercent && v.downloadLevel < 100 {
		v.downloadLevel++
		v.DownloadPercent <- int64(v.downloadLevel)
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
	v.contentLength = float64(res.ContentLength)
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
