package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	STATE_HANDSHAKE = 0
	STATE_STATUS    = 1
	STATE_LOGIN     = 2
)

type Game struct {
	MaxPlayers  int
	players     map[string]*Player
	playersLock sync.Mutex
	out         chan []byte
}

func NewGame(maxPlayers int) *Game {
	return &Game{
		MaxPlayers: maxPlayers,
		players:    make(map[string]*Player),
		out:        make(chan []byte, 256),
	}
}

func (g *Game) Listen(address string) {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}

	go g.send()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
		} else {
			go g.handleConnection(conn)
		}
	}
}

func (g *Game) AddPlayer(name string, conn net.Conn) {
	p := NewPlayer(name, conn, g)

	p.Run()

	g.playersLock.Lock()
	g.players[name] = p
	g.playersLock.Unlock()

	WriteChatMessage(g, name+" joined the game")
}

func (g *Game) RemovePlayer(name string) {
	g.playersLock.Lock()
	delete(g.players, name)
	g.playersLock.Unlock()

	WriteChatMessage(g, name+" left the game")
}

func (g *Game) Broadcast(data []byte) {
	g.playersLock.Lock()
	for _, player := range g.players {
		player.Write(data)
	}
	g.playersLock.Unlock()
}

func (g *Game) Write(data []byte) (int, error) {
	g.out <- data

	return len(data), nil
}

func (g *Game) send() {
	for {
		g.Broadcast(<-g.out)
	}
}

func (g *Game) handleConnection(conn net.Conn) {
	var state uint64 = STATE_HANDSHAKE
	var username string

	fmt.Println("New connection")

	for {
		length, err := ReadVarint(conn)
		if err != nil {
			fmt.Println("Connection died")
			return
		}

		buf := make([]byte, length)
		io.ReadFull(conn, buf)
		r := bytes.NewReader(buf)

		id, _ := binary.ReadUvarint(r)

		switch state {
		case STATE_HANDSHAKE:
			if id != 0x00 {
				continue
			}

			state = ReadHandshake(r).State

		case STATE_STATUS:
			switch id {
			case 0x00:
				g.playersLock.Lock()
				playerCount := len(g.players)
				g.playersLock.Unlock()

				WriteStatus(conn, Status{
					Version{
						Name:     MinecraftVersion,
						Protocol: MinecraftProtocol,
					},
					Players{
						Max:    g.MaxPlayers,
						Online: playerCount,
					},
					Description{"Awesomeness"},
				})

			case 0x01:
				WriteVarint(conn, uint64(len(buf)))
				conn.Write(buf)
			}

		case STATE_LOGIN:
			switch id {
			case 0x00:
				if (len(g.players)+1 > g.MaxPlayers) {
					return
				}
				
				uuid := "de305d54-75b4-431b-adb2-eb6b9e546013"
				username = ReadString(r)

				WriteLoginSuccess(conn, uuid, username)
				WriteSetCompression(conn, 65536)

				g.AddPlayer(username, conn)
				return
			}
		}
	}
}
