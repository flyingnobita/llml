package models

import (
	"net"
	"net/http"
	"time"
)

// sharedHTTPClient is the client used for every outbound request this package
// makes. One client is reused so connections are kept alive across the repeated
// probes that discovery and the runtime panel perform; a fresh client per call
// discards the connection pool along with it.
//
// It carries no Timeout of its own on purpose: per-call deadlines come from the
// caller's context, so a cancelled scan aborts immediately rather than waiting
// out a fixed timeout. The transport-level timeouts below bound the phases a
// context cannot help with on its own.
var sharedHTTPClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 3 * time.Second,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
	},
}
