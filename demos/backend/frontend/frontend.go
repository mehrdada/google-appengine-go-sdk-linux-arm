// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package frontend implements a frontend to the example memcache backend.
// TODO: Demonstrate sharding across multiple backend instances.
package frontend

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"text/template"

	"appengine"
	"appengine/urlfetch"
)

func init() {
	http.HandleFunc("/", handleFront)
}

func handleFront(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	key, value := r.FormValue("key"), r.FormValue("value")
	m := map[string]interface{}{
		"Key":            key,
		"Value":          value,
		"BackendAddress": backend(c),
	}

	switch r.Method {
	case "GET":
		m["GetValue"], m["Error"] = get(c, key)
	case "POST":
		m["SetMessage"], m["Error"] = set(c, key, value)
	}

	if err := page.Execute(w, m); err != nil {
		c.Errorf("Template execution failed: %v", err)
	}
}

func backend(c appengine.Context) string {
	// Use the load-balanced hostname for the "memcache" backend.
	return "http://" + appengine.BackendHostname(c, "memcache", -1)
}

func get(c appengine.Context, key string) ([]byte, error) {
	u := backend(c) + "/memcache/get?key=" + url.QueryEscape(key)
	resp, err := urlfetch.Client(c).Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func set(c appengine.Context, key, value string) (string, error) {
	u := backend(c) + "/memcache/set"
	resp, err := urlfetch.Client(c).PostForm(u, url.Values{
		"key":   {key},
		"value": {value},
	})
	return resp.Status, err
}

var page = template.Must(template.New("front").Parse(`
<!DOCTYPE html>
<html>
<head>
	<title>Memcache frontend</title>
	<style type="text/css">
	.box {
		border: 1px solid black;
		margin: 0.5em;
		padding: 1em;
		width: 20em;
	}
	.error {
		color: red;
		font-weight: bold;
	}
	</style>
</head>
<body>
<h1>Memcache frontend</h1>
	{{with .Error}}
	<span class="error">{{.|html}}</span>
	{{end}}

	<div class="box">
	<h3>Get</h3>
	<form method=GET action="/">
		<label for="key">Key:</label>
		<input type="text" name="key" id="key" value="{{.Key|html}}" /><br />
		<input type="submit" /><br />
		{{with .GetValue}}
		<b>{{printf "%s" .|html}}</b>
		{{end}}
	</form>
	</div>

	<div class="box">
	<h3>Set</h3>
	<form method=POST action="/">
		<label for="key">Key:</label>
		<input type="text" name="key" id="key" value="{{.Key|html}}" /><br />
		<label for="value">Value:</label>
		<input type="text" name="value" id="value" value="{{.Value|html}}" /><br />
		<input type="submit" /><br />
		{{with .SetMessage}}
		<b>{{.|html}}</b>
		{{end}}
	</form>
	</div>

	<p>Backend addresss: <code>{{.BackendAddress|html}}</code></p>
</body>
</html>
`))
