package svc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func WritePid(pid int) error {
	return os.WriteFile(PidPath(), []byte(strconv.Itoa(pid)), 0644)
}

func RemovePid() { _ = os.Remove(PidPath()) }

func WriteState(job string) error {
	return os.WriteFile(StatePath(), []byte(strings.TrimSpace(job)), 0644)
}

func ReadPid() (int, error) {
	b, err := os.ReadFile(PidPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

func SendSignal(pid int, sig os.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(sig)
}

func ReadState() (string, error) {
	b, err := os.ReadFile(StatePath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

var (
	SIGUSR1 = syscall.SIGUSR1
	SIGUSR2 = syscall.SIGUSR2
)

func ListJobs() error {
	pid, err := ReadPid()
	if err != nil {
		return fmt.Errorf("read pid: %w", err)
	}
	if err := SendSignal(pid, SIGUSR1); err != nil {
		return fmt.Errorf("daemon not running (pid %d): %w", pid, err)
	}
	time.Sleep(200 * time.Millisecond)
	state, err := ReadState()
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if state == "" || state == "idle" {
		fmt.Println("idle")
	} else {
		fmt.Println(state)
	}
	return nil
}

func KillJobs() error {
	pid, err := ReadPid()
	if err != nil {
		return fmt.Errorf("read pid: %w", err)
	}
	if err := SendSignal(pid, SIGUSR2); err != nil {
		return fmt.Errorf("daemon not running (pid %d): %w", pid, err)
	}
	fmt.Println("sent kill signal to daemon")
	return nil
}
