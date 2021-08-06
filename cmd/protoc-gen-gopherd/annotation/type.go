package annotation

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/scanner"

	"github.com/gopherd/doge/encoding"
	"google.golang.org/protobuf/compiler/protogen"

	"github.com/gopherd/tools/cmd/protoc-gen-gopherd/context"
)

// @Type
// @Type(value: uint32)
// @Type(export[=boolean], min=uint32, max=uint32)
type Type struct {
	Oneof struct {
		Empty  bool
		Value  uint32
		Source struct {
			Export   *bool
			Min, Max uint32
		}
	}
}

func (*Type) Name() string { return AnnotationType }

func (t *Type) valid(associated associated) error {
	switch {
	case associated.oneof.pkg:
		if t.Oneof.Source.Max < t.Oneof.Source.Min {
			return associated.errorf("@%s: max (%d) less than min (%d)", AnnotationType, t.Oneof.Source.Max, t.Oneof.Source.Min)
		}
	case associated.oneof.message != nil:
		if t.Oneof.Source.Export != nil {
			return associated.errorf("@%s: should has no arguments or has an integer argument for message", AnnotationType)
		}
	default:
		return associated.errorf("@%s: only supported for package and message", AnnotationType)
	}
	return nil
}

func parseTypeAnnotation(associated associated, parser *encoding.Parser) (ann Annotation, err error) {
	if err = parser.Next(); err != nil {
		return
	}
	m := &Type{}
	defer func() {
		if err == nil {
			err = m.valid(associated)
		}
	}()

	if parser.Tok == scanner.EOF {
		m.Oneof.Empty = true
		ann = m
		return
	}
	if err = parser.Expect('('); err != nil {
		return
	}
	switch parser.Tok {
	case scanner.Ident:
		key := parser.Lit
		if err = parseTypeAnnotationNamedArgument(associated, parser, m, key); err != nil {
			return
		}
		seen := make(map[string]bool)
		seen[key] = true
		for parser.Tok == ',' {
			if err = parser.Next(); err != nil {
				return
			}
			if parser.Tok != scanner.Ident {
				err = parser.ExpectError(scanner.Int)
				return
			}
			key := parser.Lit
			if seen[key] {
				err = associated.errorf("@%s: named argument %s duplicated", AnnotationType, key)
				return
			}
			if err = parseTypeAnnotationNamedArgument(associated, parser, m, key); err != nil {
				return
			}
		}
	case scanner.Int:
		var value int64
		value, err = strconv.ParseInt(parser.Lit, 0, 32)
		if err != nil {
			return
		}
		if value > math.MaxUint32 || value < 0 {
			err = associated.errorf("%s %d out of range [%d, %d]", AnnotationType, value, 0, math.MaxUint32)
			return
		}
		m.Oneof.Value = uint32(value)
		if err = parser.Next(); err != nil {
			return
		}
	default:
		err = parser.ExpectError(scanner.Ident, scanner.Int)
		return
	}
	if err = parser.Expect(')'); err != nil {
		return
	}
	if err = parser.Expect(scanner.EOF); err != nil {
		return
	}

	ann = m
	return
}

