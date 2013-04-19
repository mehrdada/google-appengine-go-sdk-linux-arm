// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package id

import (
	"fmt"
	"net/http"
	"os"

	"appengine"
)

func init() {
	http.HandleFunc("/id", handleID)
}

func handleID(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	fmt.Fprintf(w, "appengine.AppID(c) = %q\n", appengine.AppID(c))
	fmt.Fprintf(w, "appengine.VersionID(c) = %q\n", appengine.VersionID(c))

	name, index := appengine.BackendInstance(c)
	fmt.Fprintf(w, "appengine.BackendInstance(c) = %q, %d\n", name, index)

	fmt.Fprintf(w, "----------\n")
	for _, s := range os.Environ() {
		fmt.Fprintln(w, s)
	}
}
