// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package appengine_internal

import (
	"log"
	"net/http"
	"strconv"

	"code.google.com/p/goprotobuf/proto"

	pb "appengine_internal/servers"
)

// These functions are the dev implementations of the wrapper functions
// in ../appengine/identity.go. See that file for commentary.

const (
	hRequestLogId = "X-AppEngine-Internal-Request-Log-Id"
)

func BackendHostname(c apiContext, name string, index int) string {
	// TODO: Use the servers API when one exists.
	req := &pb.GetHostnameRequest{
		Server: proto.String(name),
	}
	if index != -1 {
		req.Instance = proto.String(strconv.Itoa(index))
	}
	res := &pb.GetHostnameResponse{}

	if err := c.Call("servers", "GetHostname", req, res, nil); err != nil {
		log.Printf("appengine: call to servers.GetHostname (name=%s, index=%d) failed: %s",
			name, index, err)
		return "" // The API doesn't allow for error returns.
	}

	return *res.Hostname
}

func DefaultVersionHostname(req interface{}) string {
	return req.(*http.Request).Host
}

func BackendInstance() int {
	i, err := strconv.Atoi(InstanceID())
	if err != nil {
		return -1
	}
	return i
}

func VersionID(req interface{}) string {
	return instanceConfig.VersionID
}

func InstanceID() string {
	return instanceConfig.InstanceID
}

func Datacenter() string {
	return instanceConfig.Datacenter
}

func ServerSoftware() string {
	return "Development/2.0"
}

func RequestID(req interface{}) string {
	return req.(*http.Request).Header.Get(hRequestLogId)
}
