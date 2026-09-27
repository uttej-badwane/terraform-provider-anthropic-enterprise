package apidrift

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// Field is one JSON-decoded field of a client struct.
type Field struct {
	// Type is the element type's name with pointers, slices, maps and the
	// generic Opt wrapper removed, so a nested path can be followed into it.
	// Types from other packages, such as json.RawMessage, keep their
	// qualifier and are treated as opaque.
	Type string
}

// Types maps each client struct to its JSON fields.
type Types map[string]map[string]Field

// embedKey prefixes the pseudo-field recording an embedded struct.
const embedKey = "~embed:"

// LoadTypes parses the non-test Go files in dir and records every struct's
// JSON fields. It reads source rather than reflecting over compiled types so
// that it needs no import of the client package and runs on the tree as it is.
func LoadTypes(dir string) (Types, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	types := Types{}
	embeds := map[string][]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := map[string]Field{}
				for _, fl := range st.Fields.List {
					if len(fl.Names) == 0 {
						// An embedded struct promotes its fields, as
						// encoding/json does.
						if n := elemName(fl.Type); n != "" {
							embeds[ts.Name.Name] = append(embeds[ts.Name.Name], n)
						}
						continue
					}
					jn := jsonName(fl.Tag)
					if jn == "" {
						continue
					}
					fields[jn] = Field{Type: elemName(fl.Type)}
				}
				types[ts.Name.Name] = fields
			}
		}
	}
	for name, parents := range embeds {
		for _, p := range parents {
			for k, v := range types[p] {
				if _, shadowed := types[name][k]; !shadowed {
					types[name][k] = v
				}
			}
			// Record the embedding itself so reachability can follow it. The
			// key can never collide with a JSON name, so Resolve ignores it.
			types[name][embedKey+p] = Field{Type: p}
		}
	}
	return types, nil
}

// jsonName returns the field's JSON name, or "" when it is not encoded.
func jsonName(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return ""
	}
	name, _, _ := strings.Cut(reflect.StructTag(raw).Get("json"), ",")
	if name == "-" {
		return ""
	}
	return name
}

// elemName strips pointers, slices, maps and generic wrappers down to the
// element type's name.
func elemName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return elemName(t.X)
	case *ast.ArrayType:
		return elemName(t.Elt)
	case *ast.MapType:
		return elemName(t.Value)
	case *ast.IndexExpr: // Opt[*T]
		return elemName(t.Index)
	case *ast.SelectorExpr: // json.RawMessage and the like
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	}
	return ""
}

// Resolve follows a JSON path from root and returns the struct that holds the
// last segment, the field itself, and whether the whole path is decoded.
func (t Types) Resolve(root string, path []string) (owner string, f Field, ok bool) {
	owner = root
	for i, seg := range path {
		fields, known := t[owner]
		if !known {
			return "", Field{}, false
		}
		f, ok = fields[seg]
		if !ok {
			return "", Field{}, false
		}
		if i < len(path)-1 {
			owner = f.Type
		}
	}
	return owner, f, true
}
