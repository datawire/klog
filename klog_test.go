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

package klog_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	stdLog "log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	. "k8s.io/klog"
	klogv2 "k8s.io/klog/v2"
)

// TODO: This test package should be refactored so that tests cannot
// interfere with each-other, and don't require the mutex in testSetup.

// flushBuffer wraps a bytes.Buffer to satisfy flushSyncWriter.
type flushBuffer struct {
	bytes.Buffer
}

func (f *flushBuffer) Flush() error {
	return nil
}

func (f *flushBuffer) Sync() error {
	return nil
}

// contents returns the specified log value as a string.
func contents(s severity) string {
	return global.outputs[s].String()
}

// contains reports whether the string is contained in the log.
func contains(s severity, str string, t *testing.T) bool {
	return strings.Contains(contents(s), str)
}

// Test that Info works as advertised.
func TestInfo(t *testing.T) {
	_, testCleanup := testSetup(t)
	defer testCleanup()
	Info("test")
	if !contains(infoLog, "I", t) {
		t.Errorf("Info has wrong character: %q", contents(infoLog))
	}
	if !contains(infoLog, "test", t) {
		t.Error("Info failed")
	}
}

func TestInfoDepth(t *testing.T) {
	_, testCleanup := testSetup(t)
	defer testCleanup()

	f := func() { InfoDepth(1, "depth-test1") }

	// The next three lines must stay together
	_, _, wantLine, _ := runtime.Caller(0)
	InfoDepth(0, "depth-test0")
	f()

	msgs := strings.Split(strings.TrimSuffix(contents(infoLog), "\n"), "\n")
	if len(msgs) != 2 {
		t.Fatalf("Got %d lines, expected 2", len(msgs))
	}

	for i, m := range msgs {
		if !strings.HasPrefix(m, "I") {
			t.Errorf("InfoDepth[%d] has wrong character: %q", i, m)
		}
		w := fmt.Sprintf("depth-test%d", i)
		if !strings.Contains(m, w) {
			t.Errorf("InfoDepth[%d] missing %q: %q", i, w, m)
		}

		// pull out the line number (between : and ])
		msg := m[strings.LastIndex(m, ":")+1:]
		x := strings.Index(msg, "]")
		if x < 0 {
			t.Errorf("InfoDepth[%d]: missing ']': %q", i, m)
			continue
		}
		line, err := strconv.Atoi(msg[:x])
		if err != nil {
			t.Errorf("InfoDepth[%d]: bad line number: %q", i, m)
			continue
		}
		wantLine++
		if wantLine != line {
			t.Errorf("InfoDepth[%d]: got line %d, want %d", i, line, wantLine)
		}
	}
}

func init() {
	CopyStandardLogTo("INFO")
}

// Test that CopyStandardLogTo panics on bad input.
func TestCopyStandardLogToPanic(t *testing.T) {
	defer func() {
		if s, ok := recover().(string); !ok || !strings.Contains(s, "LOG") {
			t.Errorf(`CopyStandardLogTo("LOG") should have panicked: %v`, s)
		}
	}()
	CopyStandardLogTo("LOG")
}

// Test that using the standard log package logs to INFO.
func TestStandardLog(t *testing.T) {
	_, testCleanup := testSetup(t)
	defer testCleanup()
	stdLog.Print("test")
	if !contains(infoLog, "I", t) {
		t.Errorf("Info has wrong character: %q", contents(infoLog))
	}
	if !contains(infoLog, "test", t) {
		t.Error("Info failed")
	}
}

// Test that the header has the correct format.
func TestHeader(t *testing.T) {
	_, testCleanup := testSetup(t)
	defer testCleanup()
	Info("test")
	var (
		tm   time.Month
		td   int
		tH   int
		tM   int
		tS   int
		tU   int
		line int
	)
	format := fmt.Sprintf("I%%02d%%02d %%02d:%%02d:%%02d.%%06d %7d klog_test.go:%%d] test\n", os.Getpid())
	n, err := fmt.Sscanf(contents(infoLog), format,
		&tm, &td, &tH, &tM, &tS, &tU, &line)
	if n != 7 || err != nil {
		t.Errorf("log format error: %d elements, error %s:\n%s", n, err, contents(infoLog))
	}
	// Scanf treats multiple spaces as equivalent to a single space,
	// so check for correct space-padding also.
	want := fmt.Sprintf(format, tm, td, tH, tM, tS, tU, line)
	if contents(infoLog) != want {
		t.Errorf("log format error: got:\n\t%q\nwant:\t%q", contents(infoLog), want)
	}
}

