package parser

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// GitHub's mentions (@octocat, @github/docs) and references to issues
// and pull requests (#123, GH-123, octocat/hello#123), found in the text
// once parsed, as email addresses are: never in code or links. Whether
// the person, team or issue exists is for a view to resolve.

const (
	maxLogin = 39  // the longest GitHub login
	maxRepo  = 100 // the longest repository name
	maxDigit = 10  // the most digits of an issue number
)

// Login returns whom a Mention mentions: a login, as octocat, or a team,
// as github/docs.
func (in Inline) Login() string {
	if in.Kind != Mention {
		return ""
	}
	return in.Text[1:]
}

// Ref returns the repository a Reference is in, as owner/name, or "" for
// the document's own, and the number of the issue or pull request.
func (in Inline) Ref() (repo string, number int) {
	if in.Kind != Reference {
		return "", 0
	}
	i := strings.LastIndexAny(in.Text, "#-")
	if in.Text[i] == '#' {
		repo = in.Text[:i]
	}
	number, _ = strconv.Atoi(in.Text[i+1:])
	return repo, number
}

// references turns the mentions and references in the text of node n's
// descendants into nodes of their own.
func (ip *inlineParser) references(n int32) {
	for c := ip.nodes[n].first; c >= 0; c = ip.nodes[c].next {
		switch ip.nodes[c].kind {
		case Link, Image:
		case Text:
			ip.splitReferences(c)
		default:
			ip.references(c)
		}
	}
}

// splitReferences splits text node c at its mentions and references.
func (ip *inlineParser) splitReferences(c int32) {
	nd := &ip.nodes[c]
	prevWord := nd.start > 0 && endsInWord(ip.s[:min(nd.start, len(ip.s))])
	for {
		nd = &ip.nodes[c]
		data := nd.text
		a, b, kind, steps := findReference(data, prevWord, ip.ext)
		ip.d.steps += steps
		if kind == Text {
			return
		}
		source := ip.s[nd.start:min(nd.end, len(ip.s))]
		start, end := nd.start, nd.end
		la := start + literalToSource(source, data, a)
		lb := start + literalToSource(source, data, b)
		r := ip.newNode(kind, data[a:b], la, lb)
		post := ip.newNode(Text, data[b:], lb, end)
		nd = &ip.nodes[c]
		nd.text, nd.start, nd.end = data[:a], start, la
		ip.insertAfter(c, r)
		ip.insertAfter(r, post)
		c, prevWord = post, endsInWord(data[:b])
	}
}

// findReference finds the first mention or reference in data, which a
// word character precedes when prevWord: its bytes and kind, or Text when
// there is none, and the bytes it scanned.
func findReference(data string, prevWord bool, ext Extensions) (a, b int, kind InlineKind, steps int) {
	mentions, refs := ext&ExtMentions != 0, ext&ExtReferences != 0
	wordBefore := func(i int) bool {
		if i == 0 {
			return prevWord
		}
		return endsInWord(data[:i])
	}
	for i := 0; i < len(data); i++ {
		j := strings.IndexAny(data[i:], "@#Gg")
		if j < 0 {
			return 0, 0, Text, steps + len(data) - i
		}
		steps += j + 1
		i += j
		switch data[i] {
		case '@':
			if mentions && !wordBefore(i) {
				if e := scanMention(data, i+1); e > 0 {
					return i, e, Mention, steps + e - i
				}
			}
		case '#':
			if !refs {
				continue
			}
			from := i
			if wordBefore(i) {
				var n int
				from, n = repoBefore(data, i, prevWord)
				steps += n
				if from < 0 {
					continue
				}
			}
			if e := scanNumber(data, i+1); e > 0 {
				return from, e, Reference, steps + e - i
			}
		default: // GH-123
			if refs && i+2 < len(data) && data[i+1]|0x20 == 'h' && data[i+2] == '-' && !wordBefore(i) {
				if e := scanNumber(data, i+3); e > 0 {
					return i, e, Reference, steps + e - i
				}
			}
		}
	}
	return 0, 0, Text, steps
}

// scanMention returns the end of the login, or team, at data[i:], after
// an @, or 0: a login is letters, digits and hyphens, from a letter or a
// digit, and a team a login, a slash and its name. No word follows it,
// nor a dot and a word, as in @example.com.
func scanMention(data string, i int) int {
	j := i
	if j >= len(data) || !isASCIIAlnum(data[j]) {
		return 0
	}
	for j < len(data) && (isASCIIAlnum(data[j]) || data[j] == '-') {
		j++
	}
	if j-i > maxLogin {
		return 0
	}
	if j+1 < len(data) && data[j] == '/' && isASCIIAlnum(data[j+1]) {
		j += 2
		for j < len(data) && (isASCIIAlnum(data[j]) || data[j] == '-' || data[j] == '_') {
			j++
		}
	}
	if startsWithWord(data[j:]) || j+1 < len(data) && data[j] == '.' && isASCIIAlnum(data[j+1]) {
		return 0
	}
	return j
}

// scanNumber returns the end of the issue number at data[i:], or 0: no
// more than maxDigit digits, from 1 to 9, which no word follows.
func scanNumber(data string, i int) int {
	j := i
	if j >= len(data) || data[j] < '1' || data[j] > '9' {
		return 0
	}
	for j < len(data) && isASCIIDigit(data[j]) {
		j++
	}
	if j-i > maxDigit || startsWithWord(data[j:]) {
		return 0
	}
	return j
}

// repoBefore returns where owner/name ends at data[h], a #, begins, or
// -1, and the bytes it scanned: no word, slash or dot precedes it.
func repoBefore(data string, h int, prevWord bool) (int, int) {
	j := h
	for j > 0 && h-j < maxRepo && isRepoChar(data[j-1]) {
		j--
	}
	if j == h || j == 0 || data[j-1] != '/' {
		return -1, h - j
	}
	o := j - 1
	k := o
	for k > 0 && o-k < maxLogin && (isASCIIAlnum(data[k-1]) || data[k-1] == '-') {
		k--
	}
	steps := h - k
	if k == o || !isASCIIAlnum(data[k]) {
		return -1, steps
	}
	if k == 0 {
		if prevWord {
			return -1, steps
		}
		return 0, steps
	}
	if c := data[k-1]; c == '/' || c == '.' || endsInWord(data[:k]) {
		return -1, steps
	}
	return k, steps
}

func isRepoChar(c byte) bool { return isASCIIAlnum(c) || c == '-' || c == '_' || c == '.' }

// isWordRune reports whether r is of a word: a letter, a digit or _.
func isWordRune(r rune) bool {
	if r < utf8.RuneSelf {
		return isASCIIAlnum(byte(r)) || r == '_'
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func endsInWord(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(s)
	return s != "" && isWordRune(r)
}

func startsWithWord(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return s != "" && isWordRune(r)
}

func isASCIIAlnum(c byte) bool { return isASCIIDigit(c) || isASCIILetter(c) }
