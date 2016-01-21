// +build darwin freebsd netbsd openbsd

package main

import (
    // "io"
    // "os"
    "bufio"
    "bytes"
    "fmt"
    "os/exec"
    "strconv"
    "strings"
    "time"

    "./telebot"
    // "github.com/botanio/sdk/go"
    "github.com/op/go-logging"

    // "gopkg.in/tomb.v2" // Sync coroutines
)

var log = logging.MustGetLogger("astgbot")
var format = logging.MustStringFormatter(
    `%{color}%{time:15:04:05.000} %{pid} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

type Command struct {
    functor  interface{}
    help     string
    detailed string
    name     string
}

var cmd_exec_buffers = make(map[string]*bytes.Buffer)
var commands = make(map[string]*Command)

func NewCommand(name string, help string, detailed string, functor interface{}) {
    commands[name] = &Command{name: name, help: help, detailed: detailed, functor: functor}
}

// type Metrics struct {
//     MessagesSent    int
//     MessagesRcvd    int
// }
// var botanio = botan.New("t3MgTwArZSbWcEtBEH8:v7H3oW6J_gx8")

func cmd_bg(options []string, bot *telebot.Bot, message telebot.Message) {
    bot.SendMessage(message.Chat, fmt.Sprintf("Executing background task (%i): %s", strings.Join(options, " ")), nil)
}

func cmd_exec(options []string, bot *telebot.Bot, message telebot.Message) {
    bot.SendMessage(message.Chat, "Executing: "+strings.Join(options, " "), nil)

    cmd := exec.Command(options[0], options[1:]...)
    cmdReader, err := cmd.StdoutPipe()
    if err != nil {
        bot.SendMessage(message.Chat, fmt.Sprintf("Error creating StdoutPipe for Cmd", err), nil)
        return
    }

    scanner := bufio.NewScanner(cmdReader)
    buffer := bytes.NewBufferString("")

    go func() {
        for scanner.Scan() {
            buffer.WriteString(scanner.Text())
            buffer.WriteString("\n")
        }
    }()

    ticker := time.NewTicker(time.Second * 2)
    sendOutput := func() {
        if buffer.Len() > 1000 {
            uuid, _ := telebot.NewV4().Value()
            id_key := string(fmt.Sprintf("%s", uuid)[:8])
            bot.SendMessage(message.Chat, fmt.Sprintf("Output is very big to show, it saved to buffer #%s.\nTo receive buffer you need to use /cmd_buf_read LENGTH BUFFERNAME.\nFor example: /cmd_buf_read 500 %s\n", id_key, id_key), nil)
            cmd_exec_buffers[id_key] = buffer
            ticker.Stop()

        } else {
            bufferData := buffer.String()
            buffer.Reset()
            out := fmt.Sprintf("`%s`", bufferData)
            bot.SendMessage(message.Chat, out, &telebot.SendOptions{
                ParseMode: "markdown",
            })
        }
    }
    go func() {
        for range ticker.C {
            go sendOutput()
        }
    }()

    err = cmd.Start()
    if err != nil {
        bot.SendMessage(message.Chat, fmt.Sprintf("Error starting Cmd: %s", err), nil)
        return
    }

    err = cmd.Wait()
    ticker.Stop()
    go sendOutput()
    if err != nil {
        bot.SendMessage(message.Chat, fmt.Sprintf("Error waiting for Cmd: %s", err), nil)
        return
    }
}

func cmd_buf_read(options []string, bot *telebot.Bot, message telebot.Message) {
    if len(options) == 2 {
        var length int
        if i, err := strconv.ParseInt(options[0], 10, 64); err == nil {
            length = int(i)
            idkey := string(options[1])

            if buffer, ok := cmd_exec_buffers[idkey]; ok {
                bufferData := string(buffer.Next(length))
                out := fmt.Sprintf("`%s`", bufferData)
                bot.SendMessage(message.Chat, out, &telebot.SendOptions{
                    ParseMode: "markdown",
                })
            }
        }
    }
}

func cmd_buf_clean(options []string, bot *telebot.Bot, message telebot.Message) {
    if len(options) == 1 {
        idkey := string(options[0])
        if buffer, ok := cmd_exec_buffers[idkey]; ok {
            buffer.Reset()
            bot.SendMessage(message.Chat, fmt.Sprintf("Buffer #%s cleaned.", idkey), nil)
        }
    }
}

func cmd_hi(options []string, bot *telebot.Bot, message telebot.Message) {
    bot.SendMessage(message.Chat, "Hello, "+message.Sender.FirstName+"!", nil)
}

func cmd_help(options []string, bot *telebot.Bot, message telebot.Message) {
    buffer := bytes.NewBufferString("HELP:\n")
    for key, value := range commands {
        buffer.WriteString(fmt.Sprintf("/%s\t%s\n", key, value.help))
    }
    bot.SendMessage(message.Chat, buffer.String(), nil)
}

func command(cmd string, options []string, bot *telebot.Bot, message telebot.Message) {
    if command, ok := commands[cmd[1:]]; ok {
        // fc := func(command.functor) interface{}
        go command.functor.(func(options []string, bot *telebot.Bot, message telebot.Message))(options, bot, message)
    } else {
        bot.SendMessage(message.Chat, "Unknown command "+cmd, nil)
    }
}

func init() {
    NewCommand("hi", "Print hello to you", "", cmd_hi)
    NewCommand("exec", "Executes command", "[command]", cmd_exec)
    NewCommand("cmd_buf_read", "Reads buffer of executed command", "LENGTH IDENTIFIER", cmd_buf_read)
    NewCommand("cmd_buf_clean", "Clean buffer of executed command", "IDENTIFIER", cmd_buf_clean)
    NewCommand("help", "Prints help", "[COMMAND]", cmd_help)
}

func main() {
    logging.SetFormatter(format)

    bot, err := telebot.NewBot("177634110:AAEV0Ml2id43zUd5w-qSwl7wKyjCycQ9zN0")
    if err != nil {
        return
    }
    bot.Botan.Token = "t3MgTwArZSbWcEtBEH8:v7H3oW6J_gx8"

    log.Info("starting")

    messages := make(chan telebot.Message)
    bot.Listen(messages, 1*time.Second)

    for message := range messages {
        words := strings.Fields(message.Text)

        if len(words) >= 1 {
            if words[0][0] == '/' {
                // Operate with a command
                log.Debug("Command received: %s", words[0])
                go command(words[0], words[1:], bot, message)

            } else {
                log.Debug("Message: %s", message.Text)
                bot.SendMessage(message.Chat, "Use /help to get all available commands.", nil)
                // This is a just simple text
                // Ignoring
            }
        }

    }
}
