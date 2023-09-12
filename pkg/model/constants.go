package model

import (
	"github.com/rs/zerolog/log"
	"os"
	"time"
)

var CreateTime = time.Now()

var Dev = os.Getenv("dev") == "true"

func Printf(format string, v ...interface{}) {
	if Dev {
		log.Printf(format, v...)
	}
}

func ReqLog(reqId, metaKey, instanceId, msg string, dur int64) {
	if Dev {
		log.Info().Str("key", metaKey).Str("it", instanceId).
			Str("req", reqId).Int64("dur", dur).Msg(msg)
	}
}
