package main

import (
	"flag"
	"log"
)

var URL string

func main() {
	flag.StringVar(&URL, "url", "", "")
	flag.Parse()
	if URL == "" {
		log.Panicln("ERROR! Wrong URL!")
	}
}
