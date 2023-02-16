package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/pelletier/go-toml/v2"
)

var engine Engine
var globalConfigPath = "/etc/wbot/server.conf"

type ServerConfig struct {
	Port int `toml:"port"`
}

type ConfigFile struct {
	Server ServerConfig `toml:"server"`
	Engine BotConfig    `toml:"engine"`
}

var words []string

func enforceMethod(w http.ResponseWriter, r *http.Request, allowed ...string) error {
	for _, allow := range allowed {
		if allow == r.Method {
			return nil
		}
	}

	w.Header().Add("Allow", strings.Join(allowed, ", "))
	status := http.StatusMethodNotAllowed
	msg := http.StatusText(status)
	http.Error(w, msg, status)
	return errors.New(msg)
}

func wordValid(word string) bool {
	if len(word) != 5 {
		return false
	}

	for _, c := range word {
		if !unicode.IsLetter(c) || c > unicode.MaxASCII {
			return false
		}
	}

	return true
}

func internalError(w http.ResponseWriter, err error, id uuid.UUID) {
	log.Printf("(uuid=%v) error: %v\n", id, err)
	status := http.StatusInternalServerError
	if _, ok := err.(TimeoutError); ok {
		status = http.StatusServiceUnavailable
	}
	msg := fmt.Sprintf(
		"%d - %s\nThe developers will know what to do with this: %v",
		status,
		http.StatusText(status),
		id,
	)
	http.Error(w, msg, status)
}

func getIP(r *http.Request) string {
	proxyFor := r.Header.Get("X-Real-IP")
	if len(proxyFor) > 0 {
		return proxyFor
	}

	return r.RemoteAddr
}

func writeJSON(w http.ResponseWriter, data any, id uuid.UUID) {
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		internalError(w, err, id)
	}
}

func solveWord(w http.ResponseWriter, r *http.Request) {
	if enforceMethod(w, r, "GET") != nil {
		return
	}

	id := uuid.New()
	ip := getIP(r)

	r.ParseForm()
	word := r.Form.Get("w")

	if !wordValid(word) {
		http.Error(w, "Invalid word", http.StatusBadRequest)
		log.Printf("Invalid `w' parameter in /solve request from %v\n", ip)
		return
	}

	log.Printf("(uuid=%v) /solve from %v, w=%s\n", id, ip, word)
	start := time.Now()

	data, err := engine.Solve(word)
	if err != nil {
		internalError(w, err, id)
	} else {
		writeJSON(w, data, id)
	}

	log.Printf("(uuid=%v) /solve done, took %v", id, time.Since(start))
}

func coachWord(w http.ResponseWriter, r *http.Request) {
	if enforceMethod(w, r, "GET") != nil {
		return
	}

	id := uuid.New()
	ip := getIP(r)

	r.ParseForm()
	word := r.Form.Get("w")

	if !wordValid(word) {
		http.Error(w, "Invalid target word", http.StatusBadRequest)
		log.Printf("Invalid `w' parameter in /coach request from %v\n", ip)
		return
	}

	guessesStr := r.Form.Get("guess")
	guesses := strings.Split(guessesStr, ",")
	if len(guesses) == 0 {
		http.Error(w, "Expected guess", http.StatusBadRequest)
		log.Printf("Empty `guess' parameter in /coach request from %v\n", ip)
		return
	}

	for _, g := range guesses {
		if !wordValid(g) {
			http.Error(w, "Invalid word", http.StatusBadRequest)
			log.Printf("Invalid `guess' parameter in /coach request from %v\n", ip)
			return
		}
	}

	log.Printf("(uuid=%v) /coach from %v, w=%s, guess=%s\n", id, ip, word, guessesStr)
	start := time.Now()

	data, err := engine.Coach(word, guesses)
	if err != nil {
		internalError(w, err, id)
	} else {
		writeJSON(w, data, id)
	}

	log.Printf("(uuid=%v) /coach done, took %v\n", id, time.Since(start))
}

func loadConfig() (config *ConfigFile, err error) {
	log.Printf("Reading server config at %s", globalConfigPath)

	tomlFile, err := os.Open(globalConfigPath)
	if err != nil {
		return
	}
	defer tomlFile.Close()

	config = &ConfigFile{Server: ServerConfig{Port: 8080}}

	decode := toml.NewDecoder(tomlFile)
	if err = decode.Decode(config); err != nil {
		config = nil
		return
	}

	log.Print("Server config loaded")
	return
}

func main() {
	log.SetFlags(0)

	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	bot, err := NewBot(config.Engine)
	if err != nil {
		log.Fatal(err)
	}
	defer bot.Close()
	engine = bot

	log.Println("Loading words")
	words, err = engine.WordList()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Read %d words\n", len(words))

	http.HandleFunc("/solve", solveWord)
	http.HandleFunc("/coach", coachWord)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Server.Port), nil))
}
