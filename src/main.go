package main

import (
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	URL        string
	CurrentDir string
)

func init() {
	// Load values from .env into the system
	if err := godotenv.Load(); err != nil {
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
	CurrentDir = getEnvValue("DIR")
	flag.StringVar(&URL, "url", "", "")
	flag.Parse()
	if URL == "" {
		log.Panicln("ERROR! Wrong URL!")
	}
}
