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

func addLoginHandlers(conn *irc.Connection) {
	conn.AddCallback("PRIVMSG", func(e *irc.Event) {
		fmt.Println("PRIVMSG: " + e.Message)
	})
	conn.AddCallback("NOTICE", func(e *irc.Event) {
		if strings.Contains(e.Message, "This nickname is registered") {
			conn.Privmsg("NickServ", "IDENTIFY robopi pi")
			conn.Join("#robopi")
		}
		fmt.Println("NOTICE: " + e.Message)
	})
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

type ArmMover interface {
	Move(moves []robotarm.Move) (error)
}

type botState struct {
	conn        *irc.Connection
	arm         ArmMover
	currentMove string
	dances      map[string][]robotarm.Move
}

type fakeArm struct {
}

func (*fakeArm) Move(moves []robotarm.Move)(error) {
	fmt.Println("Executing fake arm move")
	return nil
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

type botMessage struct {
	event irc.Event
	commands []string
}

func main() {
	var arm ArmMover
	realArm, err := robotarm.Open()
	if err != nil {
		fmt.Println("Unable to setup robot arm. Using a fake arm instead.")
		arm = &fakeArm{}
	} else {
		arm = &realArm
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

	conn := irc.IRC("robopi", "robopi")
	conn.UseTLS = *secureFlag
	addLoginHandlers(conn)

	go func(errChan chan error) {
		e := <-errChan
		fmt.Println("IRC error: " + e.Error())
	}(conn.Error)

	state := botState{
		conn:        conn,
		currentMove: "",
		arm:         arm,
		dances:      map[string][]robotarm.Move{},
	}

	botMessages := make(chan botMessage)
	quitChan := make(chan bool)
	conn.AddCallback("PRIVMSG", func(e *irc.Event) {
		nickStr := "robopi:"
		cmdIndex := strings.Index(e.Message, nickStr)
		if cmdIndex != -1 {
			cmdStr := e.Message[cmdIndex+len(nickStr):]
			cmds := strings.Fields(cmdStr)
			botMessages <-botMessage{
				event: *e,
				commands: cmds,
			}
		}
	})

	err = conn.Connect(server)
	if err != nil {
		fmt.Printf("Unable to connect to %v: %v\n", server, err.Error())
		os.Exit(1)
	}

	go func() {
		input := bufio.NewScanner(os.Stdin)
		for input.Scan() {
			line := input.Text()
			cmds := strings.Fields(line)
			botMessages <-botMessage{
				event:irc.Event{},
				commands:cmds,
			}
		}
		quitChan<-true
	}()

	for {
		select {
		case message := <-botMessages:
			replyFunc := func(msg string) {
				conn.Privmsg(message.event.Nick, msg)
			}
			handleCommand(message.commands, &state, replyFunc)
		case <-quitChan:
			break
		}
	}
}
