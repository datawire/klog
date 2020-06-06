// Go support for leveled logs, analogous to https://code.google.com/p/google-glog/
//
// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package klog implements logging analogous to the Google-internal C++ INFO/ERROR/V setup.
// It provides functions Info, Warning, Error, Fatal, plus formatting variants such as
// Infof. It also provides V-style logging controlled by the -v and -vmodule=file=2 flags.
//
// Basic examples:
//
//	klog.Info("Prepare to repel boarders")
//
//	klog.Fatalf("Initialization failed: %s", err)
//
// See the documentation for the V function for an explanation of these examples:
//
//	if klog.V(2) {
//		klog.Info("Starting transaction...")
//	}
//
//	klog.V(2).Infoln("Processed", nItems, "elements")
//
// Log output is buffered and written periodically using Flush. Programs
// should call Flush before exiting to guarantee all log output is written.
//
// By default, all log statements write to standard error.
// This package provides several flags that modify this behavior.
// As a result, flag.Parse must be called before any logging is done.
//
//	-logtostderr=true
//		Logs are written to standard error instead of to files.
//	-alsologtostderr=false
//		Logs are written to standard error as well as to files.
//	-stderrthreshold=ERROR
//		Log events at or above this severity are logged to standard
//		error as well as to files.
//	-log_dir=""
//		Log files will be written to this directory instead of the
//		default temporary directory.
//
//	Other flags provide aids to debugging.
//
//	-log_backtrace_at=""
//		When set to a file and line number holding a logging statement,
//		such as
//			-log_backtrace_at=gopherflakes.go:234
//		a stack trace will be written to the Info log whenever execution
//		hits that statement. (Unlike with -vmodule, the ".go" must be
//		present.)
//	-v=0
//		Enable V-leveled logging at the specified level.
//	-vmodule=""
//		The syntax of the argument is a comma-separated list of pattern=N,
//		where pattern is a literal file name (minus the ".go" suffix) or
//		"glob" pattern and N is a V level. For instance,
//			-vmodule=gopher*=3
//		sets the V level to 3 in all Go files whose names begin "gopher".
//
package klog

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	klogv2 "k8s.io/klog/v2"
)

// OutputStats tracks the number of output lines and bytes written.
type OutputStats = klogv2.OutputStats

// Stats tracks the number of lines of output and number of bytes
// per severity level. Values must be read with atomic.LoadInt64.
var Stats = &klogv2.Stats

// Level specifies a level of verbosity for V logs. *Level implements
// flag.Value; the -v flag is of type Level and should be modified
// only through the flag.Value interface.
type Level = klogv2.Level

var global struct {
	vModuleMu     sync.Mutex        // governs access to everything below
	vModuleFilter []modulePat       // from arg parsing
	vModuleCache  map[uintptr]Level // cache of pc->Level
}

// modulePat contains a filter for the -vmodule flag.
// It holds a verbosity level and a file pattern to match.
type modulePat struct {
	pattern string
	literal bool // The pattern is a literal string
	level   Level
}

// match reports whether the file matches the pattern. It uses a string
// comparison if the pattern contains no metacharacters.
func (m *modulePat) match(file string) bool {
	if m.literal {
		return file == m.pattern
	}
	match, _ := filepath.Match(m.pattern, file)
	return match
}

type vmoduleValue struct {
	inner flag.Value
}

func (m *vmoduleValue) String() string {
	return m.inner.String()
}

// Syntax: -vmodule=recordio=2,file=1,gfs*=3
func (m *vmoduleValue) Set(value string) error {
	if err := m.inner.Set(value); err != nil {
		return err
	}
	var filter []modulePat
	for _, pat := range strings.Split(value, ",") {
		if len(pat) == 0 {
			// Empty strings such as from a trailing comma can be ignored.
			continue
		}
		patLev := strings.Split(pat, "=")
		pattern := patLev[0]
		v, _ := strconv.ParseInt(patLev[1], 10, 32)
		if v == 0 {
			continue // Ignore. It's harmless but no point in paying the overhead.
		}
		// TODO: check syntax of filter?
		filter = append(filter, modulePat{pattern, isLiteral(pattern), Level(v)})
	}
	global.vModuleMu.Lock()
	defer global.vModuleMu.Unlock()
	global.vModuleFilter = filter
	global.vModuleCache = make(map[uintptr]Level)
	return nil
}