func parseTypeAnnotationNamedArgument(associated associated, parser *encoding.Parser, m *Type, key string) error {
	if err := parser.Next(); err != nil {
		return err
	}
	var hasValue bool
	if parser.Tok == '=' {
		hasValue = true
		if err := parser.Next(); err != nil {
			return err
		}
	}
	value := parser.Lit

	parseUint32 := func() (uint32, error) {
		if !hasValue {
			return 0, parser.ExpectError(scanner.Int)
		}
		if parser.Tok != scanner.Int {
			return 0, parser.ExpectError(scanner.Int)
		}
		x, err := strconv.Atoi(value)
		if err == nil && (x < 0 || x > math.MaxUint32) {
			return 0, associated.errorf("@%s: %d out of range [%d, %d]", AnnotationType, x, 0, math.MaxUint32)
		}
		return uint32(x), err
	}
	parseBool := func() (*bool, error) {
		if !hasValue {
			x := true
			return &x, nil
		}
		if parser.Tok != scanner.Ident || (value != "true" && value != "false") {
			return nil, parser.ExpectValue("true or false")
		}
		x := value == "true"
		return &x, nil
	}

	var err error
	switch key {
	case "min":
		m.Oneof.Source.Min, err = parseUint32()
	case "max":
		m.Oneof.Source.Max, err = parseUint32()
	case "export":
		m.Oneof.Source.Export, err = parseBool()
	default:
		return associated.errorf("@%s: named argument %s unrecognized", AnnotationType, key)
	}
	if err != nil {
		return err
	}
	if hasValue {
		return parser.Next()
	}
	return nil
}

func generateTypeAnnotation(ctx *context.Context, gen *protogen.Plugin, f *protogen.File, g *protogen.GeneratedFile, anns []*associatedAnnotation) error {
	var (
		messageCnt         int
		packageMessageType *Type
		typesFile          *context.File
		typedAnns          []*associatedAnnotation
		emptyTypedAnns     []*associatedAnnotation
	)
	for _, ann := range anns {
		messageType := ann.annotation.(*Type)
		if ann.associated.oneof.pkg {
			packageMessageType = messageType
			continue
		}
		if ann.associated.oneof.message == nil {
			continue
		}
		if messageType.Oneof.Empty {
			emptyTypedAnns = append(emptyTypedAnns, ann)
		} else {
			typedAnns = append(typedAnns, ann)
		}
		messageCnt++
	}
	if messageCnt > 0 {
		if ctx.Type.TypesFile != "" && packageMessageType != nil && (packageMessageType.Oneof.Source.Export == nil || *packageMessageType.Oneof.Source.Export == true) {
			var parser = newTypesTxtParser(
				packageMessageType.Oneof.Source.Min,
				packageMessageType.Oneof.Source.Max,
			)
			f, err := ctx.Open(ctx.Type.TypesFile, "", parser)
			if err != nil {
				return err
			}
			parser = f.Handler.(*typesTxtParser)
			parser.setMinmax(
				packageMessageType.Oneof.Source.Min,
				packageMessageType.Oneof.Source.Max,
			)
			typesFile = f
		}
		for _, ann := range typedAnns {
			messageType := ann.annotation.(*Type)
			if typesFile != nil {
				err := typesFile.Handler.(*typesTxtParser).generator.generate(
					ctx.Type.ConstPrefix+ann.associated.oneof.message.GoIdent.GoName+ctx.Type.ConstSuffix,
					*messageType,
				)
				if err != nil {
					if isWarning(err) {
						println("gopherd: Warning: " + err.Error())
					} else {
						return err
					}
				}
			}
		}
		for _, ann := range emptyTypedAnns {
			messageType := ann.annotation.(*Type)
			if typesFile == nil {
				return ann.associated.errorf("@%s: an integer argument required while package-level @%s not found. e.g. @Type(123)",
					AnnotationType, AnnotationType,
				)
			}
			err := typesFile.Handler.(*typesTxtParser).generator.generate(
				ctx.Type.ConstPrefix+ann.associated.oneof.message.GoIdent.GoName+ctx.Type.ConstSuffix,
				*messageType,
			)
			if err != nil {
				if isWarning(err) {
					println("gopherd: Warning: " + err.Error())
				} else {
					return err
				}
			}
			typedAnns = append(typedAnns, ann)
		}
		if len(typedAnns) > 0 {
			g.P()
			g.P("const (")
			for _, ann := range typedAnns {
				messageType := ann.annotation.(*Type)
				name := ann.associated.oneof.message.GoIdent.GoName
				constName := ctx.Type.ConstPrefix + name + ctx.Type.ConstSuffix
				g.P(constName, " = ", messageType.Oneof.Value)
			}
			g.P(")")
			g.P()
			g.P("func init() {")
			for _, ann := range typedAnns {
				name := ann.associated.oneof.message.GoIdent.GoName
				constName := ctx.Type.ConstPrefix + name + ctx.Type.ConstSuffix
				g.P("\tregistry.Register(", `"`, f.GoPackageName, `",`, constName, ", func() registry.Message { return new(", name, ") })")
			}
			g.P("}")
			for _, ann := range typedAnns {
				g.P()
				name := ann.associated.oneof.message.GoIdent.GoName
				constName := ctx.Type.ConstPrefix + name + ctx.Type.ConstSuffix
				g.P("func (*", name, ") Typeof() registry.Type { return ", constName, " }")
				g.P("func (m *", name, ") Sizeof() int { return proto.Size(m) }")
				g.P("func (m *", name, ") Nameof() string { return string(proto.MessageName(m)) }")
				g.P("func (m *", name, ") Unmarshal(buf []byte) error { return proto.Unmarshal(buf, m) }")
				g.P("func (m *", name, ") MarshalAppend(buf []byte, useCachedSize bool) ([]byte, error) {")
				g.P("\treturn proto.MarshalOptions{UseCachedSize: useCachedSize}.MarshalAppend(buf, m)")
				g.P("}")
			}
		}
	}

	return nil
}

