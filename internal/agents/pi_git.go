package agents

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	whatwg "github.com/nlnwa/whatwg-url/url"
)

// Pi tries hosted-git-info before generic Git URLs. GitHub's hosted rules matter
// here because they normalize shortcuts, tree URLs, escaped names, and refs.
// Other providers' shortcuts must not become GitHub credentials in the HTTPS
// fallback. Their URL-specific extraction rules are irrelevant to tap.
// https://github.com/earendil-works/pi/blob/v0.85.1/packages/coding-agent/src/utils/git.ts
func parsePiGitSource(source string) (host, repo string) {
	// parseSource gives local paths priority over parseGitUrl.
	if piLocalSource(source) {
		return "", ""
	}
	source = trimPiSpace(source)
	if strings.HasPrefix(source, "git:") {
		source = trimPiSpace(source[4:])
	} else if !piGitProtocol(source) {
		return "", ""
	}
	base, ref := splitPiGitRef(source)
	candidates := []string{}
	if ref != "" {
		candidates = append(candidates, base+"#"+ref)
	}
	candidates = append(candidates, source)
	if ref != "" {
		candidates = append(candidates, "https://"+base+"#"+ref)
	}
	candidates = append(candidates, "https://"+source)
	for _, candidate := range candidates {
		host, owner, project, ok := piHostedRepository(candidate)
		if ok && !(ref != "" && strings.Contains(project, "@")) {
			return piGitRepository(host, owner+"/"+project)
		}
	}
	if host, repo, ok := splitPiSCP(base); ok {
		return piGitRepository(host, repo)
	}
	if piGitProtocol(base) {
		if u, err := whatwg.Parse(base); err == nil {
			return piGitRepository(u.Hostname(), strings.TrimLeft(u.Pathname(), "/"))
		}
		return "", ""
	}
	host, repo, _ = strings.Cut(base, "/")
	if !strings.Contains(host, ".") && host != "localhost" {
		return "", ""
	}
	return piGitRepository(host, repo)
}

func piGitProtocol(source string) bool {
	return strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "ssh://") || strings.HasPrefix(source, "git://")
}

func splitPiSCP(source string) (host, repo string, ok bool) {
	if after, prefixed := strings.CutPrefix(source, "git@"); prefixed {
		host, repo, ok = strings.Cut(after, ":")
		ok = ok && host != "" && repo != "" && !strings.ContainsAny(repo, "\r\n\u2028\u2029")
	}
	return
}

func splitPiGitRef(source string) (string, string) {
	if host, repo, ok := splitPiSCP(source); ok {
		if repo, ref, ok := strings.Cut(repo, "@"); ok && repo != "" && ref != "" {
			return "git@" + host + ":" + repo, ref
		}
		return source, ""
	}
	if strings.Contains(source, "://") {
		if u, err := whatwg.Parse(source); err == nil {
			if repo, ref, ok := strings.Cut(strings.TrimLeft(u.Pathname(), "/"), "@"); ok && repo != "" && ref != "" {
				u.SetPathname("/" + repo)
				return strings.TrimSuffix(u.Href(false), "/"), ref
			}
		}
		return source, ""
	}
	if slash := strings.IndexByte(source, '/'); slash >= 0 {
		if repo, ref, ok := strings.Cut(source[slash+1:], "@"); ok && repo != "" && ref != "" {
			return source[:slash+1] + repo, ref
		}
	}
	return source, ""
}

func piGitRepository(host, repo string) (string, string) {
	repo = strings.TrimSuffix(repo, ".git")
	if host == "" || repo == "" || len(strings.Split(repo, "/")) < 2 {
		return "", ""
	}
	for i, part := range []string{host, repo} {
		decoded, err := decodePiGitPart(part)
		if err != nil {
			return "", ""
		}
		for _, value := range []string{part, decoded} {
			if strings.ContainsAny(value, "\x00\\") || strings.HasPrefix(value, "/") || (i == 0 && strings.Contains(value, "/")) {
				return "", ""
			}
			for _, segment := range strings.Split(value, "/") {
				if segment == ".." {
					return "", ""
				}
			}
		}
	}
	return host, repo
}

func decodePiGitPart(value string) (string, error) {
	decoded, err := url.PathUnescape(value)
	if err == nil && !utf8.ValidString(decoded) {
		err = url.EscapeError(value)
	}
	return decoded, err
}

