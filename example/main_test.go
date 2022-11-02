/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Main(t *testing.T) {
	// Capture stdout and revert it on test completion.
	tmp := t.TempDir()
	stdoutPath := path.Join(tmp, "stdout.txt")
	stdoutF, err := os.Create(stdoutPath)
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = stdoutF

	revertStdout := func() {
		_ = stdoutF.Close()
		os.Stdout = oldStdout
	}
	defer revertStdout()

	// create root context id
	rootContextID = atomic.AddInt32(&contextIDGenerator, 1)

	// start the test HTTP server
	ts := httptest.NewServer(http.HandlerFunc(ServeHTTP))
	defer ts.Close()

	// make a client request
	resp, err := ts.Client().Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// read back the stdout
	stdout, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}

	// sort to avoid test failures due to non-deterministic map iteration.
	lines := strings.Split(string(stdout), "\n")
	sort.Strings(lines)
	for _, s := range []string{
		"receive request /",
		"print header from server host, User-Agent -> [Go-http-client/1.1]",
		"print header from server host, Accept-Encoding -> [gzip]",
		"[http_wasm_example.cc:33]::onRequestHeaders() print from wasm, onRequestHeaders, context id: 2",
		"[http_wasm_example.cc:38]::onRequestHeaders() print from wasm, User-Agent -> Go-http-client/1.1",
		"[http_wasm_example.cc:38]::onRequestHeaders() print from wasm, Accept-Encoding -> gzip",
	} {
		require.Contains(t, lines, s)
	}
}
