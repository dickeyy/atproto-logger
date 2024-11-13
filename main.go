package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	wsURL = "ws://localhost:6008/subscribe"
)

type Record struct {
	Type      string      `json:"$type"`
	Text      string      `json:"text,omitempty"`
	Subject   *Subject    `json:"subject,omitempty"`
	CreatedAt string      `json:"createdAt,omitempty"`
	Embed     interface{} `json:"embed,omitempty"`
}

type Subject struct {
	URI string `json:"uri"`
	Cid string `json:"cid"`
}

// JetstreamMessage represents the top-level message structure
type JetstreamMessage struct {
	Did      string         `json:"did"`
	TimeUs   int64          `json:"time_us"`
	Kind     string         `json:"kind"`
	Commit   *CommitEvent   `json:"commit,omitempty"`
	Identity *IdentityEvent `json:"identity,omitempty"`
	Account  *AccountEvent  `json:"account,omitempty"`
}

// CommitEvent represents a repository commit
type CommitEvent struct {
	Rev        string          `json:"rev"`
	Operation  string          `json:"operation"`
	Collection string          `json:"collection"`
	Rkey       string          `json:"rkey"`
	Record     json.RawMessage `json:"record,omitempty"`
	Cid        string          `json:"cid,omitempty"`
}

// IdentityEvent represents an identity update
type IdentityEvent struct {
	Did    string `json:"did"`
	Handle string `json:"handle"`
	Seq    int64  `json:"seq"`
	Time   string `json:"time"`
}

// AccountEvent represents an account status change
type AccountEvent struct {
	Active bool   `json:"active"`
	Did    string `json:"did"`
	Seq    int64  `json:"seq"`
	Time   string `json:"time"`
}

func connectWebSocket() (*websocket.Conn, error) {
	dialer := websocket.DefaultDialer
	c, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial error: %v", err)
	}
	return c, nil
}

func parseMessage(messageType int, message []byte) (*JetstreamMessage, error) {
	var msg JetstreamMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %v", err)
	}
	return &msg, nil
}

func handleMessage(messageType int, msg *JetstreamMessage) {
	switch msg.Kind {
	case "commit":
		if msg.Commit == nil {
			return
		}

		logger := log.With().
			Str("did", msg.Did).
			Str("op", msg.Commit.Operation).
			Logger()

		switch msg.Commit.Collection {
		case "app.bsky.feed.post":
			var record Record
			if err := json.Unmarshal(msg.Commit.Record, &record); err != nil {
				return
			}
			logger.Info().
				Str("type", "post").
				Str("text", record.Text).
				Str("rkey", msg.Commit.Rkey).
				Interface("embed", record.Embed).
				Msg("post")

		case "app.bsky.feed.like":
			var record Record
			if err := json.Unmarshal(msg.Commit.Record, &record); err != nil {
				return
			}
			logger.Info().
				Str("type", "like").
				Str("post_uri", record.Subject.URI).
				Str("post_cid", record.Subject.Cid).
				Msg("like")

		case "app.bsky.feed.repost":
			var record Record
			if err := json.Unmarshal(msg.Commit.Record, &record); err != nil {
				return
			}
			logger.Info().
				Str("type", "repost").
				Str("post_uri", record.Subject.URI).
				Str("post_cid", record.Subject.Cid).
				Msg("repost")

		case "app.bsky.graph.follow":
			var record Record
			if err := json.Unmarshal(msg.Commit.Record, &record); err != nil {
				return
			}
			logger.Info().
				Str("type", "follow").
				Str("subject", record.Subject.URI).
				Msg("follow")

		case "app.bsky.feed.threadgate":
			logger.Info().
				Str("type", "threadgate").
				Str("rkey", msg.Commit.Rkey).
				Msg("threadgate")

		case "app.bsky.actor.profile":
			logger.Info().
				Str("type", "profile").
				RawJSON("data", msg.Commit.Record).
				Msg("profile")

		case "app.bsky.graph.block":
			var record Record
			if err := json.Unmarshal(msg.Commit.Record, &record); err != nil {
				return
			}
			logger.Info().
				Str("type", "block").
				Str("subject", record.Subject.URI).
				Msg("block")

		case "app.bsky.feed.generator":
			logger.Info().
				Str("type", "feed_generator").
				Str("rkey", msg.Commit.Rkey).
				RawJSON("data", msg.Commit.Record).
				Msg("feed_generator")

		default:
			logger.Info().
				Str("type", "other").
				Str("collection", msg.Commit.Collection).
				Str("rkey", msg.Commit.Rkey).
				RawJSON("data", msg.Commit.Record).
				Msg("other")
		}

	case "identity":
		if msg.Identity != nil {
			log.Info().
				Str("did", msg.Did).
				Str("handle", msg.Identity.Handle).
				Int64("seq", msg.Identity.Seq).
				Msg("handle_update")
		}

	case "account":
		if msg.Account != nil {
			log.Info().
				Str("did", msg.Did).
				Bool("active", msg.Account.Active).
				Int64("seq", msg.Account.Seq).
				Msg("account_update")
		}
	}
}

func monitorEvents() {
	for {
		log.Info().Msg("connecting to jetstream")

		conn, err := connectWebSocket()
		if err != nil {
			log.Error().Err(err).Msg("connection error, retrying in 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		}

		log.Info().Msg("connected")

		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)
		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					log.Error().Err(err).Msg("read error")
					return
				}

				msg, err := parseMessage(messageType, message)
				if err != nil {
					log.Error().Err(err).Msg("parse error")
					continue
				}

				handleMessage(messageType, msg)
			}
		}()

		select {
		case <-done:
			log.Info().Msg("connection closed, reconnecting in 5 seconds")
			time.Sleep(5 * time.Second)
		case <-interrupt:
			log.Info().Msg("shutting down")
			err := conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Error().Err(err).Msg("error closing connection")
			}
			conn.Close()
			return
		}
	}
}

func main() {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})

	monitorEvents()
}
