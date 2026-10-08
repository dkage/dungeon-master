// Package gatekeeper keeps agents from flooding GitHub through the gh CLI.
// Agents started by the app run `gh issue create` themselves, one request per
// call and as fast as a loop goes, which is what once locked a user out of gh
// for half an hour. The Gatekeeper puts a small gh shim first on the PATH of
// those sessions: commands that create or change things wait for a slot from
// the Quartermaster, everything else passes straight through.
package gatekeeper

import (
	"os"
	"regexp"
	"strings"
)

// Kind says whether a gh invocation reads or writes.
type Kind int

const (
	// Read is anything that does not create or change something on GitHub, and
	// also anything we do not recognise: when unsure, never get in the way.
	Read Kind = iota
	// Write creates or changes something on GitHub.
	Write
)

// writes lists the gh subcommands that create or change things, by group.
var writes = map[string]map[string]bool{
	"issue":     set("create", "edit", "close", "reopen", "comment", "delete", "transfer", "lock", "unlock", "pin", "unpin", "develop"),
	"pr":        set("create", "edit", "close", "reopen", "comment", "merge", "review", "ready", "lock", "unlock", "update-branch"),
	"label":     set("create", "edit", "delete", "clone"),
	"release":   set("create", "edit", "delete", "upload", "delete-asset"),
	"repo":      set("create", "edit", "delete", "fork", "rename", "archive", "unarchive", "sync", "deploy-key"),
	"project":   set("create", "edit", "delete", "copy", "close", "link", "unlink", "mark-template", "item-add", "item-create", "item-edit", "item-delete", "item-archive", "field-create", "field-delete"),
	"workflow":  set("run", "enable", "disable"),
	"run":       set("rerun", "cancel", "delete"),
	"gist":      set("create", "edit", "delete", "rename"),
	"secret":    set("set", "delete"),
	"variable":  set("set", "delete"),
	"ssh-key":   set("add", "delete"),
	"gpg-key":   set("add", "delete"),
	"codespace": set("create", "delete", "stop", "edit"),
	"cache":     set("delete"),
	"ruleset":   set(),
}

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}

// valueFlags are flags that take their value as the next argument and can
// appear before the subcommand (persistent flags of a command group).
var valueFlags = map[string]bool{"-R": true, "--repo": true, "--hostname": true}

// Classify decides whether a gh invocation (arguments after "gh") writes.
func Classify(args []string) Kind {
	var words []string
	argIdx := -1 // where the group's own arguments start
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") {
			if valueFlags[a] && i+1 < len(args) {
				i++
			}
			continue
		}
		words = append(words, a)
		if len(words) == 1 {
			argIdx = i + 1
		}
		if len(words) == 2 {
			break
		}
	}
	if len(words) == 0 {
		return Read
	}
	group := words[0]
	if group == "api" {
		return classifyAPI(args[argIdx:])
	}
	if len(words) < 2 {
		return Read
	}
	if subs, ok := writes[group]; ok && subs[words[1]] {
		return Write
	}
	return Read
}

var mutationWord = regexp.MustCompile(`(?i)\bmutation\b`)

// classifyAPI looks at `gh api`: the HTTP method, and for GraphQL whether the
// query is a mutation. -f/-F/--input without a method make it a POST.
func classifyAPI(args []string) Kind {
	method := ""
	hasFields := false
	graphql := false
	var query string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-X" || a == "--method":
			if i+1 < len(args) {
				method = strings.ToUpper(args[i+1])
				i++
			}
		case strings.HasPrefix(a, "--method="):
			method = strings.ToUpper(strings.TrimPrefix(a, "--method="))
		case strings.HasPrefix(a, "-X") && len(a) > 2:
			method = strings.ToUpper(a[2:])
		case a == "-f" || a == "-F" || a == "--field" || a == "--raw-field":
			hasFields = true
			if i+1 < len(args) {
				if q, ok := strings.CutPrefix(args[i+1], "query="); ok {
					query = q
				}
				i++
			}
		case strings.HasPrefix(a, "--field=") || strings.HasPrefix(a, "--raw-field="):
			hasFields = true
			v := a[strings.Index(a, "=")+1:]
			if q, ok := strings.CutPrefix(v, "query="); ok {
				query = q
			}
		case a == "--input" || strings.HasPrefix(a, "--input="):
			hasFields = true
			if a == "--input" && i+1 < len(args) {
				i++
			}
		case a == "-H" || a == "--header" || a == "--jq" || a == "-q" || a == "-t" || a == "--template" || a == "--cache" || a == "--hostname" || a == "--preview" || a == "-p":
			if i+1 < len(args) {
				i++
			}
		case !strings.HasPrefix(a, "-"):
			if a == "graphql" {
				graphql = true
			}
		}
	}
	if graphql {
		// A GraphQL call is a POST whether it reads or writes: look inside.
		if strings.HasPrefix(query, "@") {
			if b, err := os.ReadFile(query[1:]); err == nil {
				query = string(b)
			} else {
				return Read
			}
		}
		if mutationWord.MatchString(query) {
			return Write
		}
		return Read
	}
	switch method {
	case "POST", "PATCH", "PUT", "DELETE":
		return Write
	case "GET", "HEAD":
		return Read
	}
	if hasFields {
		return Write // gh sends fields as a POST body unless a method says otherwise
	}
	return Read
}
