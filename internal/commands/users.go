package commands

import (
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/mjpitz/myago/flagset"
	"github.com/mjpitz/varys/internal/client"
	"github.com/mjpitz/varys/internal/engine"
)

type UpdateUserRequest struct {
	RotateServiceKind string `json:"rotate_service_kind" usage:"the kind of service that we're rotating the credential for"`
	RotateServiceName string `json:"rotate_service_name" usage:"the name of the service we're rotating the credential for"`
}

var (
	updateUserRequest = UpdateUserRequest{}

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
				Name:  "current",
				Usage: "Interact with the currently authenticated user.",
				Subcommands: []*cli.Command{
					{
						Name:      "get",
						Usage:     "Output information about the current user.",
						ArgsUsage: " ",
						Action: func(ctx *cli.Context) error {
							api := client.Extract(ctx.Context)

							user, err := api.Users().Current().Get(ctx.Context)
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
					{
						Name:      "update",
						Usage:     "Update information about the current user.",
						ArgsUsage: " ",
						Flags:     flagset.ExtractPrefix("varys_update_user", &updateUserRequest),
						Action: func(ctx *cli.Context) error {
							api := client.Extract(ctx.Context)

							return api.Users().Current().Update(ctx.Context, engine.UpdateUserRequest{
								RotateService: engine.Service{
									Kind: updateUserRequest.RotateServiceKind,
									Name: updateUserRequest.RotateServiceName,
								},
							})
						},
					},
				},
			},
		},
		HideHelpCommand: true,
	}
)
