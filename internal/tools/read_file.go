package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// readFileDefinition tells the model it can read one file by its path, a range
// of lines at a time.
const readFileDefinition = `{
  "type": "function",
  "function": {
    "name": "read_file",
    "description": "Read a text file inside the current workspace, a range of lines at a time. Only text files can be read: not images, PDFs, archives, or compiled binaries, so do not call it for those. The result starts with which lines were returned and how many lines the file has. If fewer lines came back than asked for, request the next range.",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path of the file to read"
        },
        "start_line": {
          "type": "integer",
          "description": "First line to read, counting from 1. Defaults to 1."
        },
        "end_line": {
          "type": "integer",
          "description": "Last line to read. Defaults to start_line + 199."
        }
      },
      "required": ["path"],
      "additionalProperties": false
    }
  }
}`

// defaultLines is how many lines are read when the model gives no end_line.
const defaultLines = 200

// headerRoom is kept free under the output limit for the "lines a-b of n" line.
const headerRoom = 200

// readFile returns a range of lines from one file inside the workspace.
// The path must be given, stay inside the folder Kite was started in, and be a file.
// The result never goes over the output limit: if the range does not fit, it
// stops early and says which lines were actually returned.
func readFile(arguments string) string {
	var args struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil || strings.TrimSpace(args.Path) == "" {
		return `error: read_file needs a path, for example {"path":"main.go"}`
	}

	full, ok := insideWorkspace(args.Path)
	if !ok {
		return "error: " + args.Path + " is outside the workspace"
	}
	if isBlocked(full) {
		return "error: " + args.Path + " is blocked, read_file cannot open secret files"
	}

	info, err := os.Stat(full)
	if err != nil {
		return "error: " + args.Path + " does not exist"
	}
	if info.IsDir() {
		return "error: " + args.Path + " is a folder, not a file"
	}

	if binaryExtensions[strings.ToLower(filepath.Ext(full))] {
		return "error: " + args.Path + " is a binary file, only text files can be read"
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return "error: " + err.Error()
	}
	if isBinary(data) {
		return "error: " + args.Path + " is a binary file, only text files can be read"
	}

	lines := strings.Split(string(data), "\n")
	// A file ending in a newline would otherwise count one extra, empty line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := len(lines)
	if total == 0 {
		return "lines 0-0 of 0: the file is empty"
	}

	start := args.StartLine
	if start == 0 {
		start = 1
	}
	end := args.EndLine
	if end == 0 {
		end = start + defaultLines - 1
	}
	if start < 1 || end < start {
		return "error: start_line must be 1 or more, and end_line must not be before start_line"
	}
	if start > total {
		return fmt.Sprintf("error: start_line %d is past the end, the file has %d lines", start, total)
	}
	end = min(end, total)

	// Add whole lines until the next one would cross the limit.
	room := maxOutputBytes() - headerRoom
	var body strings.Builder
	last := start - 1
	for i := start; i <= end; i++ {
		line := lines[i-1] + "\n"
		if body.Len()+len(line) > room {
			break
		}
		body.WriteString(line)
		last = i
	}
	if last < start {
		return fmt.Sprintf("error: line %d alone is longer than the %d byte output limit", start, maxOutputBytes())
	}

	header := fmt.Sprintf("lines %d-%d of %d", start, last, total)
	if last < end {
		header += fmt.Sprintf(" (only these fit in the output limit, ask from start_line %d for more)", last+1)
	}
	return header + "\n" + body.String()
}

// binaryExtensions are rejected by name, before the file is even opened.
var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true, ".bmp": true,
	".pdf": true,
	".zip": true, ".gz": true, ".tar": true, ".tgz": true, ".7z": true, ".rar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".o": true, ".a": true, ".bin": true, ".wasm": true,
	".mp3": true, ".mp4": true, ".mov": true, ".wav": true,
	".ttf": true, ".woff": true, ".woff2": true,
	".db": true, ".sqlite": true,
}

// isBinary reports whether data is not text. Text files almost never contain a
// zero byte, and they are valid UTF-8, so failing either check means binary.
// This catches binaries whose names say nothing, like a compiled program.
// isBlocked reports whether path is a file read_file must not open: .env and
// .env.*, *.pem, and anything under .git. A symlink is checked by its target too.
func isBlocked(path string) bool {
	if blockedName(path) {
		return true
	}
	if target, err := filepath.EvalSymlinks(path); err == nil && target != path {
		return blockedName(target)
	}
	return false
}

func blockedName(path string) bool {
	base := filepath.Base(path)
	if base == ".env" || strings.HasPrefix(base, ".env.") || strings.EqualFold(filepath.Ext(base), ".pem") {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}

func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data)
}

// maxOutputBytes reads MAX_TOOL_OUTPUT_BYTES. A missing or unreadable value
// falls back to 32768.
func maxOutputBytes() int {
	if n, err := strconv.Atoi(os.Getenv("MAX_TOOL_OUTPUT_BYTES")); err == nil && n > headerRoom {
		return n
	}
	return 32768
}
