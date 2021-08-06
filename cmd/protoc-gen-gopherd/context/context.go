package context

import (
	"io/ioutil"

	"google.golang.org/protobuf/compiler/protogen"
)

type File struct {
	GeneratedFile *protogen.GeneratedFile
	Handler       Handler
	Error         error
}

type Handler interface {
	Parse(string, []byte, error) error
	Output(*protogen.GeneratedFile)
}

type Context struct {
	plugin *protogen.Plugin
	files  map[string]*File

	Type struct {
		TypesFile    string
		ConstPrefix  string
		ConstSuffix  string
		TypeRegistry string
	}
	ProtobufPkg string
}

func New(plugin *protogen.Plugin) *Context {
	return &Context{
		plugin: plugin,
		files:  make(map[string]*File),
	}
}

func (ctx *Context) Open(filename string, goImportPath protogen.GoImportPath, handler Handler) (*File, error) {
	if file, ok := ctx.files[filename]; ok {
		return file, file.Error
	}
	file := &File{
		GeneratedFile: ctx.plugin.NewGeneratedFile(filename, goImportPath),
	}
	ctx.files[filename] = file
	if handler != nil {
		data, err := ioutil.ReadFile(filename)
		err = handler.Parse(filename, data, err)
		if err != nil {
			println("gopherd: open file", filename, "error:", err.Error())
			file.Error = err
			return nil, err
		}
		file.Handler = handler
	}
	return file, nil
}

func (ctx *Context) Done() {
	for _, f := range ctx.files {
		if f.Error != nil {
			continue
		}
		f.Handler.Output(f.GeneratedFile)
	}
}
