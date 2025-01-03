# Go Safe Enum Generator

A command-line tool that generates type-safe enum implementations in Go from special comments in source code.

## Features

- Generates type-safe enum implementations
- Case-insensitive string parsing
- Handles special characters in enum labels (auto-converts to valid Go identifiers)
- Implements common interfaces:
    - fmt.Stringer
    - json.Marshaler/Unmarshaler
    - sql.Scanner/driver.Valuer
    - encoding.TextMarshaler/TextUnmarshaler
    - Optional yaml.Marshaler/Unmarshaler
- Includes gorilla/schema converter
- Integer mapping support
- Maintains original package context
- Configurable output (stdout or file)

## Installation

```bash
go install github.com/panta/go-safe-enum-generator@latest
```

## Usage

The tool looks for special comments in your Go source files to generate enum implementations. The comment format is:

```go
// ENUM Name (value1, value2, ..., valueN)
```

### Command Line Options

```
Usage: go-safe-enum-generator -f <file> [-o output] [-y]

Flags:
  -f, --file string    Input file to process
  -o, --output string  Output file (defaults to stdout)
  -y, --yaml          Generate YAML marshaler/unmarshaler
```

### Example

Input file (`types.go`):

```go
package myapp

// ENUM AuthType (unknown, plain, login, digest-md5, cram-md5)
```

Generate the enum:

```bash
# Output to stdout
go-safe-enum-generator -f types.go

# Output to file
go-safe-enum-generator -f types.go -o auth_type.go

# Include YAML support
go-safe-enum-generator -f types.go -o auth_type.go -y
```

The tool will generate a complete enum implementation. Special characters in enum labels (like hyphens) are automatically handled to generate valid Go variable names while preserving the original values in the string representation:

```go
// Usage examples
authType := AuthTypeDigestMd5
str := authType.String()            // Returns "digest-md5" (original value)

// Parse from string
var auth AuthType
err := auth.Parse("digest-md5")     // Sets to AuthTypeDigestMd5

// Create from string
auth, err := AuthTypeFromString("cram-md5") // Returns AuthTypeCramMd5

// Create from integer
auth, err := AuthTypeFromInt(3)     // Returns AuthTypeDigestMd5

// Get all possible values
values := auth.Values()             // Returns slice of all enum values

// Database operations (implements sql.Scanner and driver.Valuer)
var auth AuthType
err := row.Scan(&auth)             // Scan from database
value, err := auth.Value()         // Convert to database value

// JSON operations
data, err := json.Marshal(auth)    // Convert to JSON
err := json.Unmarshal(data, &auth) // Parse from JSON

// YAML operations (if enabled)
data, err := yaml.Marshal(auth)    // Convert to YAML
err := yaml.Unmarshal(data, &auth) // Parse from YAML

// Text marshaling
data, err := auth.MarshalText()    // Convert to text
err := auth.UnmarshalText(data)    // Parse from text
```

## Generated Code Features

Each generated enum includes:

- Type-safe enum implementation with properly sanitized Go identifiers
- String conversion methods (preserving original values)
- Case-insensitive parsing
- Integer mapping support
- Default value handling
- Error handling for invalid values
- Database integration
- JSON/YAML serialization
- Text marshaling
- Values list accessor
- Gorilla schema support

## Special Characters Handling

The generator automatically converts special characters in enum labels to create valid Go identifiers:

- Hyphens, spaces, and underscores are turned into CamelCase
- Other non-alphanumeric characters are converted to underscores
- If a label starts with a number, an underscore is prepended
- The original value is preserved in the string representation

Example:
```go
// Input:
// ENUM Status (200-OK, 404-not-found, 500-error)

// Generated variables:
StatusOK_200         = Status{"200-OK"}
Status_404NotFound   = Status{"404-not-found"}
Status_500Error      = Status{"500-error"}
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)
