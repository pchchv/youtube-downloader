package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

var (
	URL        string
	currentDir string
)

func init() {
	// Load values from .env into the system
	if err := godotenv.Load("../.env"); err != nil {
		log.Panic("No .env file found")
	}
}

func getEnvValue(v string) string {
	// Getting a value. Outputs a panic if the value is missing.
	value, exist := os.LookupEnv(v)
	if !exist {
		log.Panicf("Value %v does not exist", v)
	}
	return value
}

func main() {
	var err error
	currentDir, err = filepath.Abs(filepath.Dir(getEnvValue("DIR")))
	if err != nil {
		log.Panic(err)
	}
	log.Println("Download to dir =", currentDir)
	flag.StringVar(&URL, "url", "", "")
	flag.Parse()
	if URL == "" {
		log.Panicln("ERROR! Wrong URL!")
	}
	v := NewVideo(true)
	err = v.DecodeURL(URL)
	if err != nil {
		log.Panic(err)
	}
	err = v.Download(currentDir)
	if err != nil {
		log.Panic(err)
	}
}
