package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/gopherd/core/flags"
)

var args struct {
	inputs flags.Slice // input directories or files
	output string      // output directory or file if it has suffix ".md"
	toc    bool        // enable table of contents
	title  string      // top title
	level  int         // heading level
	tag    string      // tag: api
}

type TemplateItem struct {
	Name     string
	Content  string
	Children map[string]*TemplateItem
	Links    []string
}

func (t *TemplateItem) IsProperty() bool {
	return strings.HasPrefix(t.Name, ".")
}

func (t *TemplateItem) NumProperties() int {
	n := 0
	for _, child := range t.Children {
		if child.IsProperty() {
			n++
		}
	}
	return n
}

func trimSpace(s string) string {
	s = strings.TrimLeft(s, " \r\n")
	s = strings.TrimRight(s, " \t\r\n")
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			return s[i+1:]
		}
		return s[i:]
	}
	return ""
}

func main() {
	flag.Var(&args.inputs, "I", "Input directories or files")
	flag.StringVar(&args.output, "o", "README.md", "Output file")
	flag.BoolVar(&args.toc, "toc", false, "Enable table of contents")
	flag.StringVar(&args.title, "title", "API Reference", "top title")
	flag.StringVar(&args.tag, "tag", "api", "Specify the tag to search for")
	flag.IntVar(&args.level, "level", 1, "heading level")
	flag.Parse()

	root := &TemplateItem{Children: make(map[string]*TemplateItem)}

	for _, input := range args.inputs {
		if err := filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
				err := processFile(path, root)
				if err != nil {
					return fmt.Errorf("error processing file %s: %v", path, err)
				}
			}
			return nil
		}); err != nil {
			fmt.Printf("Error walking through directory: %v\n", err)
			return
		}
	}

	err := generateMarkdown(root, args.output)
	if err != nil {
		fmt.Printf("Error generating Markdown: %v\n", err)
		return
	}

	fmt.Println("Markdown file generated successfully.")
}

func processFile(filename string, root *TemplateItem) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	for _, commentGroup := range node.Comments {
		processCommentGroup(fset, commentGroup, root)
	}

	return nil
}

func processCommentGroup(fset *token.FileSet, group *ast.CommentGroup, root *TemplateItem) {
	if len(group.List) == 0 {
		return
	}
	var prefix = "// @" + args.tag + "("

	firstComment := group.List[0].Text
	if !strings.HasPrefix(firstComment, prefix) {
		return
	}

	path := strings.TrimPrefix(firstComment, prefix)
	end := strings.Index(path, ")")
	currentItem := addToTree(root, path[:end])
	offset := 1
	if name := strings.TrimSpace(path[end+1:]); name != "" {
		group.List[0].Text = "// `" + currentItem.Name + "` " + name
		offset = 0
	}
	path = path[:end]

	var content bytes.Buffer
	var coding int
	const (
		codingNone = iota
		codingStart
		codingEnd
	)
	for _, comment := range group.List[offset:] {
		text := strings.TrimPrefix(comment.Text, "//")
		text = trimSpace(text)
		if strings.HasPrefix(text, "```") {
			if coding == codingStart {
				coding = codingEnd
				if content.Len() > 1 && bytes.Equal(content.Bytes()[content.Len()-2:], []byte("\n\n")) {
					content.Truncate(content.Len() - 1)
				}
			} else {
				coding = codingStart
				content.WriteString("\n")
			}
		}
		content.WriteString(text)
		if coding != codingNone || (text == "" || isListItem(text)) {
			content.WriteString("\n")
		} else {
			content.WriteString(" ")
		}
		if coding == codingEnd {
			coding = codingNone
		}
	}
	if coding == codingStart {
		fmt.Fprintf(os.Stderr, "%s: unclosed code block in %q\n", fset.Position(group.Pos()), path)
		os.Exit(1)
	}
	content.WriteString("\n")

	currentItem.Content = content.String()
	currentItem.Links = extractLinks(currentItem.Content)
}

func isListItem(text string) bool {
	return len(text) >= 2 && (text[0] == '*' || text[0] == '-' || text[0] == '+') && text[1] == ' '
}

func addToTree(root *TemplateItem, path string) *TemplateItem {
	parts := strings.Split(path, "/")
	current := root

	if len(parts) > 0 {
		if index := strings.Index(parts[len(parts)-1], "."); index != -1 {
			newPart := parts[len(parts)-1][index:]
			parts[len(parts)-1] = parts[len(parts)-1][:index]
			parts = append(parts, newPart)
		}
	}

	for _, part := range parts {
		if current.Children == nil {
			current.Children = make(map[string]*TemplateItem)
		}
		if _, exists := current.Children[part]; !exists {
			current.Children[part] = &TemplateItem{Name: part}
		}
		current = current.Children[part]
	}
	return current
}

