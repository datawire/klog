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
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"

	. "k8s.io/klog"
)

type severity int32

const (
	infoLog severity = iota
	warningLog
	errorLog
	fatalLog
	numSeverity = 4
)

var severityName = []string{
	infoLog:    "INFO",
	warningLog: "WARNING",
	errorLog:   "ERROR",
	fatalLog:   "FATAL",
}

var global struct {
	mu      sync.Mutex
	outputs [numSeverity]*flushBuffer
}

func setEnv(key, value string) func() {
	old, oldOK := os.LookupEnv(key)
	os.Setenv(key, value)
	return func() {
		if oldOK {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	}
}

func testSetup(t testing.TB, args ...string) (*flag.FlagSet, func()) {
	if len(args)%2 != 0 {
		t.Fatalf("testSetup requires an even number of args")
	}

	global.mu.Lock()

	tmpdir, err := ioutil.TempDir("", strings.ReplaceAll(t.Name(), "/", "_")+".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tmpenv := "TMPDIR"
	if runtime.GOOS == "windows" {
		tmpenv = "TMP"
	}
	unsetEnv := setEnv(tmpenv, tmpdir)

	args = append([]string{
		"logtostderr", "false",
		"add_dir_header", "false",
	}, args...)

	flagset := flag.NewFlagSet("klog", flag.ContinueOnError)
	InitFlags(flagset)
	defaults := make(map[string]string)
	flagset.VisitAll(func(f *flag.Flag) {
		value := f.Value.String()
		if f.Name == "log_backtrace_at" {
			value = ""
		}
		defaults[f.Name] = value
	})

	for i := 0; i < numSeverity; i++ {
		global.outputs[i] = new(flushBuffer)
		SetOutputBySeverity(severityName[i], global.outputs[i])
	}

	for i := 0; i < numSeverity; i++ {
		global.outputs[i] = new(flushBuffer)
		SetOutputBySeverity(severityName[i], global.outputs[i])
	}

	for i := 0; i+1 < len(args); i += 2 {
		if err := flagset.Set(args[i], args[i+1]); err != nil {
			t.Fatalf("error setting %s=%q: %v", args[i], args[i+1], err)
		}
	}

	return flagset, func() {
		if r := recover(); r != nil {
			panic(r)
		}
		for i := 0; i < numSeverity; i++ {
			global.outputs[i] = nil
			SetOutputBySeverity(severityName[i], nil)
		}
		for k, v := range defaults {
			if err := flagset.Set(k, v); err != nil {
				t.Fatalf("error resetting %s=%q: %v", k, v, err)
			}
		}
		unsetEnv()
		os.RemoveAll(tmpdir)
		global.mu.Unlock()
	}
}

func runHelperProcess(t *testing.T, s ...string) {
	t.Helper()

	args := append([]string{"-test.run=" + t.Name() + "HelperProcess", "--"}, s...)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "KLOG_WANT_HELPER_PROCESS=1")

	output, err := cmd.CombinedOutput()
	cleanedOutput := strings.TrimRight(string(output), "\n")
	if cleanedOutput != "" {
		for _, line := range strings.Split(cleanedOutput, "\n") {
			t.Log(line)
		}
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func amHelperProcess() (bool, []string) {
	if os.Getenv("KLOG_WANT_HELPER_PROCESS") != "1" {
		return false, nil
	}
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	return true, args
}
