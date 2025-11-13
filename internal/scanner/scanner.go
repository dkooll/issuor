// Package scanner contains the GitHub traversal logic used by the CLI.
package scanner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/shurcooL/githubv4"
)

const graphPageSize = 100

type Item struct {
	Repo      string
	Number    int
	Title     string
	Author    string
	CreatedAt githubv4.DateTime
}

type Audience string

const (
	AudienceAll      Audience = "all"
	AudienceInternal Audience = "internal"
	AudienceExternal Audience = "external"
)

type Result struct {
	ExternalIssues []Item
	InternalIssues []Item
	ExternalPRs    []Item
	InternalPRs    []Item
	TotalRepos     int
	IncludeIssues  bool
	IncludePRs     bool
	Audience       Audience
}

type Logger func(format string, args ...any)

type Option func(*Scanner)

type Config struct {
	Organization  string
	RepoPrefix    string
	SkipUsers     []string
	IncludeIssues bool
	IncludePRs    bool
	Audience      Audience
}

type GraphQLClient interface {
	Query(ctx context.Context, q any, variables map[string]any) error
}

type Scanner struct {
	cfg       Config
	graph     GraphQLClient
	logger    Logger
	skipUsers map[string]struct{}
}

func New(cfg Config, graph GraphQLClient, opts ...Option) (*Scanner, error) {
	if graph == nil {
		return nil, errors.New("github graphql client is required")
	}
	if strings.TrimSpace(cfg.Organization) == "" {
		return nil, errors.New("organization is required")
	}
	if strings.TrimSpace(cfg.RepoPrefix) == "" {
		return nil, errors.New("repository prefix is required")
	}
	if !cfg.IncludeIssues && !cfg.IncludePRs {
		cfg.IncludeIssues = true
		cfg.IncludePRs = true
	}
	if cfg.Audience == "" {
		cfg.Audience = AudienceAll
	}
	if cfg.Audience != AudienceAll && cfg.Audience != AudienceInternal && cfg.Audience != AudienceExternal {
		return nil, fmt.Errorf("invalid audience %q (expected all, internal, or external)", cfg.Audience)
	}

	scn := &Scanner{
		cfg:       cfg,
		graph:     graph,
		skipUsers: buildSkipMap(cfg.SkipUsers),
	}
	for _, opt := range opts {
		opt(scn)
	}
	return scn, nil
}

func WithLogger(logger Logger) Option {
	return func(s *Scanner) {
		s.logger = logger
	}
}

func (s *Scanner) Scan(ctx context.Context) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	res := Result{
		IncludeIssues:  s.cfg.IncludeIssues,
		IncludePRs:     s.cfg.IncludePRs,
		Audience:       s.cfg.Audience,
		ExternalIssues: make([]Item, 0, 64),
		InternalIssues: make([]Item, 0, 64),
		ExternalPRs:    make([]Item, 0, 64),
		InternalPRs:    make([]Item, 0, 64),
	}
	repoSet := make(map[string]struct{}, 32)

	type segment struct {
		external []Item
		internal []Item
		repos    map[string]struct{}
		isPR     bool
		err      error
	}

	resultCh := make(chan segment, 2)
	var wg sync.WaitGroup

	runSearch := func(params searchParams) {
		defer wg.Done()
		ext, in, repos, err := s.search(ctx, params)
		if err != nil {
			cancel()
		}
		resultCh <- segment{external: ext, internal: in, repos: repos, isPR: params.isPR, err: err}
	}

	if s.cfg.IncludeIssues {
		wg.Add(1)
		go runSearch(searchParams{querySuffix: "is:open is:issue", isPR: false})
	}
	if s.cfg.IncludePRs {
		wg.Add(1)
		go runSearch(searchParams{querySuffix: "is:open is:pr", isPR: true})
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for seg := range resultCh {
		if seg.err != nil {
			return res, seg.err
		}
		mergeRepoSets(repoSet, seg.repos)
		if seg.isPR {
			res.ExternalPRs = append(res.ExternalPRs, seg.external...)
			res.InternalPRs = append(res.InternalPRs, seg.internal...)
		} else {
			res.ExternalIssues = append(res.ExternalIssues, seg.external...)
			res.InternalIssues = append(res.InternalIssues, seg.internal...)
		}
	}

	res.TotalRepos = len(repoSet)
	return res, nil
}

type searchParams struct {
	querySuffix string
	isPR        bool
}

