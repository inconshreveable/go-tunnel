package log

import (
	"fmt"

	log "github.com/alecthomas/log4go"
)

var root log.Logger = make(log.Logger)

func LogTo(target string) {
	var writer log.LogWriter = nil

	switch target {
	case "stdout":
		writer = log.NewConsoleLogWriter()
	case "none":
		// no logging
	default:
		writer = log.NewFileLogWriter(target, true)
	}

	if writer != nil {
		root.AddFilter("log", log.DEBUG, writer)
	}
}

type Logger interface {
	AddTags(...string)
	SetTags(...string)
	Name() string
	Debug(string, ...interface{})
	Info(string, ...interface{})
	Warn(string, ...interface{}) error
	Error(string, ...interface{}) error
}

type TaggedLogger struct {
	*log.Logger
	tags   []string
	prefix string
}

func NewTaggedLogger(tags ...string) Logger {
	l := &TaggedLogger{Logger: &root}
	l.SetTags(tags...)
	return l
}

func (l *TaggedLogger) pfx(fmtstr string) interface{} {
	return fmt.Sprintf("%s%s", l.prefix, fmtstr)
}

func (l *TaggedLogger) Debug(arg0 string, args ...interface{}) {
	l.Logger.Debug(l.pfx(arg0), args...)
}

func (l *TaggedLogger) Info(arg0 string, args ...interface{}) {
	l.Logger.Info(l.pfx(arg0), args...)
}

func (l *TaggedLogger) Warn(arg0 string, args ...interface{}) error {
	return l.Logger.Warn(l.pfx(arg0), args...)
}

func (l *TaggedLogger) Error(arg0 string, args ...interface{}) error {
	return l.Logger.Error(l.pfx(arg0), args...)
}

func (l *TaggedLogger) AddTags(tags ...string) {
	l.SetTags(append(l.tags, tags...)...)
}

func (l *TaggedLogger) SetTags(tags ...string) {
	l.tags = tags
	l.prefix = ""
	for _, t := range tags {
		l.prefix += fmt.Sprintf("[%s] ", t)
	}
}

func (l *TaggedLogger) Name() string {
	return l.prefix[:len(l.prefix)-1]
}

// we should never really use these . . . always prefer logging through a prefix logger
func Debug(arg0 string, args ...interface{}) {
	root.Debug(arg0, args...)
}

func Info(arg0 string, args ...interface{}) {
	root.Info(arg0, args...)
}

func Warn(arg0 string, args ...interface{}) error {
	return root.Warn(arg0, args...)
}

func Error(arg0 string, args ...interface{}) error {
	return root.Error(arg0, args...)
}
