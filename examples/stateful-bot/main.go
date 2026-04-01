// stateful-bot demonstrates using Client[T] with custom user state.
//
// The bot keeps a per-user message counter in BotState and echoes it back.
// The EventHooks callbacks receive *Client[BotState] directly, so they can
// access both the client methods and the custom state without closures.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"

	qrcode "github.com/skip2/go-qrcode"

	clawbot "github.com/importcjj/wechat-clawbot-client-go"
	"github.com/importcjj/wechat-clawbot-client-go/store"
)

// BotState holds business state accessible via client.UserState().
type BotState struct {
	mu       sync.Mutex
	counters map[string]int // userID -> message count
}

func (s *BotState) Increment(userID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[userID]++
	return s.counters[userID]
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	state := &BotState{counters: make(map[string]int)}

	client := clawbot.New[*BotState]("stateful-bot", state, store.NewFileStore("./data"),
		clawbot.WithEventHooks(clawbot.EventHooks[*BotState]{
			OnMessage: func(c *clawbot.Client[*BotState], msg *clawbot.Message) {
				if msg.Text == "" {
					return
				}
				count := c.UserState().Increment(msg.From)
				reply := fmt.Sprintf("#%d: %s", count, msg.Text)
				if err := c.SendText(ctx, msg.From, reply); err != nil {
					fmt.Printf("send error: %v\n", err)
				}
			},
			OnConnected: func(c *clawbot.Client[*BotState]) {
				fmt.Printf("connected as %s\n", c.ClientID())
			},
			OnDisconnected: func(_ *clawbot.Client[*BotState], err error) {
				if err != nil && !errors.Is(err, context.Canceled) {
					fmt.Printf("disconnected: %v\n", err)
				}
			},
		}),
	)

	fmt.Println("restoring session...")
	err := client.Start(ctx)

	if errors.Is(err, clawbot.ErrNotLoggedIn) {
		fmt.Println("scan QR code to login")

		session, loginErr := client.Login(ctx)
		if loginErr != nil {
			log.Fatalf("login failed: %v", loginErr)
		}

		printQRCode(session.QRCodeURL())

		if waitErr := session.Wait(ctx); waitErr != nil {
			log.Fatalf("login wait failed: %v", waitErr)
		}

		err = client.Start(ctx)
	}

	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("run failed: %v", err)
	}
}

func printQRCode(url string) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		fmt.Printf("open this URL to scan:\n%s\n", url)
		return
	}
	fmt.Println(qr.ToSmallString(false))
}
