package main

const (
	MinecraftVersion  = "1.8.3"
	MinecraftProtocol = 47
)

func main() {
	game := NewGame(16)
	game.Listen(":25565")
}