func TestHeaderWithDir(t *testing.T) {
	_, testCleanup := testSetup(t, "add_dir_header", "true")
	defer testCleanup()
	Info("test")
	var (
		tm   time.Month
		td   int
		tH   int
		tM   int
		tS   int
		tU   int
		line int
	)
	format := fmt.Sprintf("I%%02d%%02d %%02d:%%02d:%%02d.%%06d %7d klog/klog_test.go:%%d] test\n", os.Getpid())
	n, err := fmt.Sscanf(contents(infoLog), format,
		&tm, &td, &tH, &tM, &tS, &tU, &line)
	if n != 7 || err != nil {
		t.Errorf("log format error: %d elements, error %s:\n%s", n, err, contents(infoLog))
	}
	// Scanf treats multiple spaces as equivalent to a single space,
	// so check for correct space-padding also.
	want := fmt.Sprintf(format, tm, td, tH, tM, tS, tU, line)
	if contents(infoLog) != want {
		t.Errorf("log format error: got:\n\t%q\nwant:\t%q", contents(infoLog), want)
	}
}

// Test that an Error log goes to Warning and Info.
// Even in the Info log, the source character will be E, so the data should
// all be identical.
func TestError(t *testing.T) {
	_, testCleanup := testSetup(t)
	defer testCleanup()
	Error("test")
	if !contains(errorLog, "E", t) {
		t.Errorf("Error has wrong character: %q", contents(errorLog))
	}
	if !contains(errorLog, "test", t) {
		t.Error("Error failed")
	}
	str := contents(errorLog)
	if !contains(warningLog, str, t) {
		t.Error("Warning failed")
	}
	if !contains(infoLog, str, t) {
		t.Error("Info failed")
	}
}

// Test that a Warning log goes to Info.
// Even in the Info log, the source character will be W, so the data should
// all be identical.
func TestWarning(t *testing.T) {
	_, testCleanup := testSetup(t)
	defer testCleanup()
	Warning("test")
	if !contains(warningLog, "W", t) {
		t.Errorf("Warning has wrong character: %q", contents(warningLog))
	}
	if !contains(warningLog, "test", t) {
		t.Error("Warning failed")
	}
	str := contents(warningLog)
	if !contains(infoLog, str, t) {
		t.Error("Info failed")
	}
}

// Test that a V log goes to Info.
func TestV(t *testing.T) {
	_, testCleanup := testSetup(t, "v", "2")
	defer testCleanup()
	V(2).Info("test")
	if !contains(infoLog, "I", t) {
		t.Errorf("Info has wrong character: %q", contents(infoLog))
	}
	if !contains(infoLog, "test", t) {
		t.Error("Info failed")
	}
}

// Test that a vmodule enables a log in this file.
func TestVmoduleOn(t *testing.T) {
	_, testCleanup := testSetup(t, "vmodule", "klog_test=2")
	defer testCleanup()
	if !V(1) {
		t.Error("V not enabled for 1")
	}
	if !V(2) {
		t.Error("V not enabled for 2")
	}
	if V(3) {
		t.Error("V enabled for 3")
	}
	V(2).Info("test")
	if !contains(infoLog, "I", t) {
		t.Errorf("Info has wrong character: %q", contents(infoLog))
	}
	if !contains(infoLog, "test", t) {
		t.Error("Info failed")
	}
}

// Test that a vmodule of another file does not enable a log in this file.
func TestVmoduleOff(t *testing.T) {
	_, testCleanup := testSetup(t, "vmodule", "notthisfile=2")
	defer testCleanup()
	for i := 1; i <= 3; i++ {
		if V(Level(i)) {
			t.Errorf("V enabled for %d", i)
		}
	}
	V(2).Info("test")
	if contents(infoLog) != "" {
		t.Error("V logged incorrectly")
	}
}

func flushDaemon(stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-stop:
			ticker.Stop()
			return
		case <-ticker.C:
			Flush()
		}
	}
}

func TestSetOutputDataRace(t *testing.T) {
	_, testCleanup := testSetup(t)
	stop := make(chan struct{})
	defer testCleanup()
	var wg sync.WaitGroup
	for i := 1; i <= 50; i++ {
		go func() {
			flushDaemon(stop)
		}()
	}
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetOutput(ioutil.Discard)
		}()
	}
	for i := 1; i <= 50; i++ {
		go func() {
			flushDaemon(stop)
		}()
	}
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetOutputBySeverity("INFO", ioutil.Discard)
		}()
	}
	for i := 1; i <= 50; i++ {
		go func() {
			flushDaemon(stop)
		}()
	}
	wg.Wait()
	close(stop)
}

