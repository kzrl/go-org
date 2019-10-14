package org

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// JiraWriter export an org document into Jira markup.
type JiraWriter struct {
	TagsColumn int
	strings.Builder
	indent string
}

func NewJiraWriter() *JiraWriter {
	return &JiraWriter{
		TagsColumn: 77,
	}
}

func (w *JiraWriter) Before(d *Document) {}
func (w *JiraWriter) After(d *Document)  {}

func (w *JiraWriter) emptyClone() *JiraWriter {
	wcopy := *w
	wcopy.Builder = strings.Builder{}
	return &wcopy
}

func (w *JiraWriter) nodesAsString(nodes ...Node) string {
	tmp := w.emptyClone()
	WriteNodes(tmp, nodes...)
	return tmp.String()
}

func (w *JiraWriter) WriteHeadline(h Headline) {
	tmp := w.emptyClone()
	tmp.WriteString(strings.Repeat("*", h.Lvl))
	if h.Status != "" {
		tmp.WriteString(" " + h.Status)
	}
	if h.Priority != "" {
		tmp.WriteString(" [#" + h.Priority + "]")
	}
	tmp.WriteString(" ")
	WriteNodes(tmp, h.Title...)
	hString := tmp.String()
	if len(h.Tags) != 0 {
		tString := ":" + strings.Join(h.Tags, ":") + ":"
		if n := w.TagsColumn - len(tString) - len(hString); n > 0 {
			w.WriteString(hString + strings.Repeat(" ", n) + tString)
		} else {
			w.WriteString(hString + " " + tString)
		}
	} else {
		w.WriteString(hString)
	}
	w.WriteString("\n")
	if len(h.Children) != 0 {
		w.WriteString(w.indent)
	}
	if h.Properties != nil {
		WriteNodes(w, *h.Properties)
	}
	WriteNodes(w, h.Children...)
}

func (w *JiraWriter) WriteBlock(b Block) {
	w.WriteString(w.indent + "#+BEGIN_" + b.Name)
	if len(b.Parameters) != 0 {
		w.WriteString(" " + strings.Join(b.Parameters, " "))
	}
	w.WriteString("\n")
	if isRawTextBlock(b.Name) {
		w.WriteString(w.indent)
	}
	WriteNodes(w, b.Children...)
	if !isRawTextBlock(b.Name) {
		w.WriteString(w.indent)
	}
	w.WriteString("#+END_" + b.Name + "\n")
}

func (w *JiraWriter) WriteDrawer(d Drawer) {
	w.WriteString(w.indent + ":" + d.Name + ":\n")
	WriteNodes(w, d.Children...)
	w.WriteString(w.indent + ":END:\n")
}

func (w *JiraWriter) WritePropertyDrawer(d PropertyDrawer) {
	w.WriteString(":PROPERTIES:\n")
	for _, kvPair := range d.Properties {
		k, v := kvPair[0], kvPair[1]
		if v != "" {
			v = " " + v
		}
		w.WriteString(fmt.Sprintf(":%s:%s\n", k, v))
	}
	w.WriteString(":END:\n")
}

func (w *JiraWriter) WriteFootnoteDefinition(f FootnoteDefinition) {
	w.WriteString(fmt.Sprintf("[fn:%s]", f.Name))
	content := w.nodesAsString(f.Children...)
	if content != "" && !unicode.IsSpace(rune(content[0])) {
		w.WriteString(" ")
	}
	w.WriteString(content)
}

func (w *JiraWriter) WriteParagraph(p Paragraph) {
	content := w.nodesAsString(p.Children...)
	if len(content) > 0 && content[0] != '\n' {
		w.WriteString(w.indent)
	}
	w.WriteString(content + "\n")
}

func (w *JiraWriter) WriteExample(e Example) {
	for _, n := range e.Children {
		w.WriteString(w.indent + ":")
		if content := w.nodesAsString(n); content != "" {
			w.WriteString(" " + content)
		}
		w.WriteString("\n")
	}
}

// Don't want to output #+ headers to jira
func (w *JiraWriter) WriteKeyword(k Keyword) {
	/*w.WriteString(w.indent + "#+" + k.Key + ":")
	if k.Value != "" {
		w.WriteString(" " + k.Value)
	}
	w.WriteString("\n")*/
}

func (w *JiraWriter) WriteInclude(i Include) {
	w.WriteKeyword(i.Keyword)
}

func (w *JiraWriter) WriteNodeWithMeta(n NodeWithMeta) {
	for _, ns := range n.Meta.Caption {
		w.WriteString("#+CAPTION: ")
		WriteNodes(w, ns...)
		w.WriteString("\n")
	}
	for _, attributes := range n.Meta.HTMLAttributes {
		w.WriteString("#+ATTR_HTML: ")
		w.WriteString(strings.Join(attributes, " ") + "\n")
	}
	WriteNodes(w, n.Node)
}

func (w *JiraWriter) WriteComment(c Comment) {
	w.WriteString(w.indent + "#" + c.Content + "\n")
}

func (w *JiraWriter) WriteList(l List) { WriteNodes(w, l.Items...) }