type _type struct {
	name string
	typ  uint32
}

type typeContainer struct {
	types       []_type
	nameIndices map[string]int
	typeIndices map[uint32]int
	available   struct {
		sections [][2]uint32
		values   []uint32
	}
}

func newTypeContainer(min, max uint32) *typeContainer {
	tc := &typeContainer{
		nameIndices: make(map[string]int),
		typeIndices: make(map[uint32]int),
	}
	tc.setMinmax(min, max)
	return tc
}

func (tc *typeContainer) setMinmax(min, max uint32) {
	if max >= min {
		if max-min < 1024 {
			tc.available.values = make([]uint32, max-min+1)
			for i := range tc.available.values {
				tc.available.values[i] = min + uint32(i)
			}
		} else {
			tc.available.sections = [][2]uint32{{min, max + 1}}
		}
	}
}

func (tc *typeContainer) sort() {
	sort.Slice(tc.types, func(i, j int) bool {
		return tc.types[i].typ < tc.types[j].typ
	})
}

func (tc *typeContainer) split(i int, typ uint32) {
	if typ < tc.available.sections[i][0] || typ >= tc.available.sections[i][1] {
		return
	}
	if tc.available.sections[i][0]+1 == tc.available.sections[i][1] {
		// remove empty section
		tc.available.sections = append(tc.available.sections[:i], tc.available.sections[i+1:]...)
		return
	}
	n := len(tc.available.sections)
	if tc.available.sections[i][0] == typ {
		tc.available.sections[i][0] = typ + 1
	} else if tc.available.sections[i][1] == typ+1 {
		tc.available.sections[i][1] = typ
	} else {
		tc.available.sections[i][1] = typ
		var section = [2]uint32{tc.available.sections[i][0], typ}
		tc.available.sections[i][0] = typ + 1
		tc.available.sections = append(tc.available.sections, section)
		copy(tc.available.sections[i+1:], tc.available.sections[i:n])
		tc.available.sections[i] = section
	}
}

func (tc *typeContainer) insert(name string, typ uint32) int {
	if i := tc.byType(typ); i >= 0 && tc.types[i].name != name {
		return i
	}
	i := len(tc.types)
	tc.types = append(tc.types, _type{
		name: name,
		typ:  typ,
	})
	tc.nameIndices[name] = i
	tc.typeIndices[typ] = i
	tc.onInsert(typ)
	return -1
}

