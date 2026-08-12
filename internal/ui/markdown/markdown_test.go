package markdown_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/markdown"
)

// dependabotBody is the shape that motivated this package: a changelog dump
// where all but the first three lines hide behind <details>.
const dependabotBody = "Bumps the go-dependencies group with 2 updates in the / directory: " +
	"[github.com/alecthomas/chroma/v2](https://github.com/alecthomas/chroma) and " +
	"[github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea).\n" +
	"\n" +
	"Updates `github.com/alecthomas/chroma/v2` from 2.23.0 to 2.27.0\n" +
	"<details>\n" +
	"<summary>Release notes</summary>\n" +
	"<p><em>Sourced from <a href=\"https://github.com/alecthomas/chroma/releases\">chroma's releases</a>.</em></p>\n" +
	"<blockquote>\n" +
	"<h2>v2.27.0</h2>\n" +
	"<ul>\n" +
	"<li>a6d00fe fix(html): make mode class output opt-in</li>\n" +
	"<li>f52d015 chore: some house-keeping</li>\n" +
	"</ul>\n" +
	"</blockquote>\n" +
	"</details>\n"

func render(t *testing.T, body string, width, maxLines int) string {
	t.Helper()

	lines := markdown.Render(body, width, maxLines)
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("line %d is %d cells wide, past the %d it was given: %q", i, got, width, line)
		}
	}

	return plain(strings.Join(lines, "\n"))
}

// plain strips the ANSI styling so assertions read against the text.
func plain(s string) string {
	var b strings.Builder

	esc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func TestRenderFoldsReleaseNoteDumps(t *testing.T) {
	t.Parallel()

	got := render(t, dependabotBody, 60, 0)

	for _, want := range []string{
		"github.com/alecthomas/chroma/v2",
		"▸ Release notes (4 lines hidden)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering is missing %q:\n%s", want, got)
		}
	}

	for _, unwanted := range []string{"<", ">", "](", "a6d00fe", "`"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("rendering still carries %q:\n%s", unwanted, got)
		}
	}
}

func TestRenderMarkdownBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		want    []string
		notWant []string
	}{
		{
			name: "headings drop their hashes",
			body: "## What changed\ntext",
			want: []string{"What changed", "text"},
		},
		{
			name: "list items take a bullet and keep their nesting",
			body: "- top\n    - nested",
			want: []string{"• top", "  • nested"},
		},
		{
			name: "task boxes survive as boxes",
			body: "- [x] done\n- [ ] todo",
			want: []string{"☑ done", "☐ todo"},
		},
		{
			name:    "links keep the label and drop the target",
			body:    "see [the docs](https://example.com/docs) for more",
			want:    []string{"see the docs for more"},
			notWant: []string{"example.com"},
		},
		{
			name:    "a bare link label falls back to its target",
			body:    "![](https://example.com/img.png)",
			want:    []string{"https://example.com/img.png"},
			notWant: []string{"!["},
		},
		{
			name:    "html entities decode and comments vanish",
			body:    "<!-- hidden -->\nA &amp; B &lt;ok&gt;",
			want:    []string{"A & B <ok>"},
			notWant: []string{"hidden", "&amp;"},
		},
		{
			name:    "fenced code keeps its lines and loses its fences",
			body:    "```go\nfmt.Println(1)\n```",
			want:    []string{"fmt.Println(1)"},
			notWant: []string{"```"},
		},
		{
			name: "quotes take a gutter",
			body: "> quoted",
			want: []string{"│ quoted"},
		},
		{
			name:    "blank runs collapse to one line",
			body:    "first\n\n\n\nsecond",
			want:    []string{"first\n\nsecond"},
			notWant: []string{"first\n\n\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := render(t, tt.body, 60, 0)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("rendering is missing %q:\n%s", want, got)
				}
			}
			for _, unwanted := range tt.notWant {
				if strings.Contains(got, unwanted) {
					t.Errorf("rendering still carries %q:\n%s", unwanted, got)
				}
			}
		})
	}
}

// A body is wrapped by line, so a cell-count cap can miss a long one entirely.
// The line cap is what keeps a changelog from owning the pane.
func TestRenderCapsTheLineCount(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("a line of prose\n", 40)

	got := render(t, body, 60, 5)
	if lines := strings.Count(got, "\n") + 1; lines != 6 {
		t.Errorf("rendering is %d lines, want the 5 allowed plus the marker:\n%s", lines, got)
	}

	if !strings.Contains(got, "… 35 more lines") {
		t.Errorf("rendering does not say how much it dropped:\n%s", got)
	}
}

func TestRenderWrapsToTheWidthItIsGiven(t *testing.T) {
	t.Parallel()

	body := "- " + strings.Repeat("word ", 40)

	got := render(t, body, 20, 0)
	for _, line := range strings.Split(got, "\n")[1:] {
		if !strings.HasPrefix(line, "  ") {
			t.Errorf("a wrapped list item must hang under its bullet, got %q", line)
		}
	}
}
