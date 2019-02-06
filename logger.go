package rz

import (
	"os"
)

// A Logger represents an active logging object that generates lines
// of JSON output to an io.Writer. Each logging operation makes a single
// call to the Writer's Write method. There is no guaranty on access
// serialization to the Writer. If your Writer is not thread safe,
// you may consider a sync wrapper.
type Logger struct {
	writer  LevelWriter
	stack   bool
	caller  bool
	level   LogLevel
	sampler LogSampler
	context []byte
	hooks   []LogHook
}

// New creates a root logger with given options. If the output writer implements
// the LevelWriter interface, the WriteLevel method will be called instead of the Write
// one. Default writer is os.Stdout
//
// Each logging operation makes a single call to the Writer's Write method. There is no
// guaranty on access serialization to the Writer. If your Writer is not thread safe,
// you may consider using sync wrapper.
func New(options ...Option) Logger {
	logger := Logger{
		writer: levelWriterAdapter{os.Stdout},
	}
	return logger.Config(options...)
}

// Nop returns a disabled logger for which all operation are no-op.
func Nop() Logger {
	return New(Writer(nil), Level(Disabled))
}

// Config apply all the options to the logger
func (l Logger) Config(options ...Option) Logger {
	context := l.context
	l.context = make([]byte, 0, 500)
	if context != nil {
		l.context = append(l.context, context...)
	}
	for _, option := range options {
		option(&l)
	}
	return l
}

// Debug logs a new message with debug level.
func (l *Logger) Debug(message string, fields func(*Event)) {
	l.logEvent(DebugLevel, message, fields, nil)
}

// Info logs a new message with info level.
func (l *Logger) Info(message string, fields func(*Event)) {
	l.logEvent(InfoLevel, message, fields, nil)
}

// Warn logs a new message with warn level.
func (l *Logger) Warn(message string, fields func(*Event)) {
	l.logEvent(WarnLevel, message, fields, nil)
}

// Error logs a message with error level.
func (l *Logger) Error(message string, fields func(*Event)) {
	l.logEvent(ErrorLevel, message, fields, nil)
}

// Fatal logs a new message with fatal level. The os.Exit(1) function
// is then called, which terminates the program immediately.
func (l *Logger) Fatal(message string, fields func(*Event)) {
	l.logEvent(FatalLevel, message, fields, func(msg string) { os.Exit(1) })
}

// Panic logs a new message with panic level. The panic() function
// is then called, which stops the ordinary flow of a goroutine.
func (l *Logger) Panic(message string, fields func(*Event)) {
	l.logEvent(PanicLevel, message, fields, func(msg string) { panic(msg) })
}

// Log logs a new message with no level. Setting GlobalLevel to Disabled
// will still disable events produced by this method.
func (l *Logger) Log(message string, fields func(*Event)) {
	l.logEvent(NoLevel, message, fields, nil)
}

// Write implements the io.Writer interface. This is useful to set as a writer
// for the standard library log.
func (l Logger) Write(p []byte) (n int, err error) {
	n = len(p)
	if n > 0 && p[n-1] == '\n' {
		// Trim CR added by stdlog.
		p = p[0 : n-1]
	}
	l.Log(string(p), nil)
	return
}

func (l *Logger) logEvent(level LogLevel, message string, fields func(*Event), done func(string)) {
	enabled := l.should(level)
	if !enabled {
		return
	}
	e := newEvent(l.writer, level)
	e.done = done
	e.ch = l.hooks
	e.caller = l.caller
	e.stack = l.stack
	if level != NoLevel {
		e.String(LevelFieldName, level.String())
	}
	if l.context != nil && len(l.context) > 0 {
		e.buf = enc.AppendObjectData(e.buf, l.context)
	}

	if fields != nil {
		fields(e)
	}
	e.msg(message)
}

// should returns true if the log event should be logged.
func (l *Logger) should(lvl LogLevel) bool {
	if lvl < l.level || lvl < GlobalLevel() {
		return false
	}
	if l.sampler != nil && !samplingDisabled() {
		return l.sampler.Sample(lvl)
	}
	return true
}