func (tc *typeContainer) onInsert(typ uint32) {
	n := len(tc.available.sections)
	if n == 0 {
		n = len(tc.available.values)
		if n == 0 {
			return
		}
		i := sort.Search(n, func(i int) bool {
			return tc.available.values[i] >= typ
		})
		if i < n && tc.available.values[i] == typ {
			tc.available.values = append(tc.available.values[:i], tc.available.values[i+1:]...)
		}
	} else {
		i := sort.Search(n, func(i int) bool {
			return tc.available.sections[i][1] >= typ
		})
		if i < n && typ <= tc.available.sections[i-1][0] {
			tc.split(i, typ)
		}
	}
}

func (tc *typeContainer) remove(i int) {
	if i >= 0 {
		t := tc.types[i]
		delete(tc.nameIndices, t.name)
		delete(tc.typeIndices, t.typ)
		end := len(tc.types) - 1
		if i != end {
			tc.types[i] = tc.types[end]
			tc.nameIndices[tc.types[i].name] = i
			tc.typeIndices[tc.types[i].typ] = i
		}
		tc.types = tc.types[:end]
	}
}

func (gen *typeContainer) byName(name string) int {
	if i, ok := gen.nameIndices[name]; ok {
		return i
	}
	return -1
}

func (tc *typeContainer) byType(typ uint32) int {
	if i, ok := tc.typeIndices[typ]; ok {
		return i
	}
	return -1
}

type typeGenerator struct {
	min, max  uint32
	old, new  *typeContainer
	available [][2]uint32
}

func newTypeGenerator(min, max uint32) *typeGenerator {
	return &typeGenerator{
		min: min,
		max: max,
		old: newTypeContainer(1, 0),
		new: newTypeContainer(min, max),
	}
}

func (gen *typeGenerator) setMinmax(min, max uint32) {
	if min == gen.min && max == gen.max {
		return
	}
	gen.min = min
	gen.max = max
	gen.new.setMinmax(min, max)
	for _, t := range gen.new.types {
		gen.new.onInsert(t.typ)
	}
}

func (gen *typeGenerator) rand(name string) (uint32, error) {
	if len(gen.new.available.values) > 0 {
		i := rand.Intn(len(gen.new.available.values))
		return gen.new.available.values[i], nil
	} else if len(gen.new.available.sections) > 0 {
		var sum int
		for _, sec := range gen.new.available.sections {
			sum += int(sec[1]) - int(sec[0])
		}
		x := rand.Intn(sum)
		sum = 0
		for _, sec := range gen.new.available.sections {
			sum += int(sec[1]) - int(sec[0])
			if sum > x {
				return uint32(rand.Intn(int(sec[1])-int(sec[0]))) + sec[0], nil
			}
		}
	}
	return 0, fmt.Errorf("@%s: rand for %s failed, there is no available values in range [%d, %d]",
		AnnotationType, name, gen.min, gen.max,
	)
}

func (gen *typeGenerator) done() error {
	for i := range gen.old.types {
		t := gen.old.types[i]
		if dup := gen.new.byName(t.name); dup >= 0 {
			continue
		}
		if dup := gen.new.byType(t.typ); dup >= 0 {
			continue
		}
		gen.new.insert(t.name, t.typ)
	}
	gen.new.sort()
	return nil
}

func (gen *typeGenerator) generate(name string, t Type) error {
	if i := gen.new.byName(name); i >= 0 {
		return fmt.Errorf("@%s: name %s duplicated", AnnotationType, name)
	}

	var (
		i       = gen.old.byName(name)
		oldType uint32
		newType uint32
	)
	if i >= 0 {
		oldType = gen.old.types[i].typ
		if t.Oneof.Empty {
			if oldType > gen.max || oldType < gen.min {
				var err error
				newType, err = gen.rand(name)
				if err != nil {
					return err
				}
			} else {
				newType = oldType
			}
		} else {
			newType = t.Oneof.Value
		}
		gen.old.remove(i)
	} else {
		if t.Oneof.Empty {
			var err error
			newType, err = gen.rand(name)
			if err != nil {
				return err
			}
		} else {
			newType = t.Oneof.Value
		}
	}

	if j := gen.new.insert(name, newType); j >= 0 {
		return fmt.Errorf("@%s: %d duplicated: %s and %s", AnnotationType, gen.new.types[j].typ, gen.new.types[j].name, name)
	}
	if i >= 0 && oldType != newType {
		return warn(fmt.Sprintf("@%s: %s updated: %d -> %d", AnnotationType, name, oldType, newType))
	}
	return nil
}

