package tools

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// listFilesDefinition tells the model it can see every file path in the workspace.
// It takes no arguments.
const listFilesDefinition = `{
  "type": "function",
  "function": {
    "name": "list_files",
    "description": "List every file in the current workspace, as paths relative to the workspace root.",
    "parameters": {
      "type": "object",
      "properties": {},
      "additionalProperties": false
    }
  }
}`

// listFiles returns every file under the folder Kite was started in, one
// relative path per line. Hidden folders such as .git are skipped.
func listFiles(arguments string) string {
	if strings.TrimSpace(arguments) != "" {
		var args map[string]any
		if err := json.Unmarshal([]byte(arguments), &args); err != nil || len(args) > 0 {
			return "error: list_files takes no arguments"
		}
	}

	root, err := os.Getwd()
	if err != nil {
		return "error: " + err.Error()
	}

	var paths []string
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip anything that cannot be read
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, err := filepath.Rel(root, path); err == nil {
			paths = append(paths, rel)
		}
		return nil
	})

	if len(paths) == 0 {
		return "the workspace has no files"
	}
	return strings.Join(paths, "\n")
}
