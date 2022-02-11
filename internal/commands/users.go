package commands

import (
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/mjpitz/myago/flagset"
	"github.com/mjpitz/varys/internal/client"
)

var (
	Users = &cli.Command{
		Name:  "users",
		Usage: "Perform operations against the Users API.",
		Flags: flagset.ExtractPrefix("varys", &client.DefaultConfig),
		Before: func(ctx *cli.Context) error {
			api, err := client.NewAPI(client.DefaultConfig)
			if err != nil {
				return err
			}

			ctx.Context = client.WithContext(ctx.Context, api)
			return nil
		},
		Subcommands: []*cli.Command{
			{
				Name:      "list",
				Usage:     "List all users known to varys.",
				ArgsUsage: " ",
				Action: func(ctx *cli.Context) error {
					api := client.Extract(ctx.Context)

					users, err := api.Users().List(ctx.Context)
					if err != nil {
						return err
					}

					table := newTable(ctx.App.Writer)
					table.SetHeader([]string{"Kind", "ID", "Name"})

					for _, user := range users {
						table.Append([]string{user.Kind, user.ID, user.Name})
					}

					table.Render()
					return nil
				},
			},
			{
				Name:      "current",
				Usage:     "Output information about the current user.",
				ArgsUsage: " ",
				Action: func(ctx *cli.Context) error {
					api := client.Extract(ctx.Context)

					user, err := api.Users().Current(ctx.Context)
					if err != nil {
						return err
					}

					table := newTable(ctx.App.Writer)

					table.Append([]string{"ID", user.Subject})
					table.Append([]string{"PROFILE", user.Profile})
					table.Append([]string{"EMAIL", user.Email})
					table.Append([]string{"EMAIL VERIFIED", strconv.FormatBool(user.EmailVerified)})
					table.Append([]string{"GROUPS", strings.Join(user.Groups, ", ")})

					table.Render()
					return nil
				},
			},
		},
		HideHelpCommand: true,
	}
)
