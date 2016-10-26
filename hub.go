// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"log"
)

// hub maintains the set of active connections and broadcasts messages to the
// connections.
type Hub struct {
	// Registered connections.
	connections map[string]*Conn

	// Inbound messages from the connections.
	broadcast chan ConnMessage

	// Register requests from the connections.
	register chan *Conn

	// Unregister requests from connections.
	unregister chan *Conn

	rooms map[string][]*Conn
}

var hub = Hub{
	broadcast:   make(chan ConnMessage),
	register:    make(chan *Conn),
	unregister:  make(chan *Conn),
	connections: make(map[string]*Conn),
	rooms:       make(map[string][]*Conn),
}

func (h *Hub) run() {
	for {
		select {
		case conn := <-h.register:
			log.Printf("register id:%v", conn.id)
			h.connections[conn.id] = conn
		case conn := <-h.unregister:
			log.Printf("unregister id:%v", conn.id)
			h.CleanConn(conn)
		case message := <-h.broadcast:
			log.Printf("rcv event:%v id:%v data:%v", message.EventName, message.conn.id, message.Data)
			switch message.EventName {
			case "__join":
				h.Join(message)
			case "__ice_candidate":
				h.IceCandidate(message)
			case "__offer":
				h.Offer(message)
			case "__answer":
				h.Answer(message)
			}
		}
	}
}

func (h *Hub) CleanConn(conn *Conn) error {
	if nil == conn {
		log.Print("nil == conn")
		return errors.New("nil == conn")
	}
	if _, ok := h.connections[conn.id]; ok { //清理connections
		delete(h.connections, conn.id)
		close(conn.send)
	}
	if room, ok := h.rooms[conn.roomID]; ok { //清理room
		for i, c := range room {
			if c.id == conn.id {
				room = append(room[:i], room[i+1:]...)
				h.rooms[conn.roomID] = room
				break
			}
		}
	}
	for _, c := range h.connections { //告诉客户端清理自己的队列
		if c.id == conn.id {
			continue
		}
		c.send <- ConnMessage{
			EventName: "_remove_peer",
			Data: struct {
				SocketId string `form:"socketId" json:"socketId" binding:"required"`
			}{SocketId: conn.id},
		}
	}

	return nil
}

//for join event
func (h *Hub) Join(message ConnMessage) error {
	if nil == message.Data {
		log.Print("nil == message.Data")
		return errors.New("nil == message.Data")
	}
	ids := make([]string, 0)
	roomData := message.Data.(map[string]interface{})
	roomID := roomData["room"].(string)
	if _, ok := h.rooms[roomID]; !ok {
		h.rooms[roomID] = make([]*Conn, 0)
	}
	room := h.rooms[roomID]
	for _, c := range room {
		if c.id == message.conn.id {
			continue
		}
		ids = append(ids, c.id)
		c.send <- ConnMessage{
			EventName: "_new_peer",
			Data: struct {
				SocketId string `form:"socketId" json:"socketId" binding:"required"`
			}{SocketId: message.conn.id},
		}
	}
	h.rooms[roomID] = append(h.rooms[roomID], message.conn)
	message.conn.roomID = roomID
	message.conn.send <- ConnMessage{
		EventName: "_peers",
		Data: struct {
			Connections []string `form:"connections" json:"connections" binding:"required"`
			You         string   `form:"you" json:"you" binding:"required"`
		}{Connections: ids, You: message.conn.id},
	}
	return nil
}

//for __ice_candidate event
func (h *Hub) IceCandidate(message ConnMessage) error {
	if nil == message.Data {
		log.Print("nil == message.Data")
		return errors.New("nil == message.Data")
	}
	iceCandidateData := message.Data.(map[string]interface{})
	candidate := iceCandidateData["candidate"]
	socketId := iceCandidateData["socketId"].(string)
	if c, ok := h.connections[socketId]; ok {
		c.send <- ConnMessage{
			EventName: "_ice_candidate",
			Data: struct {
				Candidate interface{} `form:"candidate" json:"candidate" binding:"required"`
				SocketId  string      `form:"socketId" json:"socketId" binding:"required"`
			}{Candidate: candidate, SocketId: message.conn.id},
		}
	}
	return nil
}

//for __offer event
func (h *Hub) Offer(message ConnMessage) error {
	if nil == message.Data {
		log.Print("nil == message.Data")
		return errors.New("nil == message.Data")
	}
	offerData := message.Data.(map[string]interface{})
	sdp := offerData["sdp"]
	socketId := offerData["socketId"].(string)
	if c, ok := h.connections[socketId]; ok {
		c.send <- ConnMessage{
			EventName: "_offer",
			Data: struct {
				Sdp      interface{} `form:"sdp" json:"sdp" binding:"required"`
				SocketId string      `form:"socketId" json:"socketId" binding:"required"`
			}{Sdp: sdp, SocketId: message.conn.id},
		}
	}
	return nil
}

//for __answer event
func (h *Hub) Answer(message ConnMessage) error {
	if nil == message.Data {
		log.Print("nil == message.Data")
		return errors.New("nil == message.Data")
	}
	answerData := message.Data.(map[string]interface{})
	sdp := answerData["sdp"]
	socketId := answerData["socketId"].(string)
	if c, ok := h.connections[socketId]; ok {
		c.send <- ConnMessage{
			EventName: "_answer",
			Data: struct {
				Sdp      interface{} `form:"sdp" json:"sdp" binding:"required"`
				SocketId string      `form:"socketId" json:"socketId" binding:"required"`
			}{Sdp: sdp, SocketId: message.conn.id},
		}
	}
	return nil
}