func (w *JiraWriter) WriteListItem(li ListItem) {
	liWriter := w.emptyClone()
	liWriter.indent = w.indent + strings.Repeat(" ", len(li.Bullet)+1)
	WriteNodes(liWriter, li.Children...)
	content := strings.TrimPrefix(liWriter.String(), liWriter.indent)
	w.WriteString(w.indent + li.Bullet)
	if li.Status != "" {
		w.WriteString(fmt.Sprintf(" [%s]", li.Status))
	}
	if len(content) > 0 && content[0] == '\n' {
		w.WriteString(content)
	} else {
		w.WriteString(" " + content)
	}
}

func (w *JiraWriter) WriteDescriptiveListItem(di DescriptiveListItem) {
	w.WriteString(w.indent + di.Bullet)
	if di.Status != "" {
		w.WriteString(fmt.Sprintf(" [%s]", di.Status))
	}
	indent := w.indent + strings.Repeat(" ", len(di.Bullet)+1)
	if len(di.Term) != 0 {
		term := w.nodesAsString(di.Term...)
		w.WriteString(" " + term + " ::")
		indent = indent + strings.Repeat(" ", len(term)+4)
	}
	diWriter := w.emptyClone()
	diWriter.indent = indent
	WriteNodes(diWriter, di.Details...)
	details := strings.TrimPrefix(diWriter.String(), diWriter.indent)
	if len(details) > 0 && details[0] == '\n' {
		w.WriteString(details)
	} else {
		w.WriteString(" " + details)
	}
}

func (w *JiraWriter) WriteTable(t Table) {
	for i, row := range t.Rows {

		w.WriteString(w.indent)
		if len(row.Columns) == 0 {
			//Don't do anything for the divider rows
			continue
		} else {
			if i == 0 {
				w.WriteString("||")
			} else {
				w.WriteString(`|`)
			}
			for _, column := range row.Columns {
				w.WriteString(` `)
				content := w.nodesAsString(column.Children...)
				if content == "" {
					content = " "
				}
				n := column.Len - utf8.RuneCountInString(content)
				if n < 0 {
					n = 0
				}
				if column.Align == "center" {
					if n%2 != 0 {
						w.WriteString(" ")
					}
					w.WriteString(strings.Repeat(" ", n/2) + content + strings.Repeat(" ", n/2))
				} else if column.Align == "right" {
					w.WriteString(strings.Repeat(" ", n) + content)
				} else {
					w.WriteString(content + strings.Repeat(" ", n))
				}
				if i == 0 {
					w.WriteString(` ||`)
				} else {
					w.WriteString(` |`)
				}

			}
		}
		w.WriteString("\n")
	}
}

func (w *JiraWriter) WriteHorizontalRule(hr HorizontalRule) {
	w.WriteString(w.indent + "-----\n")
}

func (w *JiraWriter) WriteText(t Text) { w.WriteString(t.Content) }

func (w *JiraWriter) WriteEmphasis(e Emphasis) {
	borders, ok := emphasisOrgBorders[e.Kind]
	if !ok {
		panic(fmt.Sprintf("bad emphasis %#v", e))
	}
	w.WriteString(borders[0])
	WriteNodes(w, e.Content...)
	w.WriteString(borders[1])
}

func (w *JiraWriter) WriteLatexFragment(l LatexFragment) {
	w.WriteString(l.OpeningPair)
	WriteNodes(w, l.Content...)
	w.WriteString(l.ClosingPair)
}

func (w *JiraWriter) WriteStatisticToken(s StatisticToken) {
	w.WriteString(fmt.Sprintf("[%s]", s.Content))
}

func (w *JiraWriter) WriteLineBreak(l LineBreak) {
	w.WriteString(strings.Repeat("\n"+w.indent, l.Count))
}

func (w *JiraWriter) WriteExplicitLineBreak(l ExplicitLineBreak) {
	w.WriteString(`\\` + "\n" + w.indent)
}

func (w *JiraWriter) WriteTimestamp(t Timestamp) {
	w.WriteString("<")
	if t.IsDate {
		w.WriteString(t.Time.Format(datestampFormat))
	} else {
		w.WriteString(t.Time.Format(timestampFormat))
	}
	if t.Interval != "" {
		w.WriteString(" " + t.Interval)
	}
	w.WriteString(">")
}

func (w *JiraWriter) WriteFootnoteLink(l FootnoteLink) {
	w.WriteString("[fn:" + l.Name)
	if l.Definition != nil {
		w.WriteString(":")
		WriteNodes(w, l.Definition.Children[0].(Paragraph).Children...)
	}
	w.WriteString("]")
}

func (w *JiraWriter) WriteRegularLink(l RegularLink) {
	if l.AutoLink {
		w.WriteString(l.URL)
	} else if l.Description == nil {
		w.WriteString(fmt.Sprintf("[[%s]]", l.URL))
	} else {
		descriptionWriter := w.emptyClone()
		WriteNodes(descriptionWriter, l.Description...)
		description := descriptionWriter.String()
		w.WriteString(fmt.Sprintf("[%s|%s]", description, l.URL))
	}
}