// types.txt
//
// name1 = type1
// name2 = type2
// ...
type typesTxtParser struct {
	generator *typeGenerator
}

func newTypesTxtParser(min, max uint32) *typesTxtParser {
	return &typesTxtParser{
		generator: newTypeGenerator(min, max),
	}
}

func (parser *typesTxtParser) setMinmax(min, max uint32) {
	parser.generator.setMinmax(min, max)
}

func (parser *typesTxtParser) Parse(filename string, data []byte, err error) error {
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	err = nil
	s := bufio.NewScanner(bytes.NewReader(data))
	s.Split(bufio.ScanLines)
	var (
		kvRegexp = regexp.MustCompile(`([a-zA-Z_]?[a-zA-Z0-9_]*)[[:space:]]*=[[:space:]]*([0-9]+)`)
		lineno   = 0
	)
	for s.Scan() {
		lineno++
		line := strings.TrimSpace(s.Text())
		if len(line) == 0 || strings.HasPrefix(line, "//") {
			continue
		}
		result := kvRegexp.FindAllStringSubmatch(line, -1)
		if len(result) != 1 || len(result[0]) != 3 {
			return fmt.Errorf("%s:%d: invalid line, correct format: NameType = integer, result=%q", filename, lineno, result)
		}
		name := result[0][1]
		typ, err := strconv.Atoi(result[0][2])
		if err != nil {
			return fmt.Errorf("%s:%d: type `%s` is not an integer", filename, lineno, result[0][2])
		}
		parser.generator.old.insert(name, uint32(typ))
	}
	return err
}

func (parser *typesTxtParser) Output(g *protogen.GeneratedFile) {
	parser.generator.done()
	for _, t := range parser.generator.new.types {
		g.P(t.name, "=", t.typ)
	}
}

// types.proto
//
//	...
// enum <Name> {
// 	...
// }
// 	...
type typesProtoParser struct {
	leading  string
	enumName string
	trailing string

	generator *typeGenerator
}

func newTypesProtoParser(min, max uint32) *typesProtoParser {
	return &typesProtoParser{
		generator: newTypeGenerator(min, max),
	}
}

var typesProtoRegexp = regexp.MustCompile(`(?ms)(.*)enum[[:space:]]?([a-zA-Z_]?[a-zA-Z0-9_]*)[[:space:]]?{(.*)}(.*)`)

func (parser *typesProtoParser) Parse(filename string, data []byte, err error) error {
	if err != nil {
		return err
	}
	matched := typesProtoRegexp.FindAllSubmatch(data, -1)
	if len(matched) == 0 {
		return fmt.Errorf("types.proto")
	}
	return nil
}

func (parser *typesProtoParser) Output(g *protogen.GeneratedFile) {
	var (
		maxNameLength int
	)
	for i, t := range parser.generator.new.types {
		if i == 0 || len(t.name) > maxNameLength {
			maxNameLength = len(t.name)
		}
	}
	parser.generator.done()
	g.P(parser.leading)
	g.P("enum ", parser.enumName, " {")
	for _, t := range parser.generator.new.types {
		g.P("\t", t.name, strings.Repeat(" ", maxNameLength-len(t.name)), " = ", t.typ, ",")
	}
	g.P("}")
	g.P(parser.trailing)
}
