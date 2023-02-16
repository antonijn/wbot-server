package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Guess struct {
	Word      string  `json:"word"`
	Score     float32 `json:"score"`
}

type WordReport struct {
	User        Guess    `json:"user"`
	Best        []Guess  `json:"best"`
	OptionsLeft []string `json:"optionsLeft"`
	Eliminated  int32    `json:"eliminated"`
	Colors      string   `json:"colors"`
}

type Engine interface {
	Solve(word string) ([]WordReport, error)
	Coach(word string, guesses []string) (*WordReport, error)
	WordList() ([]string, error)
}

type BotConfig struct {
	ExecPath           string `toml:"exec_path"`
	IndexPath          string `toml:"index_path"`
	MaxConcurrentUsers int    `toml:"max_concurrent_users"`
	SolveTimeout       int    `toml:"solve_timeout"`
	CoachTimeout       int    `toml:"coach_timeout"`
}

type Bot struct {
	config BotConfig
	work   chan func()
}

type TimeoutError string

func (err TimeoutError) Error() string {
	return string(err)
}

func (config BotConfig) validateExec() error {
	info, err := os.Stat(config.ExecPath)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("file at %v is not a regular file", config.ExecPath)
	}

	m := info.Mode()
	if (m & 0o755) != m {
		return fmt.Errorf("engine executable must have mode 0755 or stricter")
	}

	sysStat := info.Sys().(*syscall.Stat_t)
	if sysStat.Uid != 0 || sysStat.Gid != 0 {
		return fmt.Errorf("engine executable must be owned by root")
	}

	return nil
}

func NewBot(config BotConfig) (bot *Bot, err error) {
	err = config.validateExec()
	if err == nil {
		bot = &Bot{config: config, work: make(chan func())}
		for i := 0; i < config.MaxConcurrentUsers; i++ {
			go bot.worker()
		}
	}

	return
}

func (b *Bot) Close() {
	close(b.work)
}

func (bot *Bot) worker() {
	for {
		f, ok := <-bot.work
		if !ok {
			break
		}
		f()
	}
}

func (b *Bot) execAtom(timeout int, v any, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, b.config.ExecPath, args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("WORDSMITH_INDEX=%s", b.config.IndexPath))

	reader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	limiter := io.LimitReader(reader, 1024 * 1024)
	decoder := json.NewDecoder(limiter)

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := decoder.Decode(v); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return TimeoutError("timeout")
		}
		return err
	}

	return cmd.Wait()
}

func (b *Bot) exec(timeout int, v any, args ...string) (err error) {
	wg := sync.WaitGroup{}

	task := func() {
		err = b.execAtom(timeout, v, args...)
		wg.Done()
	}

	wg.Add(1)
	select {
	case b.work <- task:
		wg.Wait()
		return

	case <- time.After(time.Duration(timeout) * time.Millisecond):
		err = TimeoutError("timeout waiting for resources")
		return
	}
}

func (b *Bot) Solve(word string) ([]WordReport, error) {
	var result []WordReport
	err := b.exec(b.config.SolveTimeout, &result, "solve", "-t", word)
	return result, err
}

func (b *Bot) Coach(word string, guesses []string) (*WordReport, error) {
	var result WordReport

	args := []string{"coach", "-t", word}
	args = append(args, guesses...)

	err := b.exec(b.config.CoachTimeout, &result, args...)
	return &result, err
}

func (b *Bot) WordList() ([]string, error) {
	var words []string
	err := b.exec(1000, &words, "list", "all")
	return words, err
}
