package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

func LoggingSettings(logFile string) {
	dir := fmt.Sprintf("log/%s", time.Now().Format("2006-01-02"))
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0777)
	}

	logfile, _ := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	multiLogFile := io.MultiWriter(os.Stdout, logfile)
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetOutput(multiLogFile)
}