func trimPiSpace(value string) string {
	return strings.TrimFunc(value, piSpace)
}

func piSpace(r rune) bool {
	return r == '\ufeff' || (r != '\u0085' && unicode.IsSpace(r))
}

var piHostedShortcuts = map[string]string{
	"github:":    "github.com",
	"gitlab:":    "gitlab.com",
	"bitbucket:": "bitbucket.org",
	"gist:":      "gist.github.com",
	"sourcehut:": "git.sr.ht",
}

// GitHub extraction and URL correction follow hosted-git-info 9.0.3:
// https://github.com/npm/hosted-git-info/tree/v9.0.3/lib
//
// Copyright (c) 2015, Rebecca Turner
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
// REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
// AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
// INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
// LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
// OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
// PERFORMANCE OF THIS SOFTWARE.
func piHostedRepository(source string) (host, owner, project string, ok bool) {
	beforeHash, _, _ := strings.Cut(source, "#")
	if strings.Count(beforeHash, "/") == 1 && !strings.HasPrefix(beforeHash, ".") &&
		!strings.HasPrefix(beforeHash, "/") && !strings.HasSuffix(beforeHash, "/") &&
		!strings.ContainsAny(beforeHash, "@:") && strings.IndexFunc(beforeHash, piSpace) < 0 {
		source = "github:" + source
	}
	u := piHostedURL(source)
	if u == nil {
		return "", "", "", false
	}
	host = piHostedShortcuts[u.Protocol()]
	shortcut := host != ""
	var ref string
	if shortcut {
		name := strings.TrimPrefix(u.Pathname(), "/")
		if at := strings.IndexByte(name, '@'); at >= 0 {
			name = name[at+1:]
		}
		if slash := strings.LastIndexByte(name, '/'); slash >= 0 {
			owner, project = name[:slash], name[slash+1:]
		} else {
			project = name
		}
		if owner == "" {
			owner = "null"
		}
		ref = strings.TrimPrefix(u.Hash(), "#")
	} else {
		host = strings.TrimPrefix(u.Hostname(), "www.")
		if host != "github.com" {
			return "", "", "", false
		}
		switch u.Protocol() {
		case "git:", "http:", "git+ssh:", "git+https:", "ssh:", "https:":
		default:
			return "", "", "", false
		}
		parts := strings.SplitN(u.Pathname(), "/", 6)
		if len(parts) < 3 || parts[1] == "" || parts[2] == "" {
			return "", "", "", false
		}
		owner, project = parts[1], strings.TrimSuffix(parts[2], ".git")
		if len(parts) > 3 && parts[3] != "" {
			if parts[3] != "tree" {
				return "", "", "", false
			}
			if len(parts) > 4 {
				ref = parts[4]
			}
		} else {
			ref = strings.TrimPrefix(u.Hash(), "#")
		}
	}
	owner, ownerErr := decodePiGitPart(owner)
	project, projectErr := decodePiGitPart(project)
	_, refErr := decodePiGitPart(ref)
	if shortcut {
		project = strings.TrimSuffix(project, ".git")
	}
	return host, owner, project, ownerErr == nil && projectErr == nil && refErr == nil
}

func piHostedURL(source string) *whatwg.Url {
	colon := strings.IndexByte(source, ':')
	switch source[:colon+1] {
	case "git+ssh:", "ssh:", "git+https:", "git:", "http:", "https:", "git+http:":
	default:
		if piHostedShortcuts[source[:colon+1]] == "" && !strings.HasPrefix(source[colon+1:], "//") {
			if at := strings.IndexByte(source, '@'); at >= 0 {
				if at > colon {
					source = "git+ssh://" + source
				}
			} else {
				source = source[:colon+1] + "//" + source[colon+1:]
			}
		}
	}
	if u, err := whatwg.Parse(source); err == nil {
		return u
	}
	beforeHash, _, _ := strings.Cut(source, "#")
	if colon := strings.LastIndexByte(beforeHash, ':'); colon > strings.LastIndexByte(beforeHash, '@') {
		source = source[:colon] + "/" + source[colon+1:]
	}
	beforeHash, _, _ = strings.Cut(source, "#")
	if !strings.Contains(beforeHash, ":") && !strings.Contains(source, "//") {
		source = "git+ssh://" + source
	}
	u, _ := whatwg.Parse(source)
	return u
}
