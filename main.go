package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/ms-choudhary/gmail2gullak/internal/email"
	"github.com/ms-choudhary/gmail2gullak/internal/gullak"
)

func main() {
	credentialsFile := flag.String("credentials", "credentials.json", "Google Oauth Credentails File")
	listenAddr := flag.String("listen", ":8999", "Listen addr for oauth redirect")
	gullakAddr := flag.String("gullakaddr", "http://localhost:3333", "Gullak server address")
	pollInterval := flag.Duration("every", 30*time.Second, "Poll interval")
	flag.Parse()

	server, err := email.NewServer(*credentialsFile)
	if err != nil {
		log.Fatal(err)
	}
	gullakClient := gullak.NewClient(*gullakAddr)

	go func() {
		ctx := context.Background()
		for {
			client, err := server.NewGmailClient(ctx)

			if err != nil {
				log.Printf("could not get email client: %v", err)
			} else {
				if err := client.ProcessMessages(ctx, gullakClient); err != nil {
					log.Printf("failed to read messages, will be retried: %v", err)
				}
			}

			time.Sleep(*pollInterval)
		}
	}()

	http.HandleFunc("/login", server.HandleLogin)
	http.HandleFunc("/status", server.HandleStatus)
	http.HandleFunc("/oauth2callback", server.HandleOauthCallback)

	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
