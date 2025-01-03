package main

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/alecthomas/kong"
)

var CLI struct {
	File   string `help:"Input file to process" short:"f" required:""`
	Output string `help:"Output file (defaults to stdout)" short:"o"`
	YAML   bool   `help:"Generate YAML marshaler/unmarshaler" short:"y"`
}

type valueInfo struct {
	Original string
	GoName   string
}

type enumDef struct {
	Package string
	Name    string
	Values  []valueInfo
	YAML    bool
}

func main() {
	ctx := kong.Parse(&CLI)
	if err := processFile(CLI.File, CLI.Output, CLI.YAML); err != nil {
		ctx.FatalIfErrorf(err)
	}
}

func getPackageName(filename string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("parsing package clause: %w", err)
	}
	return f.Name.Name, nil
}

func sanitizeGoName(s string) string {
	// first, handle hyphen, underscore and spaces separated words specially
	words := regexp.MustCompile(`[-_ ]`).Split(s, -1)

	// title case all words except the first one (which will get title case from the enum name prefix)
	for i := 0; i < len(words); i++ {
		// Sanitize each word (remove any other special characters)
		words[i] = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(words[i], "")
		if i > 0 && len(words[i]) > 0 {
			words[i] = strings.Title(strings.ToLower(words[i]))
		}
	}

	safe := strings.Join(words, "")

	// if result is empty or starts with a number, prefix with underscore
	if safe == "" || (len(safe) > 0 && safe[0] >= '0' && safe[0] <= '9') {
		safe = "_" + safe
	}

	return safe
}

