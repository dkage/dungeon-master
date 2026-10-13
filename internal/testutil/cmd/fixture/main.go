// A portable test process. Never invokes real Claude, gh or a network service.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Mode, Log, Output, Error string
	Exit                     int
	SleepMS                  int
}

func main() {
	exe, _ := os.Executable()
	var c Config
	data, _ := os.ReadFile(exe + ".json")
	_ = json.Unmarshal(data, &c)
	if c.SleepMS > 0 {
		time.Sleep(time.Duration(c.SleepMS) * time.Millisecond)
	}
	if c.Log != "" {
		f, err := os.OpenFile(c.Log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			panic(err)
		}
		if c.Mode == "e2e-gh" {
			fmt.Fprintf(f, "%d %s\n", time.Now().Unix(), strings.Join(os.Args[1:], " "))
		} else {
			fmt.Fprintf(f, "ARGS: %s\nPATH: %s\n", strings.Join(os.Args[1:], " "), os.Getenv("PATH"))
		}
		f.Close()
	}
	switch c.Mode {
	case "claude", "e2e-claude":
		fmt.Println("FAKE-CLAUDE")
		for _, a := range os.Args[1:] {
			fmt.Printf("arg=[%s]\n", a)
		}
		cwd, _ := os.Getwd()
		fmt.Println("cwd=" + filepath.Clean(cwd))
		if c.Mode == "claude" {
			for _, pair := range [][2]string{{"child-marker", "CLAUDE_CODE_CHILD_SESSION"}, {"hero-env", "HERO_VAR"}, {"path", "PATH"}, {"gate-url", "DUNGEON_GATE_URL"}, {"gate-token", "DUNGEON_GATE_TOKEN"}, {"real-gh", "DUNGEON_REAL_GH"}} {
				v, ok := os.LookupEnv(pair[1])
				if !ok {
					v = "unset"
				}
				fmt.Println(pair[0] + "=" + v)
			}
		}
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			fmt.Println("echo:" + scanner.Text())
			if scanner.Text() == "ask me" {
				fmt.Print("Overwrite the file? (y/n) ")
			}
			// The gray and dimmed text an agent prints for hints.
			if scanner.Text() == "palette" {
				fmt.Print("\x1b[2mdim-sample\x1b[0m \x1b[90mgray-sample\x1b[0m \x1b[2;90mdimgray-sample\x1b[0m\n")
			}
		}
	case "ask":
		fmt.Print(c.Output)
		bufio.NewScanner(os.Stdin).Scan()
	case "read":
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			fmt.Println("got:" + scanner.Text())
		}
	case "e2e-gh":
		fmt.Println("fake-gh-ran: " + strings.Join(os.Args[1:], " "))
	case "gh":
		if msg := os.Getenv("FAKE_GH_STDERR"); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
		if os.Getenv("FAKE_GH_ECHO_STDIN") == "1" {
			io.Copy(os.Stdout, os.Stdin)
		}
		fmt.Println("stdout-from-gh")
		if code, err := strconv.Atoi(os.Getenv("FAKE_GH_EXIT")); err == nil {
			os.Exit(code)
		}
	case "spawn":
		child := exec.Command(exe, "child")
		child.Env = append(os.Environ(), "DUNGEON_FIXTURE_CHILD=1")
		if os.Getenv("DUNGEON_FIXTURE_CHILD") != "1" {
			child.Start()
			fmt.Printf("child-pid=%d\n", child.Process.Pid)
		}
		time.Sleep(time.Minute)
	default:
		if c.Output != "" {
			fmt.Println(c.Output)
		}
		if c.Error != "" {
			fmt.Fprintln(os.Stderr, c.Error)
		}
	}
	os.Exit(c.Exit)
}
