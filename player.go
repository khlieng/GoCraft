package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

type Player struct {
	Name string
	Game *Game
	conn net.Conn
	out  chan []byte
}

func NewPlayer(name string, conn net.Conn, game *Game) *Player {
	return &Player{
		Name: name,
		Game: game,
		conn: conn,
		out:  make(chan []byte, 256),
	}
}

func (p *Player) Run() {
	WriteJoinGame(p)
	WriteSpawnPosition(p, 0, 10, 0)
	WritePositionAndLook(p, 0, 128, 0)

	go p.send()
	go p.recv()
	go p.gameloop()
}

func (p *Player) Write(data []byte) (int, error) {
	p.out <- data

	return len(data), nil
}

func (p *Player) gameloop() {
	var ticks int64 = 0
	keepAlive := time.Tick(10 * time.Second)
	timeUpdate := time.Tick(time.Second)

	for {
		select {
		case <-keepAlive:
			_, err := p.conn.Write([]byte{3, 0, 0, 0})
			if err != nil {
				return
			}

		case <-timeUpdate:
			ticks += 20

			WriteTimeUpdate(p.conn, ticks, ticks%24000)
		}
	}
}

func (p *Player) send() {
	for {
		p.conn.Write(<-p.out)
	}
}

func (p *Player) recv() {
	for {
		length, err := ReadVarint(p.conn)
		if err != nil {
			p.Game.RemovePlayer(p.Name)
			fmt.Println("Player disconnected")

			return
		}

		buf := make([]byte, length)
		io.ReadFull(p.conn, buf)
		r := bytes.NewReader(buf)

		binary.ReadUvarint(r)

		id, _ := binary.ReadUvarint(r)

		switch id {
		case 0x01:
			message := ReadString(r)
			if strings.HasPrefix(message, "/") {
				// Commands
			} else {
				WriteChatMessage(p.Game, fmt.Sprintf("<%s> %s", p.Name, message))
				fmt.Printf("<%s> %s\n", p.Name, message)
			}
		}
	}
}
