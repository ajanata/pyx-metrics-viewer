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
	"github.com/gin-gonic/gin"
	"strings"
	"time"
)

var getUserSessionsStmt *sql.Stmt

type SessionBasics struct {
	SessionId      string
	LogInTimestamp int64
}

type UserMeta struct {
	Sessions []SessionBasics
}

type userHandler struct{}

func (session *SessionBasics) FormattedTimestamp() string {
	//	return time.Unix(session.LogInTimestamp, 0).UTC().Format("Mon, 02 Jan 2006 15:04:05") + " PDT -0700"
	return time.Unix(session.LogInTimestamp, 0).UTC().Format(time.RFC1123)
}

func (session *SessionBasics) ServerId() string {
	return strings.Split(session.SessionId, "_")[0]
}

func init() {
	log.Debug("Registering user handler")
	registerHandler(userHandler{})
}

func (h userHandler) registerEndpoints(r *gin.Engine) {
	log.Debug("Registering endpoint for user handler")
	r.GET("/user/:id", getUser)
}

func (h userHandler) prepareStatements(db *sql.DB) error {
	log.Debug("Preparing statements for user handler")
	var err error
	getUserSessionsStmt, err = db.Prepare("SELECT us.session_id, ((us.meta).timestamp AT TIME ZONE 'UTC') " +
		"FROM user_session us " +
		"WHERE us.persistent_id = $1 " +
		"ORDER BY (us.meta).timestamp DESC")
	return err
}

func getUser(c *gin.Context) {
	q, err := getUserSessionsStmt.Query(c.Param("id"))
	if err != nil {
		returnError(c, 500, fmt.Sprintf("Unable to query for user with id %s: %v", c.Param("id"), err))
		return
	}
	defer q.Close()
	user := UserMeta{}
	for q.Next() {
		var sessionId string
		var timestamp time.Time
		q.Scan(&sessionId, &timestamp)
		user.Sessions = append(user.Sessions, SessionBasics{
			SessionId:      sessionId,
			LogInTimestamp: timestamp.Unix(),
		})
	}
	if q.Err() != nil {
		log.Errorf("Error while iterating over sessions for user %s: %+v", c.Param("id"), q.Err())
	}

	if strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
		c.HTML(200, "user", &user)
	} else {
		c.JSON(200, user)
	}
}
