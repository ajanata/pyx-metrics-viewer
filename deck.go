/**
 * Copyright (c) 2020, Andy Janata
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
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/gin-gonic/gin"
)

var getDeckInfo *sql.Stmt
var getWhiteCards *sql.Stmt
var getBlackCards *sql.Stmt
var csvTemplate = template.Must(template.ParseFiles("templates/deck.csv"))

type Deck struct {
	Name       string
	ID         string
	WhiteCount int
	BlackCount int
	WhiteCards []Card
	BlackCards []Card
}

type deckHandler struct{}

func init() {
	log.Debug("Registering deck handler")
	registerHandler(deckHandler{})
}

func (h deckHandler) registerEndpoints(r *gin.Engine) {
	log.Debug("Registering endpoints for deck handler")
	r.GET("/deck/:id", getDeck)
	r.GET("/deck/:id/download", downloadDeck)
}

func (h deckHandler) prepareStatements(db *sql.DB) error {
	log.Debug("Preparing statements for deck handler")
	var err error
	getDeckInfo, err = db.Prepare(`
    SELECT "name", white_count, black_count FROM deck WHERE id = $1 ORDER BY uid DESC LIMIT 1
`)
	if err != nil {
		return err
	}
	getWhiteCards, err = db.Prepare(`
    SELECT text FROM white_card WHERE watermark = $1
`)
	if err != nil {
		return err
	}
	getBlackCards, err = db.Prepare(`
    SELECT text, draw, pick FROM black_card WHERE watermark = $1
`)
	return err
}

func loadDeck(strID string) (Deck, int, error) {
	if len(strID) != 5 {
		return Deck{}, http.StatusBadRequest, errors.New("cardcast deck IDs must be 5 characters long")
	}
	// for cardcast, deck ID is the code converted to base 36 and then negated
	id, err := strconv.ParseInt(strID, 36, 64)
	if err != nil || id <= 0 {
		return Deck{}, http.StatusBadRequest, errors.New("cardcast deck IDs must only contain letters and numbers")
	}

	info, err := getDeckInfo.Query(-id)
	if err == sql.ErrNoRows {
		return Deck{}, http.StatusNotFound, errors.New("cardcast deck not found")
	} else if err != nil {
		return Deck{}, http.StatusInternalServerError, errors.New("could not load deck")
	}
	defer info.Close()

	if !info.Next() {
		return Deck{}, http.StatusNotFound, errors.New("cardcast deck not found")
	}

	var numWhite, numBlack int
	var name string
	err = info.Scan(&name, &numWhite, &numBlack)
	if err != nil {
		return Deck{}, http.StatusInternalServerError, errors.New("could not scan deck")
	}

	deck := Deck{
		Name:       name,
		ID:         strID,
		WhiteCount: numWhite,
		BlackCount: numBlack,
	}

	whites, err := getWhiteCards.Query(strID)
	if err != nil {
		return deck, http.StatusInternalServerError, errors.New("could not get white cards")
	}
	defer whites.Close()

	for whites.Next() {
		var text string
		err := whites.Scan(&text)
		if err != nil {
			return deck, http.StatusInternalServerError, errors.New("could not scan white card")
		}
		deck.WhiteCards = append(deck.WhiteCards, Card{
			Text:      text,
			Watermark: strID,
			Meta:      CardMeta{Color: "white"},
		})
	}

	blacks, err := getBlackCards.Query(strID)
	if err != nil {
		return deck, http.StatusInternalServerError, errors.New("could not get black cards")
	}
	defer blacks.Close()

	for blacks.Next() {
		var text string
		var draw, pick int16
		err := blacks.Scan(&text, &draw, &pick)
		if err != nil {
			return deck, http.StatusInternalServerError, errors.New("could not scan black card")
		}
		deck.BlackCards = append(deck.BlackCards, Card{
			Text:      text,
			Watermark: strID,
			Meta: CardMeta{
				Color: "black",
				Draw:  draw,
				Pick:  pick,
			},
		})
	}

	return deck, 0, nil
}

func getDeck(c *gin.Context) {
	strID := c.Param("id")

	deck, status, err := loadDeck(strID)
	if err != nil {
		returnError(c, status, err.Error())
	}

	if strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
		c.HTML(http.StatusOK, "deck", &deck)
	} else {
		c.JSON(http.StatusOK, deck)
	}
}

func downloadDeck(c *gin.Context) {
	strID := c.Param("id")

	deck, status, err := loadDeck(strID)
	if err != nil {
		returnError(c, status, err.Error())
	}

	buf := &bytes.Buffer{}
	err = csvTemplate.ExecuteTemplate(buf, "deck_csv", deck)
	if err != nil {
		returnError(c, http.StatusInternalServerError, "Could not prepare download")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, strID))
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}
