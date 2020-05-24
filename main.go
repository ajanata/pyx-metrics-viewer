/**
 * Copyright (c) 2018, Andy Janata
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without modification, are permitted
 * provided that the following conditions are met:
 *
 * * Redistributions of source code must retain the above copyright notice, this list of conditions
 *   and the following disclaimer.
 * * Redistributions in binary form must reproduce the above copyright notice, this list of
 *   conditions and the following disclaimer in the documentation and/or other materials provided
 *   with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR
 * IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
 * FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR
 * CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY
 * WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")
var logFormat = logging.MustStringFormatter(`%{color}%{time:15:04:05.000} %{level:.5s} %{id:03x} %{shortfunc} (%{shortfile}) %{color:reset}>%{message}`)
var config *Config

type endpointHandler interface {
	prepareStatements(*sql.DB) error
	registerEndpoints(*gin.Engine)
}

var handlers []endpointHandler

func registerHandler(handler endpointHandler) {
	handlers = append(handlers, handler)
}

func main() {
	config = loadConfig()

	backendStdErr := logging.NewLogBackend(os.Stderr, "", 0)
	formattedStdErr := logging.NewBackendFormatter(backendStdErr, logFormat)
	stdErrLeveled := logging.AddModuleLevel(formattedStdErr)
	level, err := logging.LogLevel(config.LogLevel)
	if err != nil {
		fmt.Printf("Unable to configure logging: %s", err)
		return
	}
	stdErrLeveled.SetLevel(level, "")
	logging.SetBackend(stdErrLeveled)

	if config.RunDebugServer {
		go func() {
			log.Info(http.ListenAndServe("localhost:4680", nil))
		}()
	}

	// TODO not suck
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s sslmode=disable",
		config.Database.Username, config.Database.Password, config.Database.DbName,
		config.Database.Host)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Errorf("Unable to connect to db: %v", err)
		return
	}

	// prepare statements
	for _, handler := range handlers {
		err = handler.prepareStatements(db)
		if err != nil {
			log.Errorf("Unable to prepare statement: %v", err)
			return
		}
	}

	// configure router
	r := gin.Default()

	r.SetFuncMap(template.FuncMap{
		"noescape": noescape,
	})
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "static")
	// register all handlers
	for _, handler := range handlers {
		handler.registerEndpoints(r)
	}
	r.Run(":4080")
}

func noescape(value interface{}) template.HTML {
	return template.HTML(fmt.Sprint(value))
}

func returnError(c *gin.Context, status int, msg string) {
	log.Errorf("Returning error (%d) for request (%s): %s", status, c.Request.URL, msg)
	if strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
		c.String(status, msg)
	} else {
		c.JSON(status, gin.H{"error": msg})
	}
}
