package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/auth"
	"github.com/cpbridge/api/internal/db"
)

// bootstrap-admin is intentionally an out-of-band operator command. It does
// not share the public registration path and only succeeds while no active
// administrator exists.
func main() {
	email := flag.String("email", "", "email address of the existing account to promote")
	flag.Parse()
	if strings.TrimSpace(*email) == "" {
		log.Fatal("-email is required")
	}

	database, err := db.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if err := db.EnsureSchema(database); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := database.ExecContext(ctx, `
		UPDATE users
		SET role = $1, is_active = true, updated_at = NOW()
		WHERE LOWER(email) = LOWER($2)
		  AND NOT EXISTS (SELECT 1 FROM users WHERE role = $1 AND is_active = true)
	`, auth.RoleAdmin, strings.TrimSpace(*email))
	if err != nil {
		log.Fatal(err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	if rows == 1 {
		fmt.Printf("promoted %s to %s\n", strings.TrimSpace(*email), auth.RoleAdmin)
		return
	}

	var exists bool
	if err := database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = LOWER($1))`, strings.TrimSpace(*email)).Scan(&exists); err != nil {
		log.Fatal(err)
	}
	if !exists {
		log.Fatal("account does not exist; register it first, then rerun this command")
	}
	var activeAdmin bool
	if err := database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE role = $1 AND is_active = true)`, auth.RoleAdmin).Scan(&activeAdmin); err != nil {
		log.Fatal(err)
	}
	if activeAdmin {
		log.Fatal("an active administrator already exists; use the admin user-management flow")
	}
	log.Fatal(errors.New("account could not be promoted"))
}