// isLiteral reports whether the pattern is a literal string, that is, has no metacharacters
// that require filepath.Match to be called to match the pattern.
func isLiteral(pattern string) bool {
	return !strings.ContainsAny(pattern, `\*?[]`)
}

// InitFlags is for explicitly initializing the flags.
func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	klogv2.InitFlags(flagset)

	vmoduleFlag := flagset.Lookup("vmodule")
	vmoduleFlag.Value = &vmoduleValue{inner: vmoduleFlag.Value}
}

// Flush flushes all pending log I/O.
func Flush() {
	klogv2.Flush()
}

// SetOutput sets the output destination for all severities
func SetOutput(w io.Writer) {
	klogv2.SetOutput(w)
}

// SetOutputBySeverity sets the output destination for specific severity
func SetOutputBySeverity(name string, w io.Writer) {
	klogv2.SetOutputBySeverity(name, w)
}

// CalculateMaxSize returns the real max size in bytes after considering the default max size and the flag options.
func CalculateMaxSize() uint64 {
	return klogv2.CalculateMaxSize()
}

// CopyStandardLogTo arranges for messages written to the Go "log" package's
// default logs to also appear in the Google logs for the named and lower
// severities.  Subsequent changes to the standard log's default output location
// or format may break this behavior.
//
// Valid names are "INFO", "WARNING", "ERROR", and "FATAL".  If the name is not
// recognized, CopyStandardLogTo panics.
func CopyStandardLogTo(name string) {
	klogv2.CopyStandardLogTo(name)
}

// Verbose is a boolean type that implements Infof (like Printf) etc.
// See the documentation of V for more information.
type Verbose bool

// V reports whether verbosity at the call site is at least the requested level.
// The returned value is a boolean of type Verbose, which implements Info, Infoln
// and Infof. These methods will write to the Info log if called.
// Thus, one may write either
//	if klog.V(2) { klog.Info("log this") }
// or
//	klog.V(2).Info("log this")
// The second form is shorter but the first is cheaper if logging is off because it does
// not evaluate its arguments.
//
// Whether an individual call to V generates a log record depends on the setting of
// the -v and --vmodule flags; both are off by default. If the level in the call to
// V is at least the value of -v, or of -vmodule for the source file containing the
// call, the V call will log.
func V(level Level) Verbose {
	if klogv2.V(level).Enabled() {
		return Verbose(true)
	}

	var pcs [1]uintptr
	if runtime.Callers(2, pcs[:]) == 0 {
		return Verbose(false)
	}

	verbosity := vmoduleGet(pcs[0])
	return Verbose(verbosity >= level)
}

// vmoduleGet takes the given program-counter and returns the loglevel
// of the associated "-vmodule=" rule; if no vmodule rule matches,
// then "0" is returned.
func vmoduleGet(pc uintptr) Level {
	global.vModuleMu.Lock()
	defer global.vModuleMu.Unlock()
	if v, ok := global.vModuleCache[pc]; ok {
		return v
	}

	fn := runtime.FuncForPC(pc)
	file, _ := fn.FileLine(pc)
	// The file is something like /a/b/c/d.go. We want just the d.
	if strings.HasSuffix(file, ".go") {
		file = file[:len(file)-3]
	}
	if slash := strings.LastIndex(file, "/"); slash >= 0 {
		file = file[slash+1:]
	}
	for _, filter := range global.vModuleFilter {
		if filter.match(file) {
			global.vModuleCache[pc] = filter.level
			return filter.level
		}
	}
	global.vModuleCache[pc] = 0
	return 0
}

// Info is equivalent to the global Info function, guarded by the value of v.
// See the documentation of V for usage.
func (v Verbose) Info(args ...interface{}) {
	if v {
		klogv2.InfoDepth(1, fmt.Sprint(args...))
	}
}

// Infoln is equivalent to the global Infoln function, guarded by the value of v.
// See the documentation of V for usage.
func (v Verbose) Infoln(args ...interface{}) {
	if v {
		klogv2.InfoDepth(1, fmt.Sprintln(args...))
	}
}

// Infof is equivalent to the global Infof function, guarded by the value of v.
// See the documentation of V for usage.
func (v Verbose) Infof(format string, args ...interface{}) {
	if v {
		klogv2.InfoDepth(1, fmt.Sprintf(format, args...))
	}
}

