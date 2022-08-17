package main

import (
	"encoding/json"
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

	"golang.org/x/net/proxy"
)

func SetLogOutput(w io.Writer) {
	// Set logger writer
	log.SetOutput(w)
}

func NewVideo(debug bool) *Video {
	// Initialize youtube package object
	return &Video{Debug: debug, DownloadPercent: make(chan int64, 100)}
}

func NewVideoWithSocks5Proxy(debug bool, socks5Proxy string) *Video {
	return &Video{Debug: debug, DownloadPercent: make(chan int64, 100), Socks5Proxy: socks5Proxy}
}

type stream map[string]string

type Video struct {
	Debug             bool
	Id                string
	info              string
	StreamList        []stream
	DownloadPercent   chan int64
	Socks5Proxy       string
	contentLength     float64
	totalWrittenBytes float64
	downloadLevel     float64
}

func (v *Video) DecodeURL(url string) error {
	// Decode youtube URL to retrieval video information.
	err := v.findVideoId(url)
	if err != nil {
		return fmt.Errorf("findvideoID error=%s", err)
	}
	err = v.getVideoInfo()
	if err != nil {
		return fmt.Errorf("getVideoInfo error=%s", err)
	}
	err = v.parseVideoInfo()
	if err != nil {
		return fmt.Errorf("parse video info failed, err=%s", err)
	}
	return nil
}

func (v *Video) Download(destDir string) error {
	// Starting download video to specific address.
	//download highest resolution on [0]
	err := errors.New("Empty stream list")
	destFile := filepath.Join(destDir, v.StreamList[0]["title"])
	for _, val := range v.StreamList {
		url := val["url"]
		v.log(fmt.Sprintln("Download url=", url))
		v.log(fmt.Sprintln("Download to file=", destFile))
		err = v.videoDownloadWorker(destFile, url)
		if err == nil {
			break
		}
	}
	return err
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

func (v *Video) parseVideoInfo() error {
	answer, err := url.ParseQuery(v.info)
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
	// Get video title and author.
	title, author := getVideoTitleAuthor(answer)
	// Get video download link
	var streams []stream
	for streamPos, streamRaw := range streamsList {
		streamQry, err := url.ParseQuery(streamRaw)
		if err != nil {
			log.Println(fmt.Errorf("An error occured while decoding one of the video's stream's information: stream %d: %s", streamPos, err))
			continue
		}
		if _, ok := streamQry["quality"]; !ok {
			v.log(fmt.Sprintf("An empty video's stream's information: stream %d\n", streamPos))
			continue
		}
		streams = append(streams, stream{
			"quality": streamQry["quality"][0],
			"type":    streamQry["type"][0],
			"url":     streamQry["url"][0],
			"title":   title,
			"author":  author,
		})
		v.log(fmt.Sprintf("Stream found: quality '%s', format '%s'", streamQry["quality"][0], streamQry["type"][0]))
	}
	v.StreamList = streams
	if len(v.StreamList) == 0 {
		return errors.New(fmt.Sprint("no stream list found in the server's answer"))
	}
	return nil
}

func (v *Video) getVideoInfo() error {
	url := "http://youtube.com/get_video_info?video_id=" + v.Id // WRONG URL!
	v.log(fmt.Sprintf("url: %s", url))
	httpClient, err := v.getHTTPClient()
	if err != nil {
		return err
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
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

func (v *Video) getHTTPClient() (*http.Client, error) {
	// setup a http client
	httpTransport := &http.Transport{}
	httpClient := &http.Client{Transport: httpTransport}
	if len(v.Socks5Proxy) == 0 || v.Socks5Proxy == "0" {
		v.log("Using http without proxy.")
		return httpClient, nil
	}
	dialer, err := proxy.SOCKS5("tcp", v.Socks5Proxy, nil, proxy.Direct)
	if err != nil {
		fmt.Fprintln(os.Stderr, "can't connect to the proxy:", err)
		return nil, err
	}
	// set our socks5 as the dialer
	httpTransport.Dial = dialer.Dial
	v.log(fmt.Sprintf("Using http with proxy %s.", v.Socks5Proxy))
	return httpClient, nil
}

func (v *Video) videoDownloadWorker(destFile string, target string) error {
	httpClient, err := v.getHTTPClient()
	if err != nil {
		return err
	}
	resp, err := httpClient.Get(target)
	if err != nil {
		log.Printf("Http.Get\nerror: %s\ntarget: %s\n", err, target)
		return err
	}
	defer resp.Body.Close()
	v.contentLength = float64(resp.ContentLength)
	if resp.StatusCode != http.StatusOK {
		log.Printf("reading answer: non 200[code=%v] status code received: '%s'", resp.StatusCode, err)
		return errors.New("non 200 status code received")
	}
	err = os.MkdirAll(filepath.Dir(destFile), 0755)
	if err != nil {
		return err
	}
	out, err := os.Create(destFile)
	if err != nil {
		return err
	}
	mw := io.MultiWriter(out, v)
	_, err = io.Copy(mw, resp.Body)
	if err != nil {
		v.log(fmt.Sprintln("download video error: ", err))
		return err
	}
	return nil
}

func (v *Video) log(logText string) {
	if v.Debug {
		log.Println(logText)
	}
}

func getVideoTitleAuthor(in url.Values) (string, string) {
	playResponse, ok := in["player_response"]
	if !ok {
		return "", ""
	}
	personMap := make(map[string]interface{})

	if err := json.Unmarshal([]byte(playResponse[0]), &personMap); err != nil {
		panic(err)
	}

	s := personMap["videoDetails"]
	myMap := s.(map[string]interface{})
	if title, ok := myMap["title"]; ok {
		if author, ok := myMap["author"]; ok {
			return title.(string), author.(string)
		}
	}

	return "", ""
}
