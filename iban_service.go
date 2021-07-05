/*
The MIT License (MIT)

Copyright (c) 2014 Chris Grieger
Copyright (c) 2021 Stefan Schubert

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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fourcube/goiban"
	"github.com/fourcube/goiban-data"
	"github.com/fourcube/goiban-data-loader/loader"

	"github.com/julienschmidt/httprouter"
	"github.com/pmylund/go-cache"
	"github.com/rs/cors"
)

/**
Handles requests and serves static pages.

route							description
--------------------			--------------------------------------------------------
/validate/{iban} 				Tries to validate {iban} and returns a HTTP response
								in JSON. See goiban.ValidationResult for details of the
								data returned.
*/
var (
	c   = cache.New(5*time.Minute, 30*time.Second)
	err error

	repo data.BankDataRepository
	// Set at link time
	Version string = "dev"
	// Flags
	basePath     string = "data"
	dataPath     string
	pidFile      string
	port         string
	help         bool
	printVersion bool
)

func init() {
	flag.StringVar(&dataPath, "dataPath", "", "Base path of the bank data")
	flag.StringVar(&pidFile, "pidFile", "", "PID File path")

	flag.StringVar(&port, "port", "8080", "HTTP Port or interface to listen on")
	flag.BoolVar(&help, "h", false, "Show usage")
	flag.BoolVar(&printVersion, "v", false, "Show version")
}

func main() {
	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	if printVersion {
		fmt.Println(Version)
		return
	}

	if pidFile != "" {
		CreatePidfile(pidFile)
	}

	listen()
}

func listen() {
	log.Printf("Using in-memory data store.")
	repo = data.NewInMemoryStore()

	if dataPath != "" {
		basePath = dataPath
	}
	loader.LoadBundesbankData(filepath.Join(basePath, "bundesbank.txt"), repo)
	loader.LoadAustriaData(filepath.Join(basePath, "at.csv"), repo)
	loader.LoadBelgiumData(filepath.Join(basePath, "nbb.xlsx"), repo)
	loader.LoadLuxembourgData(filepath.Join(basePath, "lu.xlsx"), repo)
	loader.LoadNetherlandsData(filepath.Join(basePath, "nl.xlsx"), repo)
	loader.LoadSwitzerlandData(filepath.Join(basePath, "ch.xlsx"), repo)

	router := httprouter.New()
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
	})

	router.GET("/validate/:iban", validationHandler)

	listeningInfo := "Listening on %s"
	handler := corsHandler.Handler(router)

	var server http.Server
	var addr string

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		log.Printf("Received SIGINT. Waiting for connections to close...")

		// We received an interrupt signal, shut down.
		if err := server.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	if strings.ContainsAny(port, ":") {
		addr = port
	} else {
		addr = ":" + port
	}

	server.Handler = handler
	server.Addr = addr

	log.Printf("goiban-service (v%s)", Version)
	log.Printf(listeningInfo, port)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal("ListenAndServe: ", err)
	}

	<-idleConnsClosed
}

// Processes requests to the /validate/ url
func validationHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var strRes string
	config := map[string]bool{}
	// Set response type to application/json.
	// See: https://www.owasp.org/index.php/XSS_(Cross_Site_Scripting)_Prevention_Cheat_Sheet#RULE_.233.1_-_HTML_escape_JSON_values_in_an_HTML_context_and_read_the_data_with_JSON.parse
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	// Allow CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// extract iban parameter
	iban := ps.ByName("iban")

	// check for additional request parameters
	validateBankCodeQueryParam := r.FormValue("validateBankCode")
	config["validateBankCode"] = toBoolean(validateBankCodeQueryParam)

	// check for additional request parameters
	getBicQueryParam := r.FormValue("getBIC")
	config["getBIC"] = toBoolean(getBicQueryParam)

	// hit the cache
	value, found := hitCache(iban + strconv.FormatBool(config["getBIC"]) + strconv.FormatBool(config["validateBankCode"]))
	if found {
		fmt.Fprintf(w, value)
		return
	}

	// no value for request parameter
	// return HTTP 400
	if len(iban) == 0 {
		res, _ := json.MarshalIndent(goiban.NewValidationResult(false, "Empty request.", iban), "", "  ")
		strRes = string(res)
		w.Header().Add("Content-Length", strconv.Itoa(len(strRes)))
		// put to cache and render
		// c.Set(iban, strRes, 0)
		http.Error(w, strRes, http.StatusBadRequest)
		return
	}

	// IBAN is not parseable
	// return HTTP 200
	parserResult := goiban.IsParseable(iban)

	if !parserResult.Valid {
		res, _ := json.MarshalIndent(goiban.NewValidationResult(false, "Cannot parse as IBAN: "+parserResult.Message, iban), "", "  ")
		strRes = string(res)
		w.Header().Add("Content-Length", strconv.Itoa(len(strRes)))

		// put to cache and render
		c.Set(iban+strconv.FormatBool(config["getBIC"])+strconv.FormatBool(config["validateBankCode"]), strRes, 0)
		fmt.Fprintf(w, strRes)
		return
	}

	// Try to validate
	parsedIban := goiban.ParseToIban(iban)
	result := parsedIban.Validate()

	// intermediate result
	if len(config) > 0 {
		result = additionalData(parsedIban, result, config)
	}

	res, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Println(err)
	}

	strRes = string(res)
	w.Header().Add("Content-Length", strconv.Itoa(len(strRes)))
	// put to cache and render

	key := iban + strconv.FormatBool(config["getBIC"]) + strconv.FormatBool(config["validateBankCode"])

	c.Set(key, strRes, 0)
	fmt.Fprintf(w, strRes)
	return
}

func toBoolean(value string) bool {
	switch value {
	case "1":
		return true
	case "true":
		return true
	default:
		return false
	}
}

func additionalData(iban *goiban.Iban, intermediateResult *goiban.ValidationResult, config map[string]bool) *goiban.ValidationResult {
	validateBankCode, ok := config["validateBankCode"]
	if ok && validateBankCode {
		intermediateResult = goiban.ValidateBankCode(iban, intermediateResult, repo)
	}

	getBic, ok := config["getBIC"]
	if ok && getBic {
		intermediateResult = goiban.GetBic(iban, intermediateResult, repo)
	}
	return intermediateResult
}

func hitCache(iban string) (string, bool) {
	val, ok := c.Get(iban)
	if ok {
		return val.(string), ok
	}

	return "", false

}