func extractLinks(content string) []string {
	re := regexp.MustCompile(`\[([^\]]+)\]\(#([^)]+)\)`)
	matches := re.FindAllStringSubmatch(content, -1)
	links := make([]string, len(matches))
	for i, match := range matches {
		links[i] = match[2] // The anchor name is in the second capture group
	}
	return links
}

type Writers interface {
	Get(path string) io.Writer
	Close() error
}

type singleWriter struct {
	w io.Writer
}

func (s *singleWriter) Get(path string) io.Writer {
	return s.w
}

func (s *singleWriter) Close() error {
	if closer, ok := s.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type dirWriter struct {
	dir     string
	writers map[string]io.Writer
}

func snakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && (unicode.IsUpper(r) || unicode.IsNumber(r) && !unicode.IsNumber(rune(s[i-1]))) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

func (m *dirWriter) Get(path string) io.Writer {
	parts := strings.SplitN(path, "/", 2)
	name := strings.ToLower(snakeCase(parts[0]))
	if m.writers == nil {
		m.writers = make(map[string]io.Writer)
	}
	if w, ok := m.writers[name]; ok {
		return w
	}
	f, err := os.Create(filepath.Join(m.dir, name+".md"))
	if err != nil {
		panic(err)
	}
	m.writers[name] = f
	return f
}

func (m *dirWriter) Close() error {
	for _, w := range m.writers {
		if closer, ok := w.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateMarkdown(item *TemplateItem, output string) error {
	var writers Writers
	var toc *bytes.Buffer
	var content *bytes.Buffer
	if strings.HasSuffix(output, ".md") {
		if args.toc {
			toc = &bytes.Buffer{}
		}
		content = &bytes.Buffer{}
		writers = &singleWriter{w: content}
	} else {
		writers = &dirWriter{dir: output}
	}

	writeMarkdownTree(toc, writers, item, 0, "")

	if content != nil {
		file, err := os.Create(output)
		if err != nil {
			return err
		}
		defer file.Close()
		if args.title != "" {
			file.WriteString("# " + args.title + "\n\n")
		}
		if toc != nil {
			file.WriteString(toc.String() + "\n")
		}
		file.WriteString(content.String())
	} else {
		if err := writers.Close(); err != nil {
			return err
		}
	}
	return nil
}

func linkName(path string) string {
	return "user-content-" + strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(path, ".", "-"), "/", "_"), " ", "-")
}

func writeMarkdownTree(toc *bytes.Buffer, writers Writers, item *TemplateItem, depth int, parentPath string) {
	if item.Name != "" {
		level := depth + args.level
		fullPath := parentPath + item.Name
		if item.IsProperty() {
			level = 6 // <h6>
		}
		w := writers.Get(fullPath)
		if toc != nil {
			fmt.Fprintf(w, "<h%d><a id=\"%s\" target=\"_self\">%s</a></h%d>\n", level, linkName(fullPath), item.Name, level)
			if !item.IsProperty() {
				fmt.Fprintf(toc, "%s<li><a href=\"#%s\">%s</a></li>\n", strings.Repeat("  ", depth), linkName(fullPath), item.Name)
			}
		} else {
			fmt.Fprintf(w, "%s %s {#%s}\n", strings.Repeat("#", level), item.Name, linkName(fullPath))
		}

		if item.Content != "" {
			text := processLinks(item.Content)
			fmt.Fprintf(w, "\n%s\n\n", trimSpace(text))
		}
	}

	children := sortedChildren(item)
	parentPath = parentPath + item.Name
	if parentPath != "" {
		parentPath = parentPath + "/"
	}
	if toc != nil && len(children) > item.NumProperties() {
		fmt.Fprintf(toc, "<ul>\n")
	}
	for _, child := range children {
		writeMarkdownTree(toc, writers, child, depth+1, parentPath)
	}
	if toc != nil && len(children) > item.NumProperties() {
		fmt.Fprintf(toc, "</ul>\n")
	}

}

func processLinks(content string) string {
	re := regexp.MustCompile(`\[([^\]]+)\]\(#([^)]+)\)`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) == 3 {
			linkText := parts[1]
			linkPath := parts[2]
			anchorName := linkName(linkPath)
			return fmt.Sprintf("[%s](#%s)", linkText, anchorName)
		}
		return match
	})
}

func specialOrder(name string) int {
	if len(name) == 0 {
		return 0
	}
	switch name[0] {
	case '.':
		return -1
	case '-':
		return -2
	case '_':
		return -3
	default:
		return 0
	}
}

func sortedChildren(item *TemplateItem) []*TemplateItem {
	var children []*TemplateItem
	for _, child := range item.Children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		oi := specialOrder(children[i].Name)
		oj := specialOrder(children[j].Name)
		if oi != oj {
			return oi < oj
		}
		return children[i].Name < children[j].Name
	})
	return children
}
