/*
The MIT License (MIT)

Copyright (c) 2014 Chris Grieger

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// CreatePidfile creates a pidfile.
// If the file already exists, it checks if the process is already running and terminates.
func CreatePidfile(pidFile string) {
	if pidFile != "" {
		if err := os.MkdirAll(filepath.Dir(pidFile), os.FileMode(0755)); err != nil {
			log.Fatalf("Could not create path to pidfile %v", err)
		}

		if _, err := os.Stat(pidFile); err != nil && !os.IsNotExist(err) {
			log.Fatalf("Failed to stat pidfile %v", err)
		}

		f, err := os.OpenFile(pidFile, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Fatalf("Failed to open pidfile %v", err)
		}
		defer f.Close()

		if pidBytes, err := ioutil.ReadAll(f); err != nil {
			log.Fatalf("Failed to read from pidfile %v", err)
		} else {
			if len(pidBytes) == 0 {
				goto foo
			}

			pid, err := strconv.Atoi(string(pidBytes))
			if err != nil {
				log.Fatalf("Invalid pid %v", err)
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				log.Fatalf("Failed to find process %v, please delete the pid file %s manually", err, pidFile)
			}

			if err := process.Signal(syscall.Signal(0)); err == nil {
				log.Fatalf("Process %d still running, please stop the process and delete the pid file %s manually", pid, pidFile)
			}
		}

	foo:
		if err = f.Truncate(0); err != nil {
			log.Fatalf("Failed to truncate pidfile %v", err)
		}
		if _, err = f.Seek(0, 0); err != nil {
			log.Fatalf("Failed to seek pidfile %v", err)
		}

		_, err = fmt.Fprintf(f, "%d", os.Getpid())
		if err != nil {
			log.Fatalf("Failed to write pidfile %v", err)
		}
	}
}
