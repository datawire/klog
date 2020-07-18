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

package klog

import (
	"flag"
	"testing"
)

// TODO: This test package should be refactored so that tests cannot
// interfere with each-other.

// Test that shortHostname works as advertised.
func TestShortHostname(t *testing.T) {
	for hostname, expect := range map[string]string{
		"":                "",
		"host":            "host",
		"host.google.com": "host",
	} {
		if got := shortHostname(hostname); expect != got {
			t.Errorf("shortHostname(%q): expected %q, got %q", hostname, expect, got)
		}
	}
}

func BenchmarkHeader(b *testing.B) {
	logging.addDirHeader = false
	for i := 0; i < b.N; i++ {
		buf, _, _ := logging.header(infoLog, 0)
		logging.putBuffer(buf)
	}
}

func BenchmarkHeaderWithDir(b *testing.B) {
	logging.addDirHeader = true
	for i := 0; i < b.N; i++ {
		buf, _, _ := logging.header(infoLog, 0)
		logging.putBuffer(buf)
	}
}

func TestInitFlags(t *testing.T) {
	fs1 := flag.NewFlagSet("test1", flag.PanicOnError)
	InitFlags(fs1)
	fs1.Set("log_dir", "/test1")
	fs1.Set("log_file_max_size", "1")
	fs2 := flag.NewFlagSet("test2", flag.PanicOnError)
	InitFlags(fs2)
	if logging.logDir != "/test1" {
		t.Fatalf("Expected log_dir to be %q, got %q", "/test1", logging.logDir)
	}
	fs2.Set("log_file_max_size", "2048")
	if logging.logFileMaxSizeMB != 2048 {
		t.Fatal("Expected log_file_max_size to be 2048")
	}
}
