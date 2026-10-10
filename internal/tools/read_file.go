package tools

import (
	"encoding/json"
	"os"
	"strings"
)

// readFileDefinition tells the model it can read one file by its path.
const readFileDefinition = `{
  "type": "function",
  "function": {
    "name": "read_file",
    "description": "Read a file inside the current workspace.",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path of the file to read"
        }
      },
      "required": ["path"],
      "additionalProperties": false
    }
  }
}`

// readFile returns the contents of one file inside the workspace.
// The path must be given, stay inside the folder Kite was started in, and be a file.
func readFile(arguments string) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil || strings.TrimSpace(args.Path) == "" {
		return `error: read_file needs a path, for example {"path":"main.go"}`
	}

	full, ok := insideWorkspace(args.Path)
	if !ok {
		return "error: " + args.Path + " is outside the workspace"
	}

	info, err := os.Stat(full)
	if err != nil {
		return "error: " + args.Path + " does not exist"
	}
	if info.IsDir() {
		return "error: " + args.Path + " is a folder, not a file"
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(data)
}
