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
	"fmt"
	"github.com/gin-gonic/gin"
	"strings"
)

type CardMeta struct {
	Color string
	Draw  int
	Pick  int
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
}

func getRound(c *gin.Context) {
	rows, err := getRoundStmt.Query(c.Param("id"))
	if err != nil {
		fmt.Printf("Unable to query for id %s: %v\n", c.Param("id"), err)
		c.JSON(500, gin.H{"error": err})
		return
	}
	// this is the same in every row, so it's fine to re-set it every row
	blackText := ""
	round := Round{}
	temp := []Card{}
	lastWasWinner := false
	for rows.Next() {
		var whiteIndex int
		var whiteText string
		var winner bool
		rows.Scan(&blackText, &whiteIndex, &whiteText, &winner)
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
				Text: whiteText,
				Meta: CardMeta{Color: "white"},
			}}
		} else {
			// we're in the middle of a play
			temp = append(temp, Card{
				Text: whiteText,
				Meta: CardMeta{Color: "white"},
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
	if blackText != "" {
		round.BlackCard = Card{
			Text: blackText,
			Meta: CardMeta{Color: "black"},
		}
		if strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
			c.HTML(200, "round", round)
		} else {
			c.JSON(200, round)
		}
	} else {
		if strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
			c.String(404, "That round cannot be found. If you just played it, wait a few seconds and try again.")
		} else {
			c.JSON(404, gin.H{
				"error": "That round cannot be found.",
			})
		}
	}
}
