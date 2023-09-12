package model

import (
	"log"
	"time"
)

var CreateTime = time.Now()

var Dev = true

func Printf(format string, v ...interface{}) {
	if Dev {
		log.Printf(format, v...)
	}
}
