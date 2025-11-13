// Package cmd wires the Cobra CLI commands.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/dkooll/issuor/internal/report"
	"github.com/dkooll/issuor/internal/scanner"
)

type rootOptions struct {
	org    string
	prefix string
	debug  bool
	skip   string
	issues bool
	prs    bool
	aud    string
}

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:   "issuor",
		Short: "Report open external issues and pull requests for a GitHub organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, opts)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.org, "org", "", "GitHub organization to scan (required)")
	flags.StringVar(&opts.prefix, "prefix", "", "Repository name prefix to match (required)")
	flags.StringVar(&opts.skip, "skip", "github-actions[bot],dependabot[bot],release-please[bot]", "Comma-separated usernames to skip")
	flags.BoolVar(&opts.debug, "debug", false, "Enable verbose diagnostics")
	flags.BoolVar(&opts.issues, "issues", true, "Include issues in the report")
	flags.BoolVar(&opts.prs, "prs", true, "Include pull requests in the report")
	flags.StringVar(&opts.aud, "audience", "all", "Authors to include: all|internal|external")

	_ = cmd.MarkFlagRequired("org")
	_ = cmd.MarkFlagRequired("prefix")

	return cmd
}

func run(cmd *cobra.Command, opts *rootOptions) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		return errors.New("GITHUB_TOKEN environment variable is required")
	}
	if !opts.issues && !opts.prs {
		return errors.New("at least one of --issues or --prs must be enabled")
	}

	graphClient, err := buildGitHubClient(ctx, token)
	if err != nil {
		return err
	}

	audience, err := parseAudience(opts.aud)
	if err != nil {
		return err
	}

	scn, err := scanner.New(
		scanner.Config{
			Organization:  opts.org,
			RepoPrefix:    opts.prefix,
			SkipUsers:     parseSkipList(opts.skip),
			IncludeIssues: opts.issues,
			IncludePRs:    opts.prs,
			Audience:      audience,
		},
		graphClient,
		scanner.WithLogger(debugLogger(cmd, opts.debug)),
	)
	if err != nil {
		return err
	}

	res, err := scn.Scan(ctx)
	if err != nil {
		return err
	}

	report.New(cmd.OutOrStdout()).Print(res)

	return nil
}

func parseSkipList(csv string) []string {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(strings.ToLower(part)); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func buildGitHubClient(ctx context.Context, token string) (*githubv4.Client, error) {
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, src)
	return githubv4.NewClient(httpClient), nil
}

func parseAudience(raw string) (scanner.Audience, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(scanner.AudienceAll), "":
		return scanner.AudienceAll, nil
	case string(scanner.AudienceInternal):
		return scanner.AudienceInternal, nil
	case string(scanner.AudienceExternal):
		return scanner.AudienceExternal, nil
	default:
		return "", fmt.Errorf("invalid --audience value %q (use all|internal|external)", raw)
	}
}

func debugLogger(cmd *cobra.Command, enabled bool) scanner.Logger {
	if !enabled {
		return nil
	}
	return func(format string, args ...any) {
		fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
	}
}
