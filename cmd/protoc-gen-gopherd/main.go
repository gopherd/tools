package main

import (
	"flag"
	"fmt"

	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"

	"github.com/gopherd/tools/cmd/protoc-gen-gopherd/annotation"
	"github.com/gopherd/tools/cmd/protoc-gen-gopherd/context"
)

func main() {
	var (
		flags       flag.FlagSet
		typesFile   = flags.String("types_file", "", "message type filename for store message types")
		typePrefix  = flags.String("const_prefix", "", "message type const prefix")
		typeSuffix  = flags.String("const_suffix", "Type", "message type const suffix")
		typeMethod  = flags.String("type_method", "Type", "message type method name")
		typeRegisty = flags.String("type_registry", "github.com/gopherd/doge/proto", "typed message registry go package")
	)
	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		var ctx = context.New(gen)
		ctx.Type.TypesFile = *typesFile
		ctx.Type.ConstPrefix = *typePrefix
		ctx.Type.ConstSuffix = *typeSuffix
		ctx.Type.TypeMethod = *typeMethod
		ctx.Type.TypeRegistry = *typeRegisty
		if ctx.Type.ConstPrefix == "" && ctx.Type.ConstSuffix == "" {
			return fmt.Errorf("gopherd plugin flags const_prefix and const_suffix are both empty")
		}
		if ctx.Type.TypeMethod != "" && ctx.Type.TypeRegistry == "" {
			return fmt.Errorf("gopherd plugin flags type_registry MUST be non-empty while type_method non-empty")
		}
		for _, f := range gen.Files {
			if f.Generate {
				gengo.GenerateFile(gen, f)
				if err := annotation.Generate(ctx, gen, f); err != nil {
					return err
				}
			}
		}
		gen.SupportedFeatures = gengo.SupportedFeatures
		ctx.Done()
		return nil
	})
}
