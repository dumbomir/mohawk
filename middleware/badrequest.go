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

// Package middleware middlewares for Mohawk
package middleware

import (
	"fmt"
	"log"
	"net/http"
)

// BadRequest will be called if no route found
type BadRequest struct {
	Verbose bool
}

// SetNext set next http serve func
func (b *BadRequest) SetNext(_h http.Handler) {
}

// ServeHTTP http serve func
func (b BadRequest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// we return 200 for any OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "authorization,content-type,hawkular-tenant")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT")
		fmt.Fprintf(w, "{\"GET\":{},\"PUT\":{},\"POST\":{}}")
		return
	}

	log.Printf("Page not found - 404:\n")
	log.Printf("%s Accept-Encoding: %s, %4s %s", r.RemoteAddr, r.Header.Get("Accept-Encoding"), r.Method, r.URL)

	w.WriteHeader(404)
	fmt.Fprintf(w, "Page not found - 404\n")
}
