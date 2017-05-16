// Copyright 2016 Red Hat, Inc. and/or its affiliates
// and other contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/yaacov/mohawk/backend"
	"github.com/yaacov/mohawk/backend/sqlite"
	"github.com/yaacov/mohawk/middleware"
	"github.com/yaacov/mohawk/router"
)

// VER the server version
const VER = "0.10.2"

// defaults
const defaultPort = 8080
const defaultBackend = "sqlite"
const defaultAPI = "0.21.0"
const defaultTLS = false
const defaultTLSKey = "server.key"
const defaultTLSCert = "server.pem"

// BackendName MoHawk active backend
var BackendName string

// GetStatus return a json status struct
func GetStatus(w http.ResponseWriter, r *http.Request, argv map[string]string) {
	resTemplate := `{"MetricsService":"STARTED","Implementation-Version":"%s","MohawkVersion":"%s","MohawkBackend":"%s"}`
	res := fmt.Sprintf(resTemplate, defaultAPI, VER, BackendName)

	w.WriteHeader(200)
	fmt.Fprintln(w, res)
}

func main() {
	var db backend.Backend
	var middlewareList []middleware.MiddleWare

	// Get user options
	portPtr := flag.Int("port", defaultPort, "server port")
	backendPtr := flag.String("backend", defaultBackend, "the backend to use [sqlite]")
	tlsPtr := flag.Bool("tls", defaultTLS, "use TLS server")
	gzipPtr := flag.Bool("gzip", false, "accept gzip encoding")
	optionsPtr := flag.String("options", "", "specific backend options [e.g. db-dirname (sqlite), max-size (random)]")
	verbosePtr := flag.Bool("verbose", false, "more debug output")
	quietPtr := flag.Bool("quiet", false, "less debug output")
	versionPtr := flag.Bool("version", false, "version number")
	keyPtr := flag.String("key", defaultTLSKey, "path to TLS key file")
	certPtr := flag.String("cert", defaultTLSCert, "path to TLS cert file")
	flag.Parse()

	// return version number and exit
	if *versionPtr {
		fmt.Printf("MoHawk version: %s\n\n", VER)
		return
	}

	// Create and init the backend
	switch *backendPtr {
	case "sqlite":
		db = &sqlite.Backend{}
	default:
		log.Fatal("Can't find backend:", *backendPtr)
	}

	// parse options
	if options, err := url.ParseQuery(*optionsPtr); err == nil {
		db.Open(options)
	} else {
		log.Fatal("Can't parse opetions:", *optionsPtr)
	}

	// set global variables
	BackendName = db.Name()

	// h common variables to be used for the backend Handler functions
	// backend the backend to use for metrics source
	h := backend.Handler{
		Verbose: *verbosePtr,
		Backend: db,
	}

	// Create the routers
	// Requests not handled by the routers will be forworded to BadRequest Handler
	rRoot := router.Router{
		Prefix: "/hawkular/metrics/",
	}
	// Root Routing table
	rRoot.Add("GET", "status", GetStatus)
	rRoot.Add("GET", "tenants", h.GetTenants)
	rRoot.Add("GET", "metrics", h.GetMetrics)

	// Metrics Routing tables
	rGauges := router.Router{
		Prefix: "/hawkular/metrics/gauges/",
	}
	rGauges.Add("GET", ":id/raw", h.GetData)
	rGauges.Add("GET", ":id/stats", h.GetData)
	rGauges.Add("POST", "raw", h.PostData)
	rGauges.Add("POST", "raw/query", h.PostQuery)
	rGauges.Add("PUT", ":id/tags", h.PutTags)
	rGauges.Add("DELETE", ":id/raw", h.DeleteData)
	rGauges.Add("DELETE", ":id/tags/:tags", h.DeleteTags)

	rCounters := router.Router{
		Prefix: "/hawkular/metrics/counters/",
	}
	rCounters.Add("GET", ":id/raw", h.GetData)
	rCounters.Add("GET", ":id/stats", h.GetData)
	rCounters.Add("POST", "raw", h.PostData)
	rCounters.Add("POST", "raw/query", h.PostQuery)
	rCounters.Add("PUT", ":id/tags", h.PutTags)

	rAvailability := router.Router{
		Prefix: "/hawkular/metrics/availability/",
	}
	rAvailability.Add("GET", ":id/raw", h.GetData)
	rAvailability.Add("GET", ":id/stats", h.GetData)

	// logger a logging middleware
	logger := middleware.Logger{
		Quiet:   *quietPtr,
		Verbose: *verbosePtr,
	}

	// gzipper a gzip encoding middleware
	gzipper := middleware.GZipper{
		UseGzip: *gzipPtr,
		Verbose: *verbosePtr,
	}

	// fallback a BadRequest middleware
	fallback := middleware.BadRequest{}

	// concat middlewars and routes (first logger until rRoot) with a fallback to BadRequest
	middlewareList = []middleware.MiddleWare{&logger, &gzipper, &rGauges, &rCounters, &rAvailability, &rRoot, &fallback}
	middleware.Append(middlewareList)

	// Run the server
	srv := &http.Server{
		Addr:           fmt.Sprintf("0.0.0.0:%d", *portPtr),
		Handler:        logger,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if *tlsPtr {
		log.Printf("Start server, listen on https://%+v", srv.Addr)
		log.Fatal(srv.ListenAndServeTLS(*certPtr, *keyPtr))
	} else {
		log.Printf("Start server, listen on http://%+v", srv.Addr)
		log.Fatal(srv.ListenAndServe())
	}
}
