package gatekeeper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func args(s string) []string { return strings.Fields(s) }

func TestClassifyIssueAndFriends(t *testing.T) {
	writesCmds := []string{
		"issue create", "issue create --title x --body y --label bug", "issue edit 12 --add-label x", "issue close 3",
		"issue reopen 3", "issue comment 3 --body hi", "issue delete 3 --yes", "issue transfer 3 other/repo", "issue lock 3",
		"issue unlock 3", "issue pin 3", "issue unpin 3", "issue develop 3 --name b",
		"pr create --fill", "pr edit 4 --title x", "pr close 4", "pr reopen 4", "pr comment 4 -b hi", "pr merge 4 --squash",
		"pr review 4 --approve", "pr ready 4", "pr lock 4", "pr update-branch 4",
		"label create bug", "label edit bug --color f00", "label delete bug", "label clone other/repo",
		"release create v1", "release edit v1", "release delete v1", "release upload v1 a.zip", "release delete-asset v1 a.zip",
		"repo create x", "repo edit --description y", "repo delete x --yes", "repo fork", "repo rename y", "repo archive x", "repo sync",
		"project create", "project item-add 1 --url u", "project item-create 1", "project item-edit", "project field-create 1",
		"workflow run ci.yml", "workflow enable ci", "workflow disable ci",
		"run rerun 123", "run cancel 123", "run delete 123",
		"gist create f", "gist edit id", "gist delete id",
		"secret set X", "variable set X", "secret delete X",
		"ssh-key add k", "gpg-key add k", "codespace create", "cache delete 1",
	}
	for _, c := range writesCmds {
		if Classify(args(c)) != Write {
			t.Errorf("%q should be a write", c)
		}
	}
	reads := []string{
		"issue list", "issue list --label bug --limit 100", "issue view 3", "issue status", "issue list --json number,title",
		"pr list", "pr view 4", "pr diff 4", "pr checks 4", "pr status", "pr checkout 4",
		"label list", "release list", "release view v1", "release download v1",
		"repo view", "repo list", "repo clone x", "repo set-default",
		"project list", "project view 1", "project item-list 1", "project field-list 1",
		"workflow list", "workflow view ci", "run list", "run view 123", "run watch 123",
		"gist list", "gist view id", "gist clone id", "secret list", "variable list", "ssh-key list", "codespace list",
		"auth status", "auth token", "auth login", "config get editor", "search issues foo", "search repos x", "browse",
		"status", "extension list", "alias list", "completion -s bash", "--version", "version", "", "help issue",
	}
	for _, c := range reads {
		if Classify(args(c)) != Read {
			t.Errorf("%q should be a read", c)
		}
	}
}

func TestClassifySkipsFlagsAndTheirValuesBeforeTheSubcommand(t *testing.T) {
	for cmd, want := range map[string]Kind{
		"-R owner/repo issue create --title x":       Write,
		"--repo owner/repo issue create":             Write,
		"issue -R owner/repo create":                 Write,
		"issue --repo owner/repo create":             Write,
		"-R owner/repo issue list":                   Read,
		"issue -R owner/repo list":                   Read,
		"--hostname github.example.com issue create": Write,
		"--help":          Read,
		"issue --help":    Read,
		"-R":              Read,
		"issue":           Read,
		"-R create issue": Read, // "create" was the value of -R, "issue" has no subcommand
	} {
		if got := Classify(args(cmd)); got != want {
			t.Errorf("%q: got %v want %v", cmd, got, want)
		}
	}
}

func TestClassifyAPI(t *testing.T) {
	cases := map[string]Kind{
		"api repos/o/r/issues":                            Read,
		"api repos/o/r/issues --paginate":                 Read,
		"api repos/o/r/issues -X POST -f title=x":         Write,
		"api repos/o/r/issues --method POST -f title=x":   Write,
		"api repos/o/r/issues --method=PATCH":             Write,
		"api repos/o/r/issues/1 -XPATCH -f state=closed":  Write,
		"api repos/o/r/issues/1 -X DELETE":                Write,
		"api repos/o/r/issues/1 -X PUT":                   Write,
		"api repos/o/r/issues -X post -f a=b":             Write, // case-insensitive
		"api repos/o/r/issues -X GET -f state=open":       Read,  // fields on a GET are query parameters
		"api repos/o/r/issues -X HEAD":                    Read,
		"api repos/o/r/issues -f title=x":                 Write, // fields imply POST
		"api repos/o/r/issues -F title=x":                 Write,
		"api repos/o/r/issues --field title=x":            Write,
		"api repos/o/r/issues --raw-field title=x":        Write,
		"api repos/o/r/issues --field=title=x":            Write,
		"api repos/o/r/issues --input body.json":          Write,
		"api repos/o/r/issues --input=body.json":          Write,
		"api repos/o/r/issues -H Accept:application/json": Read,
		"api repos/o/r/issues --jq .[].number":            Read,
		"api rate_limit":                                  Read,
		"api -X POST repos/o/r/issues":                    Write,
		"api":                                             Read,
		"api graphql -f query=query{viewer{login}}":       Read,
		"api graphql -F query=query{viewer{login}}":       Read,
		"api graphql -f query=mutation{createIssue}":      Write,
		"api graphql -f query=Mutation{x}":                Write,
		"api graphql --field query=mutation{x}":           Write,
		"api graphql --raw-field=query=mutation{x}":       Write,
		"api graphql -f owner=o -f query=mutation{x}":     Write,
		"api graphql -f owner=o -f name=r":                Read, // no query given: nothing to write
		"api graphql":                                     Read,
		"api graphql -f query=query{r{mutationsCount}}":   Read, // "mutationsCount" is not the keyword
	}
	for cmd, want := range cases {
		if got := Classify(args(cmd)); got != want {
			t.Errorf("%q: got %v want %v", cmd, got, want)
		}
	}
}

func TestClassifyGraphQLQueryFromAFile(t *testing.T) {
	dir := t.TempDir()
	m := filepath.Join(dir, "m.graphql")
	q := filepath.Join(dir, "q.graphql")
	os.WriteFile(m, []byte("mutation { addComment(input:{}) { clientMutationId } }"), 0o644)
	os.WriteFile(q, []byte("query { viewer { login } }"), 0o644)
	if Classify([]string{"api", "graphql", "-F", "query=@" + m}) != Write {
		t.Error("a mutation in a file should be a write")
	}
	if Classify([]string{"api", "graphql", "-F", "query=@" + q}) != Read {
		t.Error("a query in a file should be a read")
	}
	if Classify([]string{"api", "graphql", "-F", "query=@" + filepath.Join(dir, "missing")}) != Read {
		t.Error("an unreadable file must not block work")
	}
}

func TestUnknownThingsAreReads(t *testing.T) {
	for _, c := range []string{"frobnicate", "frobnicate now", "myalias create", "co 12", "extension exec foo", "issue frobnicate"} {
		if Classify(args(c)) != Read {
			t.Errorf("%q must pass through", c)
		}
	}
	// Arguments after "--" are never treated as the subcommand.
	if Classify([]string{"--", "issue", "create"}) != Read {
		t.Error("after --")
	}
}

func TestClassifyDoesNotMutateItsInput(t *testing.T) {
	in := []string{"-R", "o/r", "issue", "create", "--title", "x"}
	cp := append([]string(nil), in...)
	Classify(in)
	for i := range in {
		if in[i] != cp[i] {
			t.Fatal("input modified")
		}
	}
}