// vGlobs are patterns that match/don't match this file at V=2.
var vGlobs = map[string]bool{
	// Easy to test the numeric match here.
	"klog_test=1": false, // If -vmodule sets V to 1, V(2) will fail.
	"klog_test=2": true,
	"klog_test=3": true, // If -vmodule sets V to 1, V(3) will succeed.
	// These all use 2 and check the patterns. All are true.
	"*=2":           true,
	"?l*=2":         true,
	"????_*=2":      true,
	"??[mno]?_*t=2": true,
	// These all use 2 and check the patterns. All are false.
	"*x=2":         false,
	"m*=2":         false,
	"??_*=2":       false,
	"?[abc]?_*t=2": false,
}

// Test that vmodule globbing works as advertised.
func testVmoduleGlob(pat string, match bool, t *testing.T) {
	_, testCleanup := testSetup(t, "vmodule", pat)
	defer testCleanup()
	if V(2) != Verbose(match) {
		t.Errorf("incorrect match for %q: got %t expected %t", pat, V(2), match)
	}
}

// Test that a vmodule globbing works as advertised.
func TestVmoduleGlob(t *testing.T) {
	for glob, match := range vGlobs {
		testVmoduleGlob(glob, match, t)
	}
}

func TestRollover(t *testing.T) {
	_, testCleanup := testSetup(t)
	defer testCleanup()

	runHelperProcess(t)
}

func TestRolloverHelperProcess(t *testing.T) {
	ok, args := amHelperProcess()
	if !ok {
		return
	}

	if len(args) != 0 {
		t.Fatal("Wrong number of args")
	}

	klogv2.MaxSize = 512
	MaxSize = klogv2.MaxSize
	flagset := flag.NewFlagSet("klog", flag.ContinueOnError)
	InitFlags(flagset)
	flagset.Set("logtostderr", "false")
	flagset.Set("add_dir_header", "false")

	Info("x") // Be sure we have a file.
	Flush()
	fname0, err := os.Readlink(filepath.Join(os.TempDir(), filepath.Base(os.Args[0])+".INFO"))
	if err != nil {
		t.Fatal("info wasn't created")
	}
	Info(strings.Repeat("x", int(MaxSize))) // force a rollover
	Flush()

	// Make sure the next log file gets a file name with a different
	// time stamp.
	//
	// TODO: determine whether we need to support subsecond log
	// rotation.  C++ does not appear to handle this case (nor does it
	// handle Daylight Savings Time properly).
	time.Sleep(1 * time.Second)

	Info("x") // create a new file
	Flush()
	fname1, err := os.Readlink(filepath.Join(os.TempDir(), filepath.Base(os.Args[0])+".INFO"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fname0 == fname1 {
		t.Errorf("info.f.Name did not change: %v", fname0)
	}
	fileinfo, err := os.Stat(filepath.Join(os.TempDir(), filepath.Base(os.Args[0])+".INFO"))
	if err != nil {
		t.Fatalf("unexpected error: %v", fname0)
	}
	if fileinfo.Size() >= int64(MaxSize) {
		t.Errorf("file size was not reset: %d", fileinfo.Size())
	}
}

func TestOpenAppendOnStart(t *testing.T) {
	_, testCleanup := testSetup(t)
	defer testCleanup()

	const (
		x string = "xxxxxxxxxx"
		y string = "yyyyyyyyyy"
	)

	f, err := ioutil.TempFile("", "test_klog_OpenAppendOnStart")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Logging creates the file
	runHelperProcess(t, f.Name(), x)

	// ensure we wrote what we expected
	b, err := ioutil.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(b), x) {
		t.Fatalf("got %s, missing expected Info log: %s", string(b), x)
	}

	// Logging again should open the file again with O_APPEND instead of O_TRUNC
	runHelperProcess(t, f.Name(), y)

	// ensure we wrote what we expected
	b, err = ioutil.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(b), y) {
		t.Fatalf("got %s, missing expected Info log: %s", string(b), y)
	}
	// The initial log message should be preserved across create calls.
	if !strings.Contains(string(b), x) {
		t.Fatalf("got %s, missing expected Info log: %s", string(b), x)
	}
}

