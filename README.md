# issuor [![Go Reference](https://pkg.go.dev/badge/github.com/dkooll/issuor.svg)](https://pkg.go.dev/github.com/dkooll/issuor)

Issuor scans github organizations for open issues and pull requests, helping teams track external contributions and internal activity across multiple repositories.

## Why issuor?

Managing contributions across many repositories is challenging:

External contributions can get lost across repositories.

Identifying internal vs external activity requires manual checking.

Issuor helps you:

Track all open issues and PRs across an organization.

Distinguish between internal team members and external contributors.

Filter repositories by prefix to focus on relevant projects.

Skip bot accounts (GitHub Actions, Dependabot, etc.).

Get organized reports grouped by repository.

## Installation

`go install github.com/dkooll/issuor@latest`

Scan an organization:

## Scan all repositories with prefix "terraform-"

`issuor --org cloudnationhq --prefix terraform-`

## Only show external contributions

`issuor --org cloudnationhq --prefix terraform- --audience external`

## Only show issues (skip PRs)

`issuor --org cloudnationhq --prefix terraform- --prs=false`

## Enable debug output

`issuor --org cloudnationhq --prefix terraform- --debug`

## Features

`Organization Scanning`

Searches all repositories matching a prefix pattern.

Fetches open issues and pull requests via GitHub GraphQL API.

Groups results by repository for easy navigation.

Shows creation dates and author information.

`Author Filtering`

Distinguishes internal (members/collaborators) from external contributors.

Filter by audience: all, internal, or external.

Skip specific users (bots, automated accounts).

`Clean Output`

Formatted tables with repository grouping.

Bold headers for easy scanning.

Age formatting (days, weeks, months, years).

Summary statistics at the end.

## Configuration

`Command-Line Flags`

`--org`: GitHub organization to scan (required).

`--prefix`: Repository name prefix to match (required).

`--audience`: Authors to include: all|internal|external (default: all).

`--skip`: Comma-separated usernames to skip (default: github-actions[bot],dependabot[bot],release-please[bot]).

`--issues`: Include issues in the report (default: true).

`--prs`: Include pull requests in the report (default: true).

`--debug`: Enable verbose diagnostics (default: false).

`Environment Variables`

`GITHUB_TOKEN`: GitHub personal access token (required).

Token needs `repo` and `read:org` scopes for full access.

## Authentication

Issuor requires a GitHub personal access token with appropriate permissions:

Go to github Settings → developer settings → personal access tokens

Generate a new token with these scopes:

`repo` (Full control of private repositories)

`read:org` (Read org and team membership)

Export the token: `export GITHUB_TOKEN="your_token"`

## Contributors

We welcome contributions from the community! Whether it's reporting a bug, suggesting a new feature, or submitting a pull request, your input is highly valued.

<a href="https://github.com/dkooll/issuor/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=dkooll/issuor" />
</a>