func processFile(filename, output string, yaml bool) error {
	pkgName, err := getPackageName(filename)
	if err != nil {
		return fmt.Errorf("getting package name: %w", err)
	}

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var out io.Writer
	if output == "" {
		out = os.Stdout
	} else {
		f, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	scanner := bufio.NewScanner(file)
	enumRegex := regexp.MustCompile(`^\s*//\s*ENUM\s+(\w+)\s*\((.*?)\)`)

	// Write package declaration and imports
	imports := []string{
		"database/sql/driver",
		"encoding/json",
		"fmt",
		"reflect",
		"strings",
	}
	if yaml {
		imports = append(imports, "gopkg.in/yaml.v3")
	}

	fmt.Fprintf(out, "package %s\n\n", pkgName)
	fmt.Fprintln(out, "import (")
	for _, imp := range imports {
		fmt.Fprintf(out, "\t%q\n", imp)
	}
	fmt.Fprintln(out, ")")
	fmt.Fprintln(out)

	foundEnum := false
	for scanner.Scan() {
		line := scanner.Text()
		if matches := enumRegex.FindStringSubmatch(line); matches != nil {
			values := make([]valueInfo, 0)
			for _, v := range strings.Split(matches[2], ",") {
				v = strings.TrimSpace(v)
				if v != "" {
					values = append(values, valueInfo{
						Original: v,
						GoName:   sanitizeGoName(v),
					})
				}
			}

			enum := enumDef{
				Package: pkgName,
				Name:    matches[1],
				Values:  values,
				YAML:    yaml,
			}
			if err := generateEnum(out, enum); err != nil {
				return fmt.Errorf("generating enum %s: %w", enum.Name, err)
			}
			foundEnum = true
		}
	}

	if !foundEnum {
		return fmt.Errorf("no enum definitions found in %s", filename)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning file: %w", err)
	}
	return nil
}

func generateEnum(w io.Writer, enum enumDef) error {
	funcMap := template.FuncMap{
		"title": strings.Title,
		"lower": strings.ToLower,
		"goName": func(v valueInfo) string {
			return v.GoName
		},
		"original": func(v valueInfo) string {
			return v.Original
		},
	}

	const enumTemplate = `
// {{ .Name }} is an enum.
// Possible values: {{ range $i, $v := .Values }}{{if $i}}, {{end}}{{ original $v }}{{end}}
// see https://threedots.tech/post/safer-enums-in-go/
type {{ .Name }} struct {
	slug string
}

// String returns the string representation of a {{ .Name }} enum.
func (e {{ .Name }}) String() string {
	return e.slug
}

// Parse sets the enum value from a string.
func (e *{{ .Name }}) Parse(s string) error {
	s = strings.TrimSpace(s)
	switch {
	{{- range .Values }}
	case strings.EqualFold(s, {{ $.Name }}{{ goName . | title }}.slug):
		e.slug = {{ $.Name }}{{ goName . | title }}.slug
		return nil
	{{- end }}
	}

	*e = {{ $.Name }}{{ goName (index .Values 0) | title }}
	return fmt.Errorf("unknown {{ .Name | lower }}: %s", s)
}

// {{ .Name }}FromString returns a {{ .Name }} from a string.
func {{ .Name }}FromString(s string) ({{ .Name }}, error) {
	e := {{ .Name }}{}
	err := e.Parse(s)
	return e, err
}

// {{ .Name }}FromInt returns a {{ .Name }} from a numeric value.
func {{ .Name }}FromInt(value int) ({{ .Name }}, error) {
	if v, ok := {{ .Name | lower }}IntMap[value]; ok {
		return v, nil
	}
	return {{ .Name }}{}, fmt.Errorf("can't convert the value %d to a {{ .Name }}", value)
}

// {{ .Name }}SchemaConverter is for gorilla/schema (must be registered with decoder.RegisterConverter).
func {{ .Name }}SchemaConverter(value string) reflect.Value {
	var e {{ .Name }}
	if err := e.Parse(value); err != nil {
		return reflect.ValueOf(nil)
	}
	return reflect.ValueOf(e)
}

// Value implements the driver.Valuer interface for database serialization.
func (e {{ .Name }}) Value() (driver.Value, error) {
	return e.slug, nil
}

// Scan implements the sql.Scanner interface for database deserialization.
func (e *{{ .Name }}) Scan(value interface{}) error {
	if value == nil {
		e.slug = {{ $.Name }}{{ goName (index .Values 0) | title }}.slug
		return nil
	}

	switch v := value.(type) {
	default:
		return fmt.Errorf("can't convert to {{ .Name }}, unexpected type %T", v)
	case int:
		if found, ok := {{ $.Name | lower }}IntMap[v]; ok {
			e.slug = found.slug
		} else {
			return fmt.Errorf("invalid value %d for {{ .Name }}", v)
		}
	case float64:
		if found, ok := {{ $.Name | lower }}IntMap[int(v)]; ok {
			e.slug = found.slug
		} else {
			return fmt.Errorf("invalid value %f for {{ .Name }}", v)
		}
	case []byte:
		if err := e.Parse(string(v)); err != nil {
			return fmt.Errorf("can't parse {{ .Name }}: %w", err)
		}
		return nil
	case *string:
		if err := e.Parse(*v); err != nil {
			return fmt.Errorf("can't parse {{ .Name }}: %w", err)
		}
		return nil
	case string:
		if err := e.Parse(v); err != nil {
			return fmt.Errorf("can't parse {{ .Name }}: %w", err)
		}
		return nil
	}
	return fmt.Errorf("can't convert to {{ .Name }}, unexpected type %T", value)
}
{{ if .YAML }}
// MarshalYAML implements the yaml.Marshaler interface.
func (e {{ .Name }}) MarshalYAML() (interface{}, error) {
	return e.slug, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (e *{{ .Name }}) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return fmt.Errorf("can't unmarshal nil YAML into {{ .Name }}")
	}
	var text string
	if err := value.Decode(&text); err != nil {
		return err
	}
	if err := e.Parse(text); err != nil {
		return err
	}
	return nil
}
{{ end }}
// MarshalJSON implements the json.Marshaler interface.
func (e {{ .Name }}) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.slug)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (e *{{ .Name }}) UnmarshalJSON(data []byte) error {
	if data == nil {
		return fmt.Errorf("can't unmarshal nil JSON into {{ .Name }}")
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if err := e.Parse(text); err != nil {
		return err
	}
	return nil
}

// MarshalText implements the text marshaller method.
func (e {{ .Name }}) MarshalText() ([]byte, error) {
	return []byte(e.slug), nil
}

// UnmarshalText implements the text unmarshaller method.
func (e *{{ .Name }}) UnmarshalText(data []byte) error {
	if data == nil {
		return fmt.Errorf("can't unmarshal empty text into {{ .Name }}")
	}
	if err := e.Parse(string(data)); err != nil {
		return err
	}
	return nil
}

// Values returns the list of possible values for the enum.
func (e *{{ .Name }}) Values() []{{ .Name }} {
	return append([]{{ .Name }}{}, {{ .Name | lower }}Values...)
}

var (
	{{ .Name | lower }}Values   = []{{ .Name }}{{"{"}}{{ range $i, $v := .Values }}{{if $i}}, {{end}}{{ $.Name }}{{ goName $v | title }}{{end}}{{"}"}}
	{{- range $i, $v := .Values }}
	{{ $.Name }}{{ goName $v | title }} = {{ $.Name }}{"{{ original $v }}"}
	{{- end }}
	{{ .Name | lower }}IntMap   = map[int]{{ .Name }}{
		{{- range $i, $v := .Values }}
		{{ $i }}: {{ $.Name }}{{ goName $v | title }},
		{{- end }}
	}
)
`

	tmpl, err := template.New("enum").Funcs(funcMap).Parse(enumTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	if err := tmpl.Execute(w, enum); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	return nil
}
