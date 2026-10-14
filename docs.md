# KITE Documentation

## cmd/main.go Summary

`cmd/main.go` is the entry point for the KITE terminal assistant application.

### Key Responsibilities:
1. **Environment Configuration**: Loads environment variables from a local `.env` file (if present) using `godotenv.Load()`, without overwriting any already-set environment variables.
2. **TUI Initialization & Execution**: Initializes and runs the Bubble Tea terminal user interface (TUI) program via `tui.New()`. It configures the program with:
   - Alternate screen buffer (`tea.WithAltScreen()`)
   - Mouse cell motion support (`tea.WithMouseCellMotion()`)
3. **Error Handling**: Captures and reports any fatal errors encountered during the execution of the TUI program to standard error (`os.Stderr`) before exiting with a non-zero status code.