func TestOpenAppendOnStartHelperProcess(t *testing.T) {
	ok, args := amHelperProcess()
	if !ok {
		return
	}

	if len(args) != 2 {
		t.Fatal("Wrong number of args")
	}
	filename := args[0]
	message := args[1]

	flagset := flag.NewFlagSet("klog", flag.ContinueOnError)
	InitFlags(flagset)
	flagset.Set("logtostderr", "false")
	flagset.Set("add_dir_header", "false")
	flagset.Set("log_file", filename)

	// Will os.Exit(2) if Info(x) or Flush() error.
	Info(message)
	Flush()
}

func TestLogBacktraceAt(t *testing.T) {
	// The peculiar style of this code simplifies line counting and maintenance of the
	// tracing block below.
	var testCleanup func() = func() {}
	defer func() { testCleanup() }()
	var infoLine string
	setTraceLocation := func(file string, line int, ok bool, delta int) {
		if !ok {
			t.Fatal("could not get file:line")
		}
		_, file = filepath.Split(file)
		infoLine = fmt.Sprintf("%s:%d", file, line+delta)
		_, testCleanup = testSetup(t, "log_backtrace_at", infoLine)
	}
	{
		// Start of tracing block. These lines know about each other's relative position.
		_, file, line, ok := runtime.Caller(0)
		setTraceLocation(file, line, ok, +2) // Two lines between Caller and Info calls.
		Info("we want a stack trace here")
	}
	numAppearances := strings.Count(contents(infoLog), infoLine)
	if numAppearances < 2 {
		// Need 2 appearances, one in the log header and one in the trace:
		//   log_test.go:281: I0511 16:36:06.952398 02238 log_test.go:280] we want a stack trace here
		//   ...
		//   k8s.io/klog/klog_test.go:280 (0x41ba91)
		//   ...
		// We could be more precise but that would require knowing the details
		// of the traceback format, which may not be dependable.
		t.Fatal("got no trace back; log is ", contents(infoLog))
	}
}

func BenchmarkLogs(b *testing.B) {
	testFile, err := ioutil.TempFile("", "test.log")
	if err != nil {
		b.Error("unable to create temporary file")
	}
	defer os.Remove(testFile.Name())

	_, testCleanup := testSetup(b,
		"v", "0",
		"logtostderr", "false",
		"alsologtostderr", "false",
		"stderrthreshold", severityName[fatalLog],
		"log_file", testFile.Name())
	defer testCleanup()

	for i := 0; i < b.N; i++ {
		Error("error")
		Warning("warning")
		Info("info")
	}
	Flush()
}

// Test the logic on checking log size limitation.
func TestFileSizeCheck(t *testing.T) {
	testData := map[string]struct {
		testLogFile          string
		testLogFileMaxSizeMB uint64
		testCurrentSize      uint64
		expectedResult       bool
	}{
		"logFile not specified, exceeds max size": {
			testLogFile:          "",
			testLogFileMaxSizeMB: 1,
			testCurrentSize:      1024 * 1024 * 2000, //exceeds the maxSize
			expectedResult:       true,
		},

		"logFile not specified, not exceeds max size": {
			testLogFile:          "",
			testLogFileMaxSizeMB: 1,
			testCurrentSize:      1024 * 1024 * 1000, //smaller than the maxSize
			expectedResult:       false,
		},
		"logFile specified, exceeds max size": {
			testLogFile:          "/tmp/test.log",
			testLogFileMaxSizeMB: 500,                // 500MB
			testCurrentSize:      1024 * 1024 * 1000, //exceeds the logFileMaxSizeMB
			expectedResult:       true,
		},
		"logFile specified, not exceeds max size": {
			testLogFile:          "/tmp/test.log",
			testLogFileMaxSizeMB: 500,               // 500MB
			testCurrentSize:      1024 * 1024 * 300, //smaller than the logFileMaxSizeMB
			expectedResult:       false,
		},
	}

	for name, test := range testData {
		test := test // capture loop variable
		t.Run(name, func(t *testing.T) {
			_, testCleanup := testSetup(t,
				"log_file", test.testLogFile,
				"log_file_max_size", strconv.FormatUint(test.testLogFileMaxSizeMB, 10))
			defer testCleanup()
			actualResult := test.testCurrentSize >= CalculateMaxSize()
			if test.expectedResult != actualResult {
				t.Fatalf("Was expecting result equals %v, got %v",
					test.expectedResult, actualResult)
			}
		})
	}
}
