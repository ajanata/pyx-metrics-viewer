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

var getRoundWhiteCards *sql.Stmt
var getRoundInfo *sql.Stmt

type CardMeta struct {
	Color string
	Draw  int16 `json:",omitempty"`
	Pick  int16 `json:",omitempty"`
}

type Card struct {
	Text      string
	Watermark string
	Meta      CardMeta
}

type Round struct {
	BlackCard   Card
	WinningPlay []Card
	OtherPlays  [][]Card
	Timestamp   int64
}

func (round *Round) FormattedTimestamp() string {
	return time.Unix(round.Timestamp, 0).UTC().Format("Mon, 02 Jan 2006 15:04:05") + " PDT -0700"
	//time.RFC1123)
}

func prepareRoundStmts(db *sql.DB) error {
	var err error
	getRoundWhiteCards, err = db.Prepare(
		"SELECT jt.white_card_index, wc.text, wc.watermark, (rc.winner_session_id = jt.session_id) " +
			"FROM round_complete rc " +
			"JOIN round_complete__user_session__white_card jt ON jt.round_complete_uid = rc.uid " +
			"JOIN white_card wc ON wc.uid = jt.white_card_uid " +
			"WHERE rc.round_id = $1" +
			"ORDER BY jt.session_id, jt.white_card_index ASC")
	if err != nil {
		return err
	}
	getRoundInfo, err = db.Prepare("SELECT bc.text, bc.watermark, bc.pick, bc.draw, (rc.meta).timestamp " +
		"FROM round_complete rc " +
		"JOIN black_card bc on bc.uid = rc.black_card_uid " +
		"WHERE rc.round_id = $1")
	return err
}

func getRound(c *gin.Context) {
	info, err := getRoundInfo.Query(c.Param("id"))
	if err != nil {
		exitError(c, 500, fmt.Sprintf("Unable to query for id %s: %v\n", c.Param("id"), err))
		return
	}
	var blackText string
	var blackWatermark string
	var pick int16
	var draw int16
	var timestamp time.Time
	if !info.Next() {
		exitError(c, 404, "That round cannot be found. If you just played it, wait a few seconds and try again.")
		info.Close()
		return
	}
	info.Scan(&blackText, &blackWatermark, &pick, &draw, &timestamp)
	round := Round{
		BlackCard: Card{
			Text:      blackText,
			Watermark: blackWatermark,
			Meta: CardMeta{
				Color: "black",
				Draw:  draw,
				Pick:  pick,
			},
		},
		Timestamp: timestamp.Unix(),
	}
	info.Close()

	rows, err := getRoundWhiteCards.Query(c.Param("id"))
	if err != nil {
		exitError(c, 500, fmt.Sprintf("Unable to query for id %s: %v\n", c.Param("id"), err))
		return
	}
	temp := []Card{}
	lastWasWinner := false
	for rows.Next() {
		var whiteIndex int
		var whiteText string
		var whiteWatermark string
		var winner bool
		rows.Scan(&whiteIndex, &whiteText, &whiteWatermark, &winner)
		if whiteIndex == 0 {
			// we're at the start of a new play
			if len(temp) > 0 {
				if lastWasWinner {
					round.WinningPlay = temp
				} else {
					round.OtherPlays = append(round.OtherPlays, temp)
				}
			}
			temp = []Card{Card{
				Text:      whiteText,
				Watermark: whiteWatermark,
				Meta:      CardMeta{Color: "white"},
			}}
		} else {
			// we're in the middle of a play
			temp = append(temp, Card{
				Text:      whiteText,
				Watermark: whiteWatermark,
				Meta:      CardMeta{Color: "white"},
			})
		}
		lastWasWinner = winner
	}
	// deal with the last (set of) card(s)
	if len(temp) > 0 {
		if lastWasWinner {
			round.WinningPlay = temp
		} else {
			round.OtherPlays = append(round.OtherPlays, temp)
		}
	}
	if rows.Err() != nil {
		// TODO an error while iterating
	}
	rows.Close()
	if strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
		c.HTML(200, "round", &round)
	} else {
		c.JSON(200, round)
	}
}

func exitError(c *gin.Context, status int, msg string) {
	fmt.Printf("%s\n", msg)
	if strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
		c.String(status, msg)
	} else {
		c.JSON(status, gin.H{"error": msg})
	}
}
