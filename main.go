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

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	config := loadConfig()

	// TODO not suck
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s sslmode=disable",
		config.Database.Username, config.Database.Password, config.Database.DbName,
		config.Database.Host)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("Unable to connect to db: %v\n", err)
		return
	}

	// prepare statements
	err = prepareRoundStmts(db)
	if err != nil {
		fmt.Printf("Unable to prepare statement: %v\n", err)
		return
	}

	// configure router
	r := gin.Default()
	r.GET("/round/:id", getRound)

	r.SetFuncMap(template.FuncMap{
		"html": html,
	})
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "static")
	r.Run(":4080")
}

func html(value interface{}) template.HTML {
	return template.HTML(fmt.Sprint(value))
}