// Info logs to the INFO log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Info(args ...interface{}) {
	klogv2.InfoDepth(1, fmt.Sprint(args...))
}

// InfoDepth acts as Info but uses depth to determine which call frame to log.
// InfoDepth(0, "msg") is the same as Info("msg").
func InfoDepth(depth int, args ...interface{}) {
	klogv2.InfoDepth(depth+1, fmt.Sprint(args...))
}

// Infoln logs to the INFO log.
// Arguments are handled in the manner of fmt.Println; a newline is always appended.
func Infoln(args ...interface{}) {
	klogv2.InfoDepth(1, fmt.Sprintln(args...))
}

// Infof logs to the INFO log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Infof(format string, args ...interface{}) {
	klogv2.InfoDepth(1, fmt.Sprintf(format, args...))
}

// Warning logs to the WARNING and INFO logs.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Warning(args ...interface{}) {
	klogv2.WarningDepth(1, fmt.Sprint(args...))
}

// WarningDepth acts as Warning but uses depth to determine which call frame to log.
// WarningDepth(0, "msg") is the same as Warning("msg").
func WarningDepth(depth int, args ...interface{}) {
	klogv2.WarningDepth(depth+1, fmt.Sprint(args...))
}

// Warningln logs to the WARNING and INFO logs.
// Arguments are handled in the manner of fmt.Println; a newline is always appended.
func Warningln(args ...interface{}) {
	klogv2.WarningDepth(1, fmt.Sprintln(args...))
}

// Warningf logs to the WARNING and INFO logs.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Warningf(format string, args ...interface{}) {
	klogv2.WarningDepth(1, fmt.Sprintf(format, args...))
}

// Error logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Error(args ...interface{}) {
	klogv2.ErrorDepth(1, fmt.Sprint(args...))
}

// ErrorDepth acts as Error but uses depth to determine which call frame to log.
// ErrorDepth(0, "msg") is the same as Error("msg").
func ErrorDepth(depth int, args ...interface{}) {
	klogv2.ErrorDepth(depth+1, fmt.Sprint(args...))
}

// Errorln logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled in the manner of fmt.Println; a newline is always appended.
func Errorln(args ...interface{}) {
	klogv2.ErrorDepth(1, fmt.Sprintln(args...))
}

// Errorf logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Errorf(format string, args ...interface{}) {
	klogv2.ErrorDepth(1, fmt.Sprintf(format, args...))
}

// Fatal logs to the FATAL, ERROR, WARNING, and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Fatal(args ...interface{}) {
	klogv2.FatalDepth(1, fmt.Sprint(args...))
}

// FatalDepth acts as Fatal but uses depth to determine which call frame to log.
// FatalDepth(0, "msg") is the same as Fatal("msg").
func FatalDepth(depth int, args ...interface{}) {
	klogv2.FatalDepth(depth+1, fmt.Sprint(args...))
}

// Fatalln logs to the FATAL, ERROR, WARNING, and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Println; a newline is always appended.
func Fatalln(args ...interface{}) {
	klogv2.FatalDepth(1, fmt.Sprintln(args...))
}

// Fatalf logs to the FATAL, ERROR, WARNING, and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Fatalf(format string, args ...interface{}) {
	klogv2.FatalDepth(1, fmt.Sprintf(format, args...))
}

// fatalNoStacks is non-zero if we are to exit without dumping goroutine stacks.
// It allows Exit and relatives to use the Fatal logs.
var fatalNoStacks uint32

// Exit logs to the FATAL, ERROR, WARNING, and INFO logs, then calls os.Exit(1).
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Exit(args ...interface{}) {
	klogv2.ExitDepth(1, fmt.Sprint(args...))
}

// ExitDepth acts as Exit but uses depth to determine which call frame to log.
// ExitDepth(0, "msg") is the same as Exit("msg").
func ExitDepth(depth int, args ...interface{}) {
	klogv2.ExitDepth(depth+1, fmt.Sprint(args...))
}

// Exitln logs to the FATAL, ERROR, WARNING, and INFO logs, then calls os.Exit(1).
func Exitln(args ...interface{}) {
	klogv2.ExitDepth(1, fmt.Sprintln(args...))
}

// Exitf logs to the FATAL, ERROR, WARNING, and INFO logs, then calls os.Exit(1).
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Exitf(format string, args ...interface{}) {
	klogv2.ExitDepth(1, fmt.Sprintf(format, args...))
}
