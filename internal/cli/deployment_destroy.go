package cli

import (
	"context"
	"strings"

	"github.com/posener/complete"

	clientpkg "github.com/hashicorp/waypoint/internal/client"
	"github.com/hashicorp/waypoint/internal/clierrors"
	"github.com/hashicorp/waypoint/internal/pkg/flag"
	pb "github.com/hashicorp/waypoint/internal/server/gen"
	"github.com/hashicorp/waypoint/sdk/terminal"
)

type DeploymentDestroyCommand struct {
	*baseCommand

	flagForce bool
}

func (c *DeploymentDestroyCommand) Run(args []string) int {
	ctx := c.Ctx
	flags := c.Flags()

	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithArgs(args),
		WithFlags(flags),
		WithSingleApp(),
	); err != nil {
		return 1
	}
	args = flags.Args()

	// Determine the deployments to delete
	var deployments []*pb.Deployment

	var err error
	if len(args) > 0 {
		// If we have arguments, we only delete the deployments specified.
		deployments, err = c.getDeployments(ctx, args)
		if err != nil {
			c.ui.Output(clierrors.Humanize(err), terminal.WithErrorStyle())
			return 1
		}
	} else {
		// No arguments, get ALL deployments that are still physically created.
		deployments, err = c.allDeployments(ctx)
		if err != nil {
			c.ui.Output(clierrors.Humanize(err), terminal.WithErrorStyle())
			return 1
		}
	}

	// Destroy each deployment
	c.ui.Output("%d deployments will be destroyed.", len(deployments), terminal.WithHeaderStyle())
	for _, deployment := range deployments {
		// Can't destroy a deployment that was not successful
		if deployment.Status.GetState() != pb.Status_SUCCESS {
			continue
		}

		// Get our app client
		app := c.project.App(deployment.Application.Application)

		c.ui.Output("Destroying deployment: %s", deployment.Id, terminal.WithInfoStyle())
		if err := app.DestroyDeploy(ctx, &pb.Job_DestroyDeployOp{
			Deployment: deployment,
		}); err != nil {
			c.ui.Output("Error destroying the deployment: %s", err.Error(), terminal.WithErrorStyle())
			return 1
		}
	}

	return 0
}

func (c *DeploymentDestroyCommand) getDeployments(ctx context.Context, ids []string) ([]*pb.Deployment, error) {
	var result []*pb.Deployment

	// Get each deployment
	client := c.project.Client()
	for _, id := range ids {
		deployment, err := client.GetDeployment(ctx, &pb.GetDeploymentRequest{
			DeploymentId: id,
		})
		if err != nil {
			return nil, err
		}

		result = append(result, deployment)
	}

	return result, nil
}

func (c *DeploymentDestroyCommand) allDeployments(ctx context.Context) ([]*pb.Deployment, error) {
	var result []*pb.Deployment

	client := c.project.Client()
	err := c.DoApp(c.Ctx, func(ctx context.Context, app *clientpkg.App) error {
		resp, err := client.ListDeployments(ctx, &pb.ListDeploymentsRequest{
			Application:   app.Ref(),
			Workspace:     c.project.WorkspaceRef(),
			PhysicalState: pb.Operation_CREATED,
			Order: &pb.OperationOrder{
				Order: pb.OperationOrder_COMPLETE_TIME,
				Desc:  true,
			},
		})
		if err != nil {
			return err
		}

		result = append(result, resp.Deployments...)
		return nil
	})

	return result, err
}

func (c *DeploymentDestroyCommand) Flags() *flag.Sets {
	return c.flagSet(flagSetOperation, func(set *flag.Sets) {
		f := set.NewSet("Command Options")
		f.BoolVar(&flag.BoolVar{
			Name:    "force",
			Target:  &c.flagForce,
			Usage:   "Yes to all confirmations.",
			Default: false,
		})
	})
}

func (c *DeploymentDestroyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *DeploymentDestroyCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *DeploymentDestroyCommand) Synopsis() string {
	return "Destroy one or more deployments."
}

func (c *DeploymentDestroyCommand) Help() string {
	helpText := `
Usage: waypoint deployment destroy [options] [id...]

  Destroy one or more deployments. This will "undeploy" this specific
  instance of an application.

  When no arguments are given, this will default to destroying ALL
  deployments. This will require interactive confirmation by the user
  unless the force flag (-force) is specified.

` + c.Flags().Help()

	return strings.TrimSpace(helpText)
}
