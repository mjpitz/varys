package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/mjpitz/myago/flagset"
	"github.com/mjpitz/varys/internal/client"
	"github.com/mjpitz/varys/internal/engine"
)

type user struct {
	Kind string `json:"kind" usage:"specify the kind of user we're referring to" required:"true"`
	ID   string `json:"id" usage:"specify the id of the user we're granting access" required:"true"`
}

type grantRequest struct {
	User       user             `json:"user"`
	Permission *cli.StringSlice `json:"permission" alias:"p" usage:"the permissions [options: read,write,update,delete,admin,system]"`
}

var (
	createRequest = engine.CreateServiceRequest{
		Templates: engine.Templates{
			UserTemplate:     "basic",
			PasswordTemplate: "max",
		},
	}
	updateRequest = engine.UpdateServiceRequest{}

	updateGrantRequest = grantRequest{}

	deleteGrantRequest = grantRequest{}

	Services = &cli.Command{
		Name:  "services",
		Usage: "Perform operations against the Services API.",
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
				Name:      "connect",
				Usage:     "Connects to a service managed by varys.",
				ArgsUsage: "<kind> <name> [program...]",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()

					switch {
					case len(args) < 2:
						return fmt.Errorf("expecting two arguments: <kind> <name>")
					case len(args) < 3:
						// TODO: make this optional with a driver approach
						return fmt.Errorf("missing program to execute")
					}

					kind := args[0]
					name := args[1]
					program := args[2:]

					api := client.Extract(ctx.Context)

					serviceCreds, err := api.Services().Credentials(ctx.Context, kind, name)
					if err != nil {
						return err
					}

					// TODO: eventually use a driver approach

					pargs := make([]string, 0)
					if len(program) > 1 {
						pargs = program[1:]
					}

					prefix := strings.ToUpper(kind) + "_" + strings.ToUpper(name)

					cmd := exec.Command(program[0], pargs...)
					cmd.Stdin = os.Stdin
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr

					cmd.Env = append(cmd.Env,
						prefix+"_ADDRESS="+serviceCreds.Address,
						prefix+"_USERNAME="+serviceCreds.Credentials.Username,
						prefix+"_PASSWORD="+serviceCreds.Credentials.Password,
					)

					err = cmd.Run()
					if err != nil {
						return err
					}

					return nil
				},
			},
			{
				Name:      "create",
				Usage:     "Create a new service in varys.",
				ArgsUsage: "<kind> <name>",
				Flags:     flagset.ExtractPrefix("varys_create_service", &createRequest),
				Action: func(ctx *cli.Context) error {
					args := ctx.Args()

					createRequest.Kind = args.Get(0)
					createRequest.Name = args.Get(1)

					if createRequest.Kind == "" || createRequest.Name == "" {
						return fmt.Errorf("expecting two arguments: <kind> <name>")
					}

					api := client.Extract(ctx.Context)

					return api.Services().Create(ctx.Context, createRequest)
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete a service in varys.",
				ArgsUsage: "<kind> <name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args()

					kind := args.Get(0)
					name := args.Get(1)

					if kind == "" || name == "" {
						return fmt.Errorf("expecting two arguments: <kind> <name>")
					}

					api := client.Extract(ctx.Context)

					return api.Services().Delete(ctx.Context, kind, name)
				},
			},
			{
				Name:      "get",
				Usage:     "Get a service from varys.",
				ArgsUsage: "<kind> <name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args()

					kind := args.Get(0)
					name := args.Get(1)

					if kind == "" || name == "" {
						return fmt.Errorf("expecting two arguments: <kind> <name>")
					}

					api := client.Extract(ctx.Context)

					service, err := api.Services().Get(ctx.Context, kind, name)
					if err != nil {
						return err
					}

					table := newTable(ctx.App.Writer)

					table.Append([]string{"KIND", service.Kind})
					table.Append([]string{"NAME", service.Name})
					table.Append([]string{"ADDRESS", service.Address})
					table.Append([]string{"USER TEMPLATE", string(service.Templates.UserTemplate)})
					table.Append([]string{"PASSWORD TEMPLATE", string(service.Templates.PasswordTemplate)})

					table.Render()
					return nil
				},
			},
			{
				Name:  "grants",
				Usage: "Manage who has access to a given service.",
				Subcommands: []*cli.Command{
					{
						Name:      "list",
						Usage:     "List all users who have access to a service and their permissions.",
						ArgsUsage: " ",
						Action: func(ctx *cli.Context) error {
							args := ctx.Args()

							kind := args.Get(0)
							name := args.Get(1)

							if kind == "" || name == "" {
								return fmt.Errorf("expecting two arguments: <kind> <name>")
							}

							api := client.Extract(ctx.Context)

							grants, err := api.Services().Grants().List(ctx.Context, kind, name)
							if err != nil {
								return err
							}

							table := newTable(ctx.App.Writer)
							table.SetHeader([]string{"UserKind", "UserID", "UserName", "Roles"})

							for _, grant := range grants {
								table.Append([]string{
									grant.User.Kind, grant.User.Name, grant.User.ID,
									strings.Join(grant.Roles, ", "),
								})
							}

							table.Render()
							return nil
						},
					},
					{
						Name:      "update",
						Usage:     "Update a user's access to a service in varys.",
						ArgsUsage: "<kind> <name>",
						Flags:     flagset.ExtractPrefix("varys_update_service_grant", &updateGrantRequest),
						Action: func(ctx *cli.Context) error {
							args := ctx.Args()

							kind := args.Get(0)
							name := args.Get(1)

							if kind == "" || name == "" {
								return fmt.Errorf("expecting two arguments: <kind> <name>")
							}

							permissions := updateGrantRequest.Permission.Value()
							if len(permissions) == 0 {
								return fmt.Errorf("must provide at least one permission")
							}

							suffix := fmt.Sprintf("%s:%s", kind, name)

							roles := make([]string, 0)
							for _, permission := range permissions {
								roles = append(roles, permission+":"+suffix)
							}

							api := client.Extract(ctx.Context)

							return api.Services().Grants().Update(ctx.Context, kind, name, engine.UserGrant{
								User: engine.User{
									Kind: updateGrantRequest.User.Kind,
									ID:   updateGrantRequest.User.ID,
								},
								Roles: roles,
							})
						},
					},
					{
						Name:      "delete",
						Usage:     "Remove a user's access to a service in varys.",
						ArgsUsage: "<kind> <name>",
						Flags:     flagset.ExtractPrefix("varys_delete_service_grant", &deleteGrantRequest),
						Action: func(ctx *cli.Context) error {
							args := ctx.Args()

							kind := args.Get(0)
							name := args.Get(1)

							if kind == "" || name == "" {
								return fmt.Errorf("expecting two arguments: <kind> <name>")
							}

							permissions := updateGrantRequest.Permission.Value()
							if len(permissions) == 0 {
								return fmt.Errorf("must provide at least one permission")
							}

							suffix := fmt.Sprintf("%s:%s", kind, name)

							roles := make([]string, 0)
							for _, permission := range permissions {
								roles = append(roles, permission+":"+suffix)
							}

							api := client.Extract(ctx.Context)

							return api.Services().Grants().Delete(ctx.Context, kind, name, engine.UserGrant{
								User: engine.User{
									Kind: updateGrantRequest.User.Kind,
									ID:   updateGrantRequest.User.ID,
								},
								Roles: roles,
							})
						},
					},
				},
			},
			{
				Name:      "list",
				Usage:     "List all services managed by varys.",
				ArgsUsage: " ",
				Action: func(ctx *cli.Context) error {
					api := client.Extract(ctx.Context)

					services, err := api.Services().List(ctx.Context)
					if err != nil {
						return err
					}

					table := newTable(ctx.App.Writer)
					table.SetHeader([]string{"Kind", "Name", "Address"})

					for _, service := range services {
						table.Append([]string{service.Kind, service.Name, service.Address})
					}

					table.Render()
					return nil
				},
			},
			{
				Name:      "update",
				Usage:     "Update a service in varys.",
				ArgsUsage: "<kind> <name>",
				Flags:     flagset.ExtractPrefix("varys_update_service", &updateRequest),
				Action: func(ctx *cli.Context) error {
					args := ctx.Args()

					kind := args.Get(0)
					name := args.Get(1)

					if kind == "" || name == "" {
						return fmt.Errorf("expecting two arguments: <kind> <name>")
					}

					api := client.Extract(ctx.Context)

					return api.Services().Update(ctx.Context, kind, name, updateRequest)
				},
			},
		},
		HideHelpCommand: true,
	}
)
