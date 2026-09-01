// Command wtsinventory extracts WTS endpoint paths from production Go source.
// It uses the Go parser so comments and string-like text cannot create false
// ownership, while endpoints embedded in full URLs and fmt templates remain
// visible to the inventory audit.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/monitor"
)

var (
	apiPathRE     = regexp.MustCompile(`/api/v[0-9]+/[A-Za-z0-9/_.%+\-\[\]{}]+`)
	fmtVerbRE     = regexp.MustCompile(`%(?:\[[0-9]+\])?[-+#0 ']*[0-9]*(?:\.[0-9]+)?[vTtbcdoOqxXUeEfgGsxp%]`)
	fmtPositionRE = regexp.MustCompile(`^%\[([0-9]+)\]`)
)

type probeFact struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Host   string `json:"host"`
	Path   string `json:"path"`
}

func main() {
	root := flag.String("root", ".", "repository root to inspect")
	roots := flag.String("roots", "", "comma-separated production Go files or directories")
	mode := flag.String("mode", "exposures", "output mode: exposures or probes")
	flag.Parse()

	var (
		payload any
		err     error
	)
	switch *mode {
	case "exposures":
		if strings.TrimSpace(*roots) == "" {
			err = fmt.Errorf("-roots is required in exposures mode")
			break
		}
		payload, err = discover(*root, strings.Split(*roots, ","))
	case "probes":
		payload, err = runtimeProbes()
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func discover(root string, sourceRoots []string) (map[string][]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	sources := map[string]map[string]struct{}{}
	for _, relSource := range sourceRoots {
		relSource = strings.TrimSpace(relSource)
		if relSource == "" {
			continue
		}
		source := filepath.Join(root, filepath.FromSlash(relSource))
		info, err := os.Stat(source)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source root %s: %w", relSource, err)
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if strings.HasSuffix(source, ".go") && !strings.HasSuffix(source, "_test.go") {
				if err := discoverFile(root, source, sources); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			return discoverFile(root, path, sources)
		}); err != nil {
			return nil, err
		}
	}

	out := make(map[string][]string, len(sources))
	for path, owners := range sources {
		for owner := range owners {
			out[path] = append(out[path], owner)
		}
		sort.Strings(out[path])
	}
	return out, nil
}

func discoverFile(root, filename string, exposures map[string]map[string]struct{}) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", filename, err)
	}
	rel, err := filepath.Rel(root, filename)
	if err != nil {
		return err
	}
	constants := fileStringConstants(file)
	record := func(value string) {
		for _, found := range apiPathRE.FindAllString(value, -1) {
			if exposures[found] == nil {
				exposures[found] = map[string]struct{}{}
			}
			exposures[found][filepath.ToSlash(rel)] = struct{}{}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch expression := node.(type) {
		case *ast.CallExpr:
			if value, ok := sprintfStringTemplate(expression, constants); ok {
				record(value)
				return false
			}
			return true
		case *ast.BinaryExpr:
			if expression.Op != token.ADD {
				return true
			}
			if value, ok := stringTemplate(expression, constants); ok {
				record(value)
			}
			return false
		case *ast.BasicLit:
			if expression.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(expression.Value)
			if err == nil {
				record(value)
			}
		}
		return true
	})
	return nil
}

func fileStringConstants(file *ast.File) map[string]string {
	expressions := make(map[string]ast.Expr)
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			values, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range values.Names {
				if index < len(values.Values) {
					expressions[name.Name] = values.Values[index]
				} else if len(values.Values) == 1 {
					expressions[name.Name] = values.Values[0]
				}
			}
		}
	}

	constants := make(map[string]string, len(expressions))
	for range len(expressions) {
		progress := false
		for name, expression := range expressions {
			if _, resolved := constants[name]; resolved {
				continue
			}
			if value, ok := constantString(expression, constants); ok {
				constants[name] = value
				progress = true
			}
		}
		if !progress {
			break
		}
	}
	return constants
}

func constantString(expression ast.Expr, constants map[string]string) (string, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		decoded, err := strconv.Unquote(value.Value)
		return decoded, err == nil
	case *ast.Ident:
		resolved, ok := constants[value.Name]
		return resolved, ok
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := constantString(value.X, constants)
		right, rightOK := constantString(value.Y, constants)
		return left + right, leftOK && rightOK
	case *ast.ParenExpr:
		return constantString(value.X, constants)
	default:
		return "", false
	}
}

func stringTemplate(expression ast.Expr, constants map[string]string) (string, bool) {
	if resolved, ok := constantString(expression, constants); ok {
		return resolved, true
	}
	switch value := expression.(type) {
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "{param}", false
		}
		left, leftLiteral := stringTemplate(value.X, constants)
		right, rightLiteral := stringTemplate(value.Y, constants)
		return left + right, leftLiteral || rightLiteral
	case *ast.ParenExpr:
		return stringTemplate(value.X, constants)
	case *ast.CallExpr:
		return sprintfStringTemplate(value, constants)
	default:
		return "{param}", false
	}
}

func sprintfStringTemplate(call *ast.CallExpr, constants map[string]string) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Sprintf" || len(call.Args) == 0 {
		return "{param}", false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok || packageName.Name != "fmt" {
		return "{param}", false
	}
	format, ok := constantString(call.Args[0], constants)
	if !ok {
		return "{param}", false
	}

	var output strings.Builder
	cursor, nextArg := 0, 1
	hasLiteral := format != ""
	for _, location := range fmtVerbRE.FindAllStringIndex(format, -1) {
		output.WriteString(format[cursor:location[0]])
		verb := format[location[0]:location[1]]
		cursor = location[1]
		if verb == "%%" {
			output.WriteByte('%')
			continue
		}

		argIndex := nextArg
		if match := fmtPositionRE.FindStringSubmatch(verb); len(match) == 2 {
			if explicit, err := strconv.Atoi(match[1]); err == nil {
				argIndex = explicit
				nextArg = explicit + 1
			}
		} else {
			nextArg++
		}
		if argIndex >= len(call.Args) {
			output.WriteString("{param}")
			continue
		}
		argument, literal := stringTemplate(call.Args[argIndex], constants)
		output.WriteString(argument)
		hasLiteral = hasLiteral || literal
	}
	output.WriteString(format[cursor:])
	return output.String(), hasLiteral
}

func runtimeProbes() ([]probeFact, error) {
	probes := monitor.Probes()
	out := make([]probeFact, 0, len(probes))
	for _, probe := range probes {
		u, err := url.Parse(probe.URL)
		if err != nil {
			return nil, fmt.Errorf("probe %q URL: %w", probe.Name, err)
		}
		host := strings.TrimSuffix(u.Hostname(), ".tossinvest.com")
		switch host {
		case "wts-api", "wts-cert-api", "wts-info-api":
		default:
			return nil, fmt.Errorf("probe %q uses unsupported host %q", probe.Name, u.Hostname())
		}
		out = append(out, probeFact{
			Name: probe.Name, Method: probe.Method, Host: host, Path: u.Path,
		})
	}
	return out, nil
}
