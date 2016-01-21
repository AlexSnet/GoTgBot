package process

import (
    "io"
    // "os"
    "fmt"
    "os/exec"
    "sync"
    "time"
    // "bytes"
    "bufio"
    "container/list"
    "crypto/sha1"
    "strings"
)

type historyEntry struct {
    ts     time.Time
    status string
}

type BufferEntry struct {
    ts   time.Time
    data string
}

type Task struct {
    id         string
    start_time time.Time
    command    []string

    cmd *exec.Cmd

    buffer struct {
        stdout list.List
        stderr list.List
    }

    cancel struct {
        isPending bool
        message   string
    }
}

func (t *Task) CreatePipes() {
    t.buffer.stdout.Init()

    cmdReader, err := t.cmd.StdoutPipe()
    if err != nil {
        panic(err)
    }
    scanner := bufio.NewScanner(cmdReader)
    go func() {
        for scanner.Scan() {
            t.buffer.stdout.PushBack(&BufferEntry{time.Now(), scanner.Text()})
            if t.cancel.isPending {
                return
            }
        }
    }()
}

func (t *Task) Start() {
    // create pipes
    t.CreatePipes()

    err := t.cmd.Start()
    if err != nil {
        panic(err)
    }
}

func (t *Task) Wait() {
    err := t.cmd.Wait()
    if err != nil {
        panic(err)
    }
}

type TaskList struct {
    mu    sync.RWMutex
    tasks map[string]*Task
}

func (tl *TaskList) Start(command []string) *Task {
    // Start time
    start_time := time.Now()

    // Generating id for task
    h := sha1.New()
    io.WriteString(h, fmt.Sprintf("%i::%s", start_time, strings.Join(command, " ")))
    id := fmt.Sprintf("%x", h.Sum(nil))[:6]

    //
    cmdex := exec.Command(command[0], command[1:]...)

    t := &Task{
        id:         id,
        start_time: start_time,
        command:    command,
        cmd:        cmdex,
    }

    tl.mu.Lock()
    if tl.tasks == nil {
        tl.tasks = make(map[string]*Task)
    }
    tl.tasks[id] = t
    tl.mu.Unlock()

    return t
}

// Type CancelErr is the type used for cancellation-induced panics.
type CancelErr string

// Error returns the error message for a CancelErr.
func (e CancelErr) Error() string {
    return string(e)
}

func (t *Task) doCancel() {
    message := "killed"
    if len(t.cancel.message) > 0 {
        message += ": " + t.cancel.message
    }

    panic(CancelErr(message))
}

func (tl *TaskList) done(id string, e interface{}) {
    tl.mu.Lock()
    t, present := tl.tasks[id]
    if present {
        delete(tl.tasks, id)
    }
    tl.mu.Unlock()
    _ = t
    if present {
        // ts := time.Now()

    } else if e != nil {
        _, canceled := e.(CancelErr)
        if !canceled {
            panic(e)
        }
    }
}

var DefaultProclist TaskList

// func main() {
//     DefaultProclist.Start([]string{"ls", "-lah"})

//     t := DefaultProclist.Start([]string{"top"})
//     t.Start()

//     time.Sleep(1 * time.Second)

//     fmt.Printf("Task id: %s\n", t.id)
//     fmt.Printf("Tasks: %s\n", DefaultProclist.tasks)
//     fmt.Printf("Tasks: %n\n", len(DefaultProclist.tasks))

//     time.Sleep(1 * time.Second)

//     for e := t.buffer.stdout.Front(); e != nil; e = e.Next() {
//         m := e.Value.(*BufferEntry)
//         fmt.Printf("%s\t %s\n", m.ts, m.data)
//     }
// }
