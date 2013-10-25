package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/robertknight/robopi/robotarm"
	"github.com/thoj/go-ircevent"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var moveMap = map[string]robotarm.MoveType{
	"base/left":     robotarm.BaseLeft,
	"base/right":    robotarm.BaseRight,
	"grip/open":     robotarm.GripOpen,
	"grip/close":    robotarm.GripClose,
	"wrist/up":      robotarm.WristUp,
	"wrist/down":    robotarm.WristDown,
	"shoulder/up":   robotarm.ShoulderUp,
	"shoulder/down": robotarm.ShoulderDown,
	"elbow/up":      robotarm.ElbowUp,
	"elbow/down":    robotarm.ElbowDown,
}

func setupIrc(server string, secure bool) (*irc.Connection, error) {
	conn := irc.IRC("robopi", "robopi")
	conn.UseTLS = secure
	err := conn.Connect(server)
	if err != nil {
		fmt.Println("Failed to connect: " + err.Error())
		return conn, err
	}

	loggedIn := false
	conn.AddCallback("PRIVMSG", func(e *irc.Event) {
		fmt.Println("PRIVMSG: " + e.Message)
	})
	conn.AddCallback("NOTICE", func(e *irc.Event) {
		if !loggedIn && strings.Contains(e.Message, "This nickname is registered") {
			conn.Privmsg("NickServ", "IDENTIFY robopi pi")
			conn.Join("#robopi")
			loggedIn = true
		}
		fmt.Println("NOTICE: " + e.Message)
	})

	go func() {
		e := <-conn.Error
		fmt.Println("IRC error: " + e.Error())
	}()

	return conn, nil
}

func parseDanceMove(bodyPart string, direction string, duration float64) (robotarm.Move, error) {
	moveStr := bodyPart + "/" + direction
	moveType := moveMap[moveStr]

	if moveType == nil {
		return robotarm.Move{}, errors.New("Unknown move")
	}

	return robotarm.Move{
		moveType,
		time.Duration(int64(duration * float64(time.Second))),
	}, nil
}

type botState struct {
	conn        *irc.Connection
	arm         robotarm.Arm
	currentMove string
	dances      map[string][]robotarm.Move
}

func handleCommand(cmds []string, state *botState, reply func(msg string)) {
	if len(cmds) > 0 {
		switch cmds[0] {
		case "teach":
			if len(cmds) > 1 {
				name := cmds[1]
				if state.dances[name] != nil {
					reply("That's old hat - I already know '" + name + "'")
					reply("Use 'forget " + name + "' if you want to teach me again")
				} else {
					state.currentMove = name
					reply("Teach me the '" + state.currentMove + "' dance!")
					reply("Use 'move <body part> <direction> <duration>' for each move and " +
						"'done' when you're finished :)")
				}
			} else {
				reply("I need the name of a dance to learn! - Use 'teach <dance>'!")
			}
		case "move":
			if len(cmds) > 3 {
				bodyPart := cmds[1]
				direction := cmds[2]
				duration, _ := strconv.ParseFloat(cmds[3], 64)

				move, err := parseDanceMove(bodyPart, direction, duration)
				if err != nil {
					reply("I don't know that move :(")

					var moveList []string
					for k, _ := range moveMap {
						partDir := strings.Split(k, "/")
						moveList = append(moveList, partDir[0]+" "+partDir[1])
					}
					sort.StringSlice(moveList).Sort()

					reply("I do know: " + strings.Join(moveList, ", "))

				} else {
					reply("OK!")
					if len(state.currentMove) > 0 {
						state.dances[state.currentMove] = append(state.dances[state.currentMove], move)
					} else {
						err := state.arm.Move([]robotarm.Move{move})
						if err != nil {
							reply("Oh dear - my arm failed me :(")
						}
					}
				}
			} else {
				reply("I need a move to do! - Use 'move <body part> <direction> <duration>'")
			}
		case "done":
			reply("Use 'dance " + state.currentMove + "' to see this!")
			state.currentMove = ""
		case "dance":
			if len(cmds) > 1 {
				name := cmds[1]
				if len(state.dances[name]) > 0 {
					err := state.arm.Move(state.dances[name])
					if err != nil {
						reply("Oh dear - my arm didn't work :(")
					}
				} else {
					reply("I don't know that :( - Use 'teach " + name + "' to teach me")
				}
			} else {
				var keys []string
				for k, _ := range state.dances {
					keys = append(keys, k)
				}
				sort.StringSlice(keys).Sort()
				reply("I need the name of a dance to do! - I know these ones: " + strings.Join(keys, ", "))
			}
		case "forget":
			if len(cmds) > 1 {
				name := cmds[1]
				delete(state.dances, name)
			}
		case "join":
			if len(cmds) > 1 {
				state.conn.Join(cmds[1])
			} else {
				reply("I need the name of a channel to join")
			}
		case "leave":
			if len(cmds) > 1 {
				state.conn.Part(cmds[1])
			} else {
				reply("I need the name of a channel to leave")
			}
		case "echo":
			reply(strings.Join(cmds[1:], " "))
		default:
			reply("I don't understand '" + cmds[0] + "'")
			reply("Use 'teach', 'move' or 'dance'")
		}
	}
}

func main() {
	arm, err := robotarm.Open()
	if err != nil {
		panic("Unable to setup robot arm")
	}

	secureFlag := flag.Bool("secure", false, "Use a secure connection")
	flag.Parse()

	if len(os.Args) < 2 {
		fmt.Println("I need the name of a server to join")
		os.Exit(1)
	}

	server := flag.Args()[0]
	if !strings.Contains(server,":") {
		server += ":6667"
	}
	fmt.Println("Joining " + server)

	conn, err := setupIrc(server, *secureFlag)
	if err != nil {
		fmt.Println("Unable to connect to " + server)
		os.Exit(1)
	}

	state := botState{
		conn:        conn,
		currentMove: "",
		arm:         arm,
		dances:      map[string][]robotarm.Move{},
	}

	conn.AddCallback("PRIVMSG", func(e *irc.Event) {
		replyFunc := func(msg string) {
			conn.Privmsg(e.Nick, msg)
		}

		nickStr := "robopi:"
		cmdIndex := strings.Index(e.Message, nickStr)
		if cmdIndex != -1 {
			cmdStr := e.Message[cmdIndex+len(nickStr):]
			cmds := strings.Fields(cmdStr)
			handleCommand(cmds, &state, replyFunc)
		}
	})

	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		line := input.Text()
		replyFunc := func(msg string) {
			fmt.Println("reply: " + msg)
		}
		cmds := strings.Fields(line)
		handleCommand(cmds, &state, replyFunc)
	}
}
