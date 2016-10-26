// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/pborman/uuid"
	"log"
	"net/http"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 51200
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Conn is an middleman between the websocket connection and the hub.
type Conn struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send   chan ConnMessage
	id     string
	roomID string
}

type ConnMessage struct {
	EventName string      `form:"eventName" json:"eventName" binding:"required"`
	Data      interface{} `form:"data" json:"data" binding:"required"`
	conn      *Conn
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Conn) readPump() {
	defer func() {
		hub.unregister <- c
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			log.Printf("%v read message error:%v", c.id, err.Error())
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Print(err.Error())
			}
			break
		}
		connMessage := ConnMessage{conn: c}
		if err = json.Unmarshal([]byte(message), &connMessage); nil != err {
			log.Printf("%s %v", message, err.Error())
		} else {
			hub.broadcast <- connMessage
		}
	}
}

// write writes a message with the given message type and payload.
func (c *Conn) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case connMessage, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			connMessage.conn = nil
			message := new(bytes.Buffer)
			if err := json.NewEncoder(message).Encode(connMessage); err != nil {
				log.Printf("%s %v", message, err.Error())
			} else {
				log.Printf("send to id:%v message:%v", c.id, message)
				if err := c.write(websocket.TextMessage, message.Bytes()); err != nil {
					return
				}
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(err.Error())
		return
	}
	conn := &Conn{send: make(chan ConnMessage, 256), ws: ws, id: uuid.New()}
	hub.register <- conn
	go conn.writePump()
	conn.readPump()
}
