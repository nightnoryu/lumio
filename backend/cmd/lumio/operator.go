package main

import (
	"context"
	"errors"
	"fmt"

	"lumio/internal/app"
)

func operatorCommand(ctx context.Context, service *app.Service, cfg *config, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: lumio invite|reset-link EMAIL")
	}
	var token string
	var err error
	switch args[0] {
	case "invite":
		token, err = service.Invite(ctx, args[1])
	case "reset-link":
		token, err = service.IssueReset(ctx, args[1])
	default:
		return errors.New("usage: lumio invite|reset-link EMAIL")
	}
	if err != nil {
		return err
	}
	if args[0] == "reset-link" {
		fmt.Printf("%s/#reset=%s\n", cfg.DashboardOrigin, token)
	} else {
		fmt.Println(token)
	}
	return nil
}
