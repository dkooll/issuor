// Package report owns formatting of scan results.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/dkooll/issuor/internal/scanner"
)

type Printer struct {
	out io.Writer
}

func New(out io.Writer) *Printer {
	return &Printer{out: out}
}

func (p *Printer) Print(res scanner.Result) {
	if res.IncludeIssues {
		p.printSection("external issues", res.ExternalIssues, res.Audience)
		p.printSection("internal issues", res.InternalIssues, res.Audience)
	}

	if res.IncludePRs {
		p.printSection("external prs", res.ExternalPRs, res.Audience)
		p.printSection("internal prs", res.InternalPRs, res.Audience)
	}

	fmt.Fprintf(p.out, "\n\033[1msummary\033[0m\n")
	fmt.Fprintf(p.out, "\033[1mrepositories scanned (%d)\033[0m\n", res.TotalRepos)
	if res.IncludeIssues {
		fmt.Fprintf(p.out, "\033[1missues (%d external, %d internal)\033[0m\n",
			len(res.ExternalIssues), len(res.InternalIssues))
	}
	if res.IncludePRs {
		fmt.Fprintf(p.out, "\033[1mprs (%d external, %d internal)\033[0m\n",
			len(res.ExternalPRs), len(res.InternalPRs))
	}
}

func (p *Printer) printSection(label string, items []scanner.Item, audience scanner.Audience) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(p.out, "\n\033[1m%s (%d)\033[0m\n", label, len(items))
	fmt.Fprintf(p.out, "\033[1maudience (%s)\033[0m\n", audience)
	fmt.Fprintf(p.out, "\033[1mrepos (%d)\033[0m\n\n", uniqueRepoCount(items))

	tw := tabwriter.NewWriter(p.out, 0, 0, 2, ' ', 0)
	printGrouped(tw, items, strings.Fields(label)[0], 70)
	_ = tw.Flush()
	fmt.Fprintln(p.out)
}

func printGrouped(w *tabwriter.Writer, items []scanner.Item, _ string, titleWidth int) {
	if len(items) == 0 {
		fmt.Fprintln(w, "(none)\t\t\t")
		return
	}

	byRepo := make(map[string][]scanner.Item, len(items)/2)
	repos := make([]string, 0, len(items)/2)
	for _, itm := range items {
		if _, ok := byRepo[itm.Repo]; !ok {
			repos = append(repos, itm.Repo)
		}
		byRepo[itm.Repo] = append(byRepo[itm.Repo], itm)
	}
	sort.Strings(repos)

	for _, repo := range repos {
		first := true
		for _, itm := range byRepo[repo] {
			repoCell := ""
			if first {
				repoCell = repo
				first = false
			}
			title := normalizeTitle(itm.Title)
			titleCell := truncateTitle(title, titleWidth)
			ageCell := formatAge(itm.CreatedAt.Time)
			fmt.Fprintf(w, "%s\t#%d\t%s\t%s\t%s\n",
				repoCell,
				itm.Number,
				titleCell,
				ageCell,
				strings.ToLower(itm.Author),
			)
		}
	}
}

func normalizeTitle(raw string) string {
	title := strings.ToLower(strings.TrimSpace(raw))
	title = removeBracketed(title)
	title = stripDescriptor(title)
	fields := strings.Fields(title)
	if len(fields) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(title))
	sb.WriteString(fields[0])
	for i := 1; i < len(fields); i++ {
		sb.WriteByte(' ')
		sb.WriteString(fields[i])
	}
	return sb.String()
}

func removeBracketed(title string) string {
	return strings.Map(func(r rune) rune {
		if r == '[' || r == ']' {
			return -1
		}
		return r
	}, title)
}

func stripDescriptor(title string) string {
	idx := strings.IndexAny(title, ":-")
	if idx <= 0 {
		return title
	}
	prefix := strings.TrimSpace(title[:idx])
	if prefix == "" {
		return title
	}
	for _, r := range prefix {
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			continue
		}
		return title
	}
	rest := strings.TrimSpace(title[idx+1:])
	if rest == "" {
		return title
	}
	return rest
}

func uniqueRepoCount(items []scanner.Item) int {
	set := make(map[string]struct{})
	for _, itm := range items {
		set[itm.Repo] = struct{}{}
	}
	return len(set)
}

func truncateTitle(title string, limit int) string {
	if len(title) <= limit {
		return title
	}
	if limit <= 3 {
		return title[:limit]
	}
	return strings.TrimSpace(title[:limit-3]) + "..."
}

func formatAge(created time.Time) string {
	duration := time.Since(created)
	days := int(duration.Hours() / 24)

	if days == 0 {
		return "today"
	} else if days == 1 {
		return "1 day"
	} else if days < 7 {
		return fmt.Sprintf("%d days", days)
	} else if days < 30 {
		weeks := days / 7
		if weeks == 1 {
			return "1 week"
		}
		return fmt.Sprintf("%d weeks", weeks)
	} else if days < 365 {
		months := days / 30
		if months == 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", months)
	} else {
		years := days / 365
		if years == 1 {
			return "1 year"
		}
		return fmt.Sprintf("%d years", years)
	}
}
