// Command vapid generates a VAPID key pair for Web Push.
//
//	go run ./cmd/vapid
//
// It prints the two variables to set (keep the private key secret, e.g. as a
// Railway service variable). Rotating the pair invalidates every existing
// browser subscription: browsers must subscribe again.
package main

import (
	"fmt"
	"os"

	"github.com/Jihed-khelifi/staryield/backend/internal/push"
)

func main() {
	pub, priv, err := push.GenerateKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vapid: generate keys:", err)
		os.Exit(1)
	}
	if err := (push.Config{PublicKey: pub, PrivateKey: priv, Subject: "mailto:support@staryield.net"}).Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "vapid: generated keys are invalid:", err)
		os.Exit(1)
	}
	fmt.Printf("STARYIELD_PUSH_VAPID_PUBLIC_KEY=%s\n", pub)
	fmt.Printf("STARYIELD_PUSH_VAPID_PRIVATE_KEY=%s\n", priv)
}
