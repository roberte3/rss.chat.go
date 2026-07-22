package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/coder/websocket"
	"rss.chat.go/db"
	"rss.chat.go/setup"
)

type Settings struct {
	Note        string `json:"note"`
	ProductName string `json:"productName"`
}

func readSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func main() {
	setupFlag := flag.Bool("setup", false, "initialize the database and exit")
	flag.Parse()

	conn, err := db.Open("rss.chat.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer conn.Close()

	if *setupFlag {
		if err := setup.CreateDatabase(conn); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
		if err := setup.CreateSettings("settings.json"); err != nil {
			log.Fatalf("setup failed: %v", err)
		}

		log.Println("setup complete")
		return
	}

	log.Println("connected to database")
	runHttpSvr()
}

func runHttpSvr() {
	mux := http.NewServeMux()

	//Health Handler
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	//Websocket
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			panic(err)
		}
		defer c.CloseNow()
	})

	fmt.Println("Server is running on :8081...")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		panic(err)
	}
}
