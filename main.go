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
	_ "github.com/lib/pq"
)

type RoundInfo struct {
	BlackCard   string
	WinningPlay []string
	OtherPlays  [][]string
}

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

	r := gin.Default()
	getRoundStmt, err := db.Prepare("SELECT bc.text, jt.white_card_index i, wc.text, (rc.winner_session_id = jt.session_id) winner " +
		"FROM round_complete rc " +
		"JOIN round_complete__user_session__white_card jt ON jt.round_complete_uid = rc.uid " +
		"JOIN white_card wc ON wc.uid = jt.white_card_uid " +
		"JOIN black_card bc ON bc.uid = rc.black_card_uid " +
		"WHERE rc.round_id = $1" +
		"ORDER BY jt.session_id, jt.white_card_index ASC")
	if err != nil {
		fmt.Printf("Unable to prepare statement: %v\n", err)
		return
	}
	r.GET("/round/:id", func(c *gin.Context) {
		rows, err := getRoundStmt.Query(c.Param("id"))
		if err != nil {
			fmt.Printf("Unable to query for id %s: %v\n", c.Param("id"), err)
			c.JSON(500, gin.H{"error": err})
			return
		}
		// this is the same in every row, so it's fine to re-set it every row
		round := RoundInfo{}
		temp := []string{}
		lastWasWinner := false
		for rows.Next() {
			var whiteIndex int
			var whiteText string
			var winner bool
			rows.Scan(&round.BlackCard, &whiteIndex, &whiteText, &winner)
			if whiteIndex == 0 {
				// we're at the start of a new play
				if len(temp) > 0 {
					if lastWasWinner {
						round.WinningPlay = temp
					} else {
						round.OtherPlays = append(round.OtherPlays, temp)
					}
				}
				temp = []string{whiteText}
			} else {
				// we're in the middle of a play
				temp = append(temp, whiteText)
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
		if round.BlackCard != "" {
			c.JSON(200, round)
		} else {
			c.JSON(404, gin.H{
				"error": "That round cannot be found.",
			})
		}
	})

	r.Static("/static", "static")
	r.Run(":4080")
}
