package main

import (
	"flag"
	"log"
)

var URL string

func main() {
	flag.StringVar(&URL, "url", "", "YouTube video url")
	flag.Parse()
	if URL == "" {
		log.Panic("ERROR! Wrong URL!")
	}
}