func (s *Scanner) search(ctx context.Context, params searchParams) (external []Item, internal []Item, repos map[string]struct{}, err error) {
	repos = make(map[string]struct{}, 32)
	external = make([]Item, 0, 64)
	internal = make([]Item, 0, 64)

	queryString := fmt.Sprintf("org:%s %s", s.cfg.Organization, params.querySuffix)
	var cursor *githubv4.String

	for {
		var q searchQuery
		vars := map[string]any{
			"query":    githubv4.String(queryString),
			"pageSize": githubv4.Int(graphPageSize),
			"cursor":   cursor,
		}

		if err := s.graph.Query(ctx, &q, vars); err != nil {
			return nil, nil, nil, err
		}

		if s.logger != nil {
			s.logger("Fetched %d items (page), total count in search: %d", len(q.Search.Nodes), q.Search.IssueCount)
		}

		for _, node := range q.Search.Nodes {
			item, repoName, author, internalAuthor, ok := s.extractItem(node, params.isPR)
			if !ok {
				continue
			}

			if !strings.HasPrefix(repoName, s.cfg.RepoPrefix) {
				continue
			}
			repos[repoName] = struct{}{}

			if len(s.skipUsers) > 0 && author != "" {
				if _, skip := s.skipUsers[strings.ToLower(author)]; skip {
					continue
				}
			}

			if s.cfg.Audience == AudienceInternal && !internalAuthor {
				continue
			}
			if s.cfg.Audience == AudienceExternal && internalAuthor {
				continue
			}

			if internalAuthor {
				internal = append(internal, item)
			} else {
				external = append(external, item)
			}
		}

		if !q.Search.PageInfo.HasNextPage {
			break
		}
		cursor = &q.Search.PageInfo.EndCursor
	}

	return external, internal, repos, nil
}

func (s *Scanner) extractItem(node searchNode, isPR bool) (Item, string, string, bool, bool) {
	if isPR {
		if node.PullRequest.ID == "" || node.PullRequest.Repository.Name == "" {
			return Item{}, "", "", false, false
		}
		repoName := string(node.PullRequest.Repository.Name)
		author := string(node.PullRequest.Author.Login)
		itm := Item{
			Repo:      repoName,
			Number:    int(node.PullRequest.Number),
			Title:     string(node.PullRequest.Title),
			Author:    author,
			CreatedAt: node.PullRequest.CreatedAt,
		}
		return itm, repoName, author, isInternalAssociation(node.PullRequest.AuthorAssociation), true
	}

	if node.Issue.ID == "" || node.Issue.Repository.Name == "" {
		return Item{}, "", "", false, false
	}
	repoName := string(node.Issue.Repository.Name)
	author := string(node.Issue.Author.Login)
	itm := Item{
		Repo:      repoName,
		Number:    int(node.Issue.Number),
		Title:     string(node.Issue.Title),
		Author:    author,
		CreatedAt: node.Issue.CreatedAt,
	}
	return itm, repoName, author, isInternalAssociation(node.Issue.AuthorAssociation), true
}

func isInternalAssociation(assoc githubv4.CommentAuthorAssociation) bool {
	switch assoc {
	case githubv4.CommentAuthorAssociationMember,
		githubv4.CommentAuthorAssociationOwner,
		githubv4.CommentAuthorAssociationCollaborator:
		return true
	default:
		return false
	}
}

func buildSkipMap(users []string) map[string]struct{} {
	if len(users) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(users))
	for _, u := range users {
		u = strings.TrimSpace(strings.ToLower(u))
		if u == "" {
			continue
		}
		m[u] = struct{}{}
	}
	return m
}

func mergeRepoSets(dst, src map[string]struct{}) {
	if src == nil {
		return
	}
	for k := range src {
		dst[k] = struct{}{}
	}
}

type searchQuery struct {
	Search struct {
		IssueCount githubv4.Int
		PageInfo   struct {
			HasNextPage githubv4.Boolean
			EndCursor   githubv4.String
		}
		Nodes []searchNode
	} `graphql:"search(query: $query, type: ISSUE, first: $pageSize, after: $cursor)"`
}

type searchNode struct {
	Issue       issueNode       `graphql:"... on Issue"`
	PullRequest pullRequestNode `graphql:"... on PullRequest"`
}

type issueNode struct {
	ID                githubv4.ID
	Number            githubv4.Int
	Title             githubv4.String
	CreatedAt         githubv4.DateTime
	AuthorAssociation githubv4.CommentAuthorAssociation
	Author            struct {
		Login githubv4.String
	}
	Repository struct {
		Name githubv4.String
	}
}

type pullRequestNode struct {
	ID                githubv4.ID
	Number            githubv4.Int
	Title             githubv4.String
	CreatedAt         githubv4.DateTime
	AuthorAssociation githubv4.CommentAuthorAssociation
	Author            struct {
		Login githubv4.String
	}
	Repository struct {
		Name githubv4.String
	}
}
