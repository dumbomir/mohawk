// Copyright 2016,2017 Yaacov Zamir <kobi.zamir@gmail.com>
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

// Package api API REST server
package api

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/viper"

	"github.com/MohawkTSDB/mohawk/alerts"
	"github.com/MohawkTSDB/mohawk/middleware"
	"github.com/MohawkTSDB/mohawk/router"
	"github.com/MohawkTSDB/mohawk/storage"
	"github.com/MohawkTSDB/mohawk/storage/example"
	"github.com/MohawkTSDB/mohawk/storage/memory"
	"github.com/MohawkTSDB/mohawk/storage/mongo"
	"github.com/MohawkTSDB/mohawk/storage/sqlite"
)

// VER the server version
const VER = "0.22.1"

// defaults
const defaultAPI = "0.21.0"

// BackendName Mohawk active storage
var BackendName string

// GetStatus return a json status struct
func GetStatus(w http.ResponseWriter, r *http.Request, argv map[string]string) {
	resTemplate := `{"MetricsService":"STARTED","Implementation-Version":"%s","MohawkVersion":"%s","MohawkBackend":"%s"}`
	res := fmt.Sprintf(resTemplate, defaultAPI, VER, BackendName)

	w.WriteHeader(200)
	fmt.Fprintln(w, res)
}

// Serve run the REST API server
func Serve() error {
	var db storage.Backend

	var backendQuery = viper.GetString("storage")
	var optionsQuery = viper.GetString("options")
	var verbose = viper.GetBool("verbose")
	var media = viper.GetString("media")
	var tls = viper.GetBool("tls")
	var gzip = viper.GetBool("gzip")
	var token = viper.GetString("token")
	var port = viper.GetInt("port")
	var cert = viper.GetString("cert")
	var key = viper.GetString("key")
	var configAlerts = viper.ConfigFileUsed() != "" && viper.Get("alerts") != ""

	// Create and init the storage
	switch backendQuery {
	case "sqlite":
		db = &sqlite.Backend{}
	case "memory":
		db = &memory.Backend{}
	case "mongo":
		db = &mongo.Backend{}
	case "example":
		db = &example.Backend{}
	default:
		log.Fatal("Can't find storage:", backendQuery)
	}

	// parse options
	if options, err := url.ParseQuery(optionsQuery); err == nil {
		db.Open(options)
	} else {
		log.Fatal("Can't parse opetions:", optionsQuery)
	}

	// set global variables
	BackendName = db.Name()

	// Create alerts runner
	if configAlerts {
		// parse alert list from config yaml
		l := []*alerts.Alert{}
		viper.UnmarshalKey("alerts", &l)

		// creat and Init the alert handler
		a := &alerts.Alerts{
			Backend: db,
			Verbose: verbose,
			Alerts:  l,
		}
		a.Init()
	}

	// h common variables to be used for the storage Handler functions
	// Backend the storage to use for metrics source
	h := Handler{
		Verbose: verbose,
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
	rGauges.Add("PUT", "tags", h.PutMultiTags)
	rGauges.Add("PUT", ":id/tags", h.PutTags)
	rGauges.Add("DELETE", ":id/raw", h.DeleteData)
	rGauges.Add("DELETE", ":id/tags/:tags", h.DeleteTags)

	// deprecated
	rGauges.Add("GET", ":id/data", h.GetData)
	rGauges.Add("POST", "data", h.PostData)
	rGauges.Add("POST", "stats/query", h.PostQuery)

	rCounters := router.Router{
		Prefix: "/hawkular/metrics/counters/",
	}
	rCounters.Add("GET", ":id/raw", h.GetData)
	rCounters.Add("GET", ":id/stats", h.GetData)
	rCounters.Add("POST", "raw", h.PostData)
	rCounters.Add("POST", "raw/query", h.PostQuery)
	rCounters.Add("PUT", ":id/tags", h.PutTags)

	// deprecated
	rCounters.Add("GET", ":id/data", h.GetData)
	rCounters.Add("POST", "data", h.PostData)
	rCounters.Add("POST", "stats/query", h.PostQuery)

	rAvailability := router.Router{
		Prefix: "/hawkular/metrics/availability/",
	}
	rAvailability.Add("GET", ":id/raw", h.GetData)
	rAvailability.Add("GET", ":id/stats", h.GetData)

	// create a list of routes
	routers := []*router.Router{}
	routers = append(routers, &rGauges, &rCounters, &rAvailability, &rRoot)

	// fallback handler, static decorator + bad request handler
	staticDecorator := middleware.FileServeDecorator(media)
	badRequestHandler := middleware.BadRequestHandler(log.Printf)
	fallback := staticDecorator(badRequestHandler)

	// concat all routers and add fallback handler
	core := router.Append(fallback, routers...)

	// create a list of middlwares
	decorators := []middleware.Decorator{}
	decorators = append(decorators, middleware.LoggingDecorator(log.Printf), middleware.DefaultHeadersDecorator())
	if token != "" {
		decorators = append(decorators, middleware.AuthDecorator(token, "^/hawkular/metrics/status$"))
	}
	if gzip {
		decorators = append(decorators, middleware.GzipDecodeDecorator(), middleware.GzipEncodeDecorator())
	}

	// concat middlewars and routes (first logger until rRoot) with a fallback to BadRequest
	handler := middleware.Append(core, decorators...)

	// Run the server
	srv := &http.Server{
		Addr:           fmt.Sprintf("0.0.0.0:%d", port),
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if tls {
		log.Printf("Start server, listen on https://%+v", srv.Addr)
		log.Fatal(srv.ListenAndServeTLS(cert, key))
	} else {
		log.Printf("Start server, listen on http://%+v", srv.Addr)
		log.Fatal(srv.ListenAndServe())
	}

	return nil
}