// Package markdown renders GitHub markdown bodies as terminal lines. Pull
// request bodies and comments arrive as GitHub-flavored markdown carrying raw
// HTML, and a release-note dump from a bot is mostly `<details>` blocks and
// anchor tags, so printing the source verbatim buries the few lines a reader
// wants. This is a reader for that text, not a conforming CommonMark parser.
package markdown

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

const (
	bullet      = "• "
	quoteMarker = "│ "
	foldMarker  = "▸ "
	ruleWidth   = 24
	indentUnit  = 2
	maxIndent   = 6
)

var (
	fenceRe       = regexp.MustCompile("^(```|~~~)")
	ruleRe        = regexp.MustCompile(`^(\*\s*\*\s*\*[\s*]*|-\s*-\s*-[\s-]*|_\s*_\s*_[\s_]*)$`)
	headingRe     = regexp.MustCompile(`^#{1,6}\s+(.*?)\s*#*$`)
	htmlHeadingRe = regexp.MustCompile(`(?i)^<h[1-6][^>]*>(.*?)</h[1-6]>$`)
	bulletRe      = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(.*)$`)
	taskRe        = regexp.MustCompile(`^\[([ xX])\]\s+(.*)$`)
	listItemRe    = regexp.MustCompile(`(?i)^<li[^>]*>(.*?)(</li>)?$`)
	summaryRe     = regexp.MustCompile(`(?i)<summary[^>]*>(.*?)</summary>`)
	tagOnlyRe     = regexp.MustCompile(`^(<[^>]+>\s*)+$`)
	commentRe     = regexp.MustCompile(`(?s)<!--.*?-->`)
	anchorRe      = regexp.MustCompile(`(?is)<a\s[^>]*>(.*?)</a>`)
	breakRe       = regexp.MustCompile(`(?i)<br\s*/?>`)
	imageRe       = regexp.MustCompile(`!\[([^\]]*)\]\(\s*<?([^)\s>]*)>?[^)]*\)`)
	linkRe        = regexp.MustCompile(`\[([^\]]+)\]\(\s*<?([^)\s>]*)>?[^)]*\)`)
	tagRe         = regexp.MustCompile(`<[^>]+>`)
	emphasisRe    = regexp.MustCompile(`\*\*|__|` + "`")
	spaceRe       = regexp.MustCompile(`[ \t]+`)
)

// Render lays out a markdown body as styled lines wrapped to width, keeping at
// most maxLines of them and saying how many it dropped. A maxLines of zero
// keeps everything.
func Render(body string, width, maxLines int) []string {
	var lines []string

	pending := false
	segs := parse(body)

	for i := range segs {
		seg := &segs[i]
		if seg.blank {
			pending = len(lines) > 0

			continue
		}

		if pending {
			lines = append(lines, "")
			pending = false
		}

		lines = append(lines, seg.render(width)...)
	}

	return clamp(lines, maxLines)
}

func clamp(lines []string, maxLines int) []string {
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}

	hidden := len(lines) - maxLines

	return append(lines[:maxLines:maxLines],
		styles.SubtitleStyle.Render("… "+strconv.Itoa(hidden)+" more lines"))
}

// segment is one rendered block: a paragraph, a list item, a folded details
// summary, or a blank separator.
type segment struct {
	text     string
	prefix   string
	style    lipgloss.Style
	blank    bool
	rule     bool
	verbatim bool
}

func (s segment) render(width int) []string {
	width = max(width, 1)

	if s.rule {
		return []string{styles.SubtitleStyle.Render(strings.Repeat("─", min(width, ruleWidth)))}
	}

	if s.verbatim {
		return []string{s.style.Render(clip(s.prefix+s.text, width))}
	}

	indent := lipgloss.Width(s.prefix)
	wrapped := strings.Split(lipgloss.Wrap(s.text, max(width-indent, 1), ""), "\n")

	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		lead := ""
		switch {
		case i > 0:
			lead = strings.Repeat(" ", indent)
		case s.prefix != "":
			lead = styles.SubtitleStyle.Render(s.prefix)
		}

		lines = append(lines, lead+s.style.Render(line))
	}

	return lines
}

func clip(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}

	return lipgloss.NewStyle().MaxWidth(width).Render(text)
}

// parser carries the state a line cannot decide on its own: whether it sits
// inside a fenced code block, and how deep in a `<details>` block it is.
type parser struct {
	fenced  bool
	depth   int
	hidden  int
	summary string
}

func parse(body string) []segment {
	var (
		p    parser
		segs []segment
	)

	body = commentRe.ReplaceAllString(body, "")
	for _, raw := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		segs = append(segs, p.line(raw)...)
	}

	return append(segs, p.flush()...)
}

func (p *parser) line(raw string) []segment {
	trimmed := strings.TrimSpace(raw)

	if p.depth > 0 || strings.HasPrefix(strings.ToLower(trimmed), "<details") {
		return p.fold(trimmed)
	}

	if fenceRe.MatchString(trimmed) {
		p.fenced = !p.fenced

		return nil
	}

	if p.fenced {
		return []segment{{text: raw, prefix: "  ", style: styles.SubtitleStyle, verbatim: true}}
	}

	return block(raw, trimmed)
}

// fold swallows a `<details>` block, leaving one line naming its summary and
// how much it hides. Bots publish whole changelogs this way, so unfolding one
// costs a reader every other line on the pane.
func (p *parser) fold(trimmed string) []segment {
	lower := strings.ToLower(trimmed)

	switch {
	case strings.HasPrefix(lower, "<details"):
		p.depth++

		return nil
	case strings.HasPrefix(lower, "</details"):
		p.depth--
		if p.depth > 0 {
			return nil
		}

		return p.flush()
	}

	if p.summary == "" {
		if match := summaryRe.FindStringSubmatch(trimmed); match != nil {
			p.summary = inline(match[1])

			return nil
		}
	}

	if trimmed != "" && !tagOnlyRe.MatchString(trimmed) {
		p.hidden++
	}

	return nil
}

func (p *parser) flush() []segment {
	if p.depth == 0 && p.summary == "" && p.hidden == 0 {
		return nil
	}

	label := p.summary
	if label == "" {
		label = "details"
	}
	if p.hidden > 0 {
		label += " (" + strconv.Itoa(p.hidden) + " lines hidden)"
	}

	p.depth, p.hidden, p.summary = 0, 0, ""

	return []segment{{text: foldMarker + label, style: styles.SubtitleStyle}}
}

func block(raw, trimmed string) []segment {
	switch {
	case trimmed == "":
		return []segment{{blank: true}}
	case ruleRe.MatchString(trimmed):
		return []segment{{rule: true}}
	case tagOnlyRe.MatchString(trimmed):
		return nil
	}

	if match := headingRe.FindStringSubmatch(trimmed); match != nil {
		return heading(match[1])
	}

	if match := htmlHeadingRe.FindStringSubmatch(trimmed); match != nil {
		return heading(match[1])
	}

	if strings.HasPrefix(trimmed, ">") {
		return []segment{{
			text:   inline(strings.TrimPrefix(trimmed, ">")),
			prefix: quoteMarker,
			style:  styles.SubtitleStyle,
		}}
	}

	if match := listItemRe.FindStringSubmatch(trimmed); match != nil {
		return listItem("", match[1])
	}

	if match := bulletRe.FindStringSubmatch(raw); match != nil {
		return listItem(match[1], match[3])
	}

	return []segment{{text: inline(trimmed), style: styles.TableRowStyle}}
}

func heading(text string) []segment {
	text = inline(text)
	if text == "" {
		return nil
	}

	return []segment{{text: text, style: styles.HeaderStyle}}
}

func listItem(indent, text string) []segment {
	marker := bullet
	if match := taskRe.FindStringSubmatch(strings.TrimSpace(text)); match != nil {
		marker = "☐ "
		if match[1] != " " {
			marker = "☑ "
		}
		text = match[2]
	}

	text = inline(text)
	if text == "" {
		return nil
	}

	pad := min(len(indent)/indentUnit*indentUnit, maxIndent)

	return []segment{{
		text:   text,
		prefix: strings.Repeat(" ", pad) + marker,
		style:  styles.TableRowStyle,
	}}
}

// inline flattens what is left of a line to plain text: links keep their
// label, tags and emphasis markers go, and entities decode.
func inline(text string) string {
	text = breakRe.ReplaceAllString(text, " ")
	text = anchorRe.ReplaceAllString(text, "$1")
	text = imageRe.ReplaceAllStringFunc(text, label(imageRe))
	text = linkRe.ReplaceAllStringFunc(text, label(linkRe))
	text = tagRe.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = emphasisRe.ReplaceAllString(text, "")

	return strings.TrimSpace(spaceRe.ReplaceAllString(text, " "))
}

// label keeps a markdown link's own text, falling back to its target when the
// text carries nothing a reader can act on (an image with no alt text).
func label(pattern *regexp.Regexp) func(string) string {
	return func(link string) string {
		match := pattern.FindStringSubmatch(link)
		if text := strings.TrimSpace(match[1]); text != "" {
			return text
		}

		return match[2]
	}
}
