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

var getSessionInfoStmt *sql.Stmt
var getSessionPlayedRoundsStmt *sql.Stmt
var getSessionJudgedRoundsStmt *sql.Stmt

type SessionMeta struct {
	LogInTimestamp int64
	PersistentId   string
	PlayedRounds   []RoundMeta
	JudgedRounds   []RoundMeta
}

type sessionHandler struct{}

func (session *SessionMeta) FormattedTimestamp() string {
	//	return time.Unix(session.LogInTimestamp, 0).UTC().Format("Mon, 02 Jan 2006 15:04:05") + " PDT -0700"
	return time.Unix(session.LogInTimestamp, 0).UTC().Format(time.RFC1123)
}

func init() {
	log.Debug("Registering session handler")
	registerHandler(sessionHandler{})
}

func (h sessionHandler) registerEndpoints(r *gin.Engine) {
	log.Debug("Registering endpoint for session handler")
	r.GET("/session/:id", getSession)
}

func (h sessionHandler) prepareStatements(db *sql.DB) error {
	log.Debug("Preparing statements for session handler")
	var err error
	getSessionInfoStmt, err = db.Prepare("SELECT ((meta).timestamp AT TIME ZONE 'UTC'), persistent_id " +
		"FROM user_session " +
		"WHERE session_id = $1 " +
		"ORDER BY ((meta).timestamp) DESC")
	if err != nil {
		return err
	}
	getSessionPlayedRoundsStmt, err = db.Prepare("SELECT bc.text, bc.watermark, bc.pick, bc.draw, rc.round_id, ((rc.meta).timestamp AT TIME ZONE 'UTC') " +
		"FROM round_complete__user_session__white_card jt " +
		"JOIN round_complete rc ON rc.uid = jt.round_complete_uid " +
		"JOIN black_card bc ON bc.uid = rc.black_card_uid " +
		"WHERE jt.session_id = $1 AND jt.white_card_index = 0 " +
		"ORDER BY ((rc.meta).timestamp) DESC")
	if err != nil {
		return err
	}
	getSessionJudgedRoundsStmt, err = db.Prepare("SELECT bc.text, bc.watermark, bc.pick, bc.draw, rc.round_id, ((rc.meta).timestamp AT TIME ZONE 'UTC') " +
		"FROM round_complete rc " +
		"JOIN black_card bc ON bc.uid = rc.black_card_uid " +
		"WHERE rc.judge_session_id = $1 " +
		"ORDER BY ((rc.meta).timestamp) DESC")

	return err
}

func getSession(c *gin.Context) {
	q, err := getSessionInfoStmt.Query(c.Param("id"))
	if err != nil {
		returnError(c, 500, fmt.Sprintf("Unable to query for session with id %s: %v", c.Param("id"), err))
		return
	}
	defer q.Close()
	session := SessionMeta{}
	if !q.Next() {
		msg := "ID not found"
		if q.Err() != nil {
			log.Error("While processing result for session %s: %v", c.Param("id"), q.Err())
			msg = q.Err().Error()
		}
		returnError(c, 500, fmt.Sprintf("Unable to query for session with id %s: %s", c.Param("id"), msg))
		return
	}
	var timestamp time.Time
	q.Scan(&timestamp, &session.PersistentId)
	session.LogInTimestamp = timestamp.Unix()
	session.PlayedRounds, err = getSessionRounds(getSessionPlayedRoundsStmt.Query(c.Param("id")))
	if err != nil {
		returnError(c, 500, fmt.Sprintf("Unable to query for session with id %s: %v", c.Param("id"), err))
		return
	}
	session.JudgedRounds, err = getSessionRounds(getSessionJudgedRoundsStmt.Query(c.Param("id")))
	if err != nil {
		returnError(c, 500, fmt.Sprintf("Unable to query for session with id %s: %v", c.Param("id"), err))
		return
	}

	if strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
		c.HTML(200, "session", &session)
	} else {
		c.JSON(200, session)
	}
}

func getSessionRounds(q *sql.Rows, err error) ([]RoundMeta, error) {
	rounds := []RoundMeta{}
	if err != nil {
		return rounds, err
	}
	defer q.Close()
	for q.Next() {
		var text string
		var watermark string
		var pick int16
		var draw int16
		var roundId string
		var timestamp time.Time
		q.Scan(&text, &watermark, &pick, &draw, &roundId, &timestamp)
		rounds = append(rounds, RoundMeta{
			BlackCard: Card{
				Text:      text,
				Watermark: watermark,
				Meta: CardMeta{
					Color: "black",
					Draw:  draw,
					Pick:  pick,
				},
			},
			RoundId:   roundId,
			Timestamp: timestamp.Unix(),
		})
	}

	return rounds, q.Err()
}
