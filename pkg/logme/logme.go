package logme

import (
	"log"
	"os"
)

var infoLogger = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime|log.Lshortfile)
var debugLogger = log.New(os.Stdout, "[DEBUG] ", log.Ldate|log.Ltime|log.Lshortfile)
var errorLogger = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)

var isDebugMode bool = os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true"
var silentMode bool = os.Getenv("SILENT") == "1" || os.Getenv("GITHUB_ACTIONS") == "true"

func DebugF(msg string, args ...interface{}) {
	if isDebugMode {
		debugLogger.Printf(msg, args...)
	}
}

func Debugln(args ...interface{}) {
	// check if ENV DEBUG is 1
	if isDebugMode {
		debugLogger.Println(args...)
	}
}

func InfoF(msg string, args ...interface{}) {
	if !silentMode {
		infoLogger.Printf(msg, args...)
	}
}

func Infoln(arg ...interface{}) {
	if !silentMode {
		infoLogger.Println(arg...)
	}
}

func ErrorF(msg string, args ...interface{}) {
	if !silentMode {
		errorLogger.Printf(msg, args...)
	}
}

func Errorln(arg ...interface{}) {
	if !silentMode {
		errorLogger.Println(arg...)
	}
}

func FatalLn(arg ...interface{}) {
	errorLogger.Fatalln(arg...)
}

func FatalF(msg string, args ...interface{}) {
	errorLogger.Fatalf(msg, args...)
}
