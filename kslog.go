package kslog

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

var logging = NewLogger()

type loglevel uint8

const (
	EMERGE loglevel = iota
	ALERT
	CRIT
	ERROR
	WARNING
	NOTICE
	INFO
	DEBUG
	DEBUG2
	MAXLEVEL = 9
)

type logger struct {
	sink  chan *logItem
	level loglevel
	file  *os.File
}

func NewLogger() *logger {
	var err error = nil

	l := new(logger)
	l.sink = make(chan *logItem, 1000)
	l.level = DEBUG2

	name := logName(time.Now())
	logpath := "/var/log/kslog/" + getProgram()
	os.MkdirAll(logpath, 770)

	l.file, err = os.Create(logpath + "/" + name)
	if err != nil {
		fmt.Println("Error oppening file for logging", err)
	}

	go l.sinkLoop()

	return l
}

func getProgram() string {
	progpath := os.Args[0]
	progpath = strings.Replace(progpath, "\\", "/", -1)
	program := path.Base(progpath)

	return program
}

func logName(t time.Time) string {
	name := fmt.Sprintf("%s.log.%04d%02d%02d-%02d%02d%02d.%d",
		getProgram(),
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
		os.Getpid())
	return name
}

type logItem struct {
	message *string
	args    *map[string]interface{}
	level   loglevel
	line    int
	file    *string
	module  *string
	code    int32
}

func map2str(args *map[string]interface{}) string {
	buf := bytes.NewBuffer(nil)

	for k, v := range *args {
		buf.WriteString(fmt.Sprintf("[ %s: %v ] ", k, v))
	}
	return buf.String()
}

func args2map(args ...interface{}) (*map[string]interface{}, error) {
	argsMap := make(map[string]interface{})
	key := "_unknown"
	if argsLen := len(args); argsLen > 0 {
		if argsLen%2 != 0 {
			return nil, errors.New("Bad key value match")
		}
		for argNum := 0; argNum < argsLen; argNum++ {
			arg := args[argNum]
			switch argNum % 2 {
			case 1:
				argsMap[key] = arg
			case 0:
				if arg != nil {
					if strArg, ok := arg.(string); ok {
						key = strArg
					} else {
						return nil, errors.New("Key is not a string")
					}
				}
			}
		}
	}
	return &argsMap, nil
}
func getCaller(depth int) (*string, int) {
	_, file, line, ok := runtime.Caller(4)
	if !ok {
		file = "???"
		line = 1
	} else {
		slash := strings.LastIndex(file, "/")
		if slash >= 0 {
			file = file[slash+1:]
		}
	}

	return &file, line
}

func (this *logger) output(level loglevel, code int32, module *string, message *string, depth int, args ...interface{}) {
	file, line := getCaller(4)
	argMap, err := args2map(args...)
	if err != nil {
		log.Printf("ERROR: %s at %s:%d", err.Error(), file, line)
	}

	item := &logItem{
		message: message,
		level:   level,
		module:  module,
		line:    line,
		file:    file,
		code:    code,
		args:    argMap,
	}

	this.sink <- item
}

func (this *logger) sinkLoop() {
	for {
		select {
		case li := <-this.sink:
			this.sinkLogItem(li)
			this.sinkLogItemToFile(li)
		}
	}
}

func (this *logger) sinkLogItem(li *logItem) {
	s := fmt.Sprintf("%d: %s:%d %d", li.level, *li.file, li.line, li.code)
	//fmt.Printf("%d: %s:%d %d : \"%s\" %s\n", li.level, *li.file, li.line, li.code, *li.message, map2str(li.args))
	fmt.Printf("%-30s : %s %s\n", s, *li.message, map2str(li.args))
}

func (this *logger) sinkLogItemToFile(li *logItem) {
	out := fmt.Sprintf("%d: %s:%d %d : \"%s\" %s\n", li.level, *li.file, li.line, li.code, *li.message, map2str(li.args))
	this.file.WriteString(out)
}

func (this *logger) print(level loglevel, module *string, code int32, args ...interface{}) {
	if this.level >= level {
		buf := new(bytes.Buffer)
		fmt.Fprint(buf, args...)
		str := buf.String()
		this.output(level, code, module, &str, 0)
	}
}

func (this *logger) printex(level loglevel, module *string, code int32, message *string, args ...interface{}) {
	if this.level >= level {
		this.output(level, code, module, message, 0, args...)
	}
}

func (this *logger) printf(level loglevel, module *string, code int32, format string, args ...interface{}) {
	if this.level >= level {
		buf := new(bytes.Buffer)
		fmt.Fprintf(buf, format, args...)
		str := buf.String()
		this.output(level, code, module, &str, 0)
	}
}

// Emergef logs to the EMERGE log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Emergef(module string, code int32, format string, args ...interface{}) {
	logging.printf(EMERGE, &module, code, format, args...)
}

// Emerge logs to the EMERGE log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Emerge(module string, code int32, args ...interface{}) {
	logging.print(EMERGE, &module, code, args...)
}

// Emerge logs to the EMERGE log.
// Argument are string and anonymous struct
func EmergeEx(module string, code int32, message string, args ...interface{}) {
	logging.printex(EMERGE, &module, code, &message, args...)
}

// Errorf logs to the ERROR log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Errorf(module string, code int32, format string, args ...interface{}) {
	logging.printf(ERROR, &module, code, format, args...)
}

// Error logs to the ERROR log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Error(module string, code int32, args ...interface{}) {
	logging.print(ERROR, &module, code, args...)
}

// Error logs to the ERROR log.
// Argument are string and anonymous struct
func ErrorEx(module string, code int32, message string, args ...interface{}) {
	logging.printex(ERROR, &module, code, &message, args...)
}

// Noticef logs to the NOTICE log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Noticef(module string, code int32, format string, args ...interface{}) {
	logging.printf(NOTICE, &module, code, format, args...)
}

// Notice logs to the NOTICE log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Notice(module string, code int32, args ...interface{}) {
	logging.print(NOTICE, &module, code, args...)
}

// Notice logs to the NOTICE log.
// Argument are string and anonymous struct
func NoticeEx(module string, code int32, message string, args ...interface{}) {
	logging.printex(NOTICE, &module, code, &message, args...)
}

// Infof logs to the INFO log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Infof(module string, code int32, format string, args ...interface{}) {
	logging.printf(INFO, &module, code, format, args...)
}

// Info logs to the INFO log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Info(module string, code int32, args ...interface{}) {
	logging.print(INFO, &module, code, args...)
}

// Info logs to the INFO log.
// Argument are string and anonymous struct
func InfoEx(module string, code int32, message string, args ...interface{}) {
	logging.printex(INFO, &module, code, &message, args...)
}

// Debugf logs to the DEBUG log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Debugf(module string, code int32, format string, args ...interface{}) {
	logging.printf(DEBUG, &module, code, format, args...)
}

// Debug logs to the DEBUG log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Debug(module string, code int32, args ...interface{}) {
	logging.print(DEBUG, &module, code, args...)
}

// Debug logs to the DEBUG log.
// Argument are string and anonymous struct
func DebugEx(module string, code int32, message string, args ...interface{}) {
	logging.printex(DEBUG, &module, code, &message, args...)
}
