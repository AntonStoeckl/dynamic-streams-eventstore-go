package main

import (
	"os"
	"strings"
)

// ANSI color codes for terminal output.
const (
	// Basic colors.
	ColorReset = "\033[0m"
	ColorBold  = "\033[1m"
	ColorDim   = "\033[2m"

	// Text colors.
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorGray    = "\033[90m"

	// Bright colors.
	ColorBrightRed     = "\033[91m"
	ColorBrightGreen   = "\033[92m"
	ColorBrightYellow  = "\033[93m"
	ColorBrightBlue    = "\033[94m"
	ColorBrightMagenta = "\033[95m"
	ColorBrightCyan    = "\033[96m"
	ColorBrightWhite   = "\033[97m"

	// Background colors.
	ColorBgRed    = "\033[41m"
	ColorBgGreen  = "\033[42m"
	ColorBgYellow = "\033[43m"
	ColorBgBlue   = "\033[44m"
)

// ColorSupported checks if the terminal supports colors.
func ColorSupported() bool {
	term := os.Getenv("TERM")
	if term == "" {
		return false
	}

	// Check for common color-supporting terminals
	colorTerms := []string{"xterm", "screen", "tmux", "color", "ansi"}
	for _, colorTerm := range colorTerms {
		if strings.Contains(strings.ToLower(term), colorTerm) {
			return true
		}
	}

	// Check COLORTERM environment variable
	return os.Getenv("COLORTERM") != ""
}

// Colorize wraps text with color codes if colors are supported.
func Colorize(text, color string) string {
	if !ColorSupported() {
		return text
	}
	return color + text + ColorReset
}

// Predefined color functions for common use cases.
func Red(text string) string     { return Colorize(text, ColorRed) }
func Green(text string) string   { return Colorize(text, ColorGreen) }
func Yellow(text string) string  { return Colorize(text, ColorYellow) }
func Blue(text string) string    { return Colorize(text, ColorBlue) }
func Magenta(text string) string { return Colorize(text, ColorMagenta) }
func Cyan(text string) string    { return Colorize(text, ColorCyan) }
func Gray(text string) string    { return Colorize(text, ColorGray) }
func Bold(text string) string    { return Colorize(text, ColorBold) }
func Dim(text string) string     { return Colorize(text, ColorDim) }

// Bright versions.
func BrightRed(text string) string     { return Colorize(text, ColorBrightRed) }
func BrightGreen(text string) string   { return Colorize(text, ColorBrightGreen) }
func BrightYellow(text string) string  { return Colorize(text, ColorBrightYellow) }
func BrightBlue(text string) string    { return Colorize(text, ColorBrightBlue) }
func BrightMagenta(text string) string { return Colorize(text, ColorBrightMagenta) }
func BrightCyan(text string) string    { return Colorize(text, ColorBrightCyan) }

// Status-specific colors.
func Success(text string) string { return BrightGreen(text) }
func Error(text string) string   { return BrightRed(text) }
func Warning(text string) string { return BrightYellow(text) }
func Info(text string) string    { return BrightCyan(text) }
func Debug(text string) string   { return Gray(text) }

// Header creates a colored header with optional emoji.
func Header(text string) string {
	return Bold(BrightCyan(text))
}

// Separator creates a colored separator line.
func Separator(char string, length int) string {
	return Gray(strings.Repeat(char, length))
}

// StatusIcon returns a colored status icon.
func StatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "success", "ok", "done", "complete":
		return Success("✅")
	case "error", "fail", "failed":
		return Error("❌")
	case "warning", "warn":
		return Warning("⚠️")
	case "info", "information":
		return Info("ℹ️")
	case "debug":
		return Debug("🔍")
	case "progress", "working":
		return Yellow("🔄")
	case "stats", "metrics":
		return Blue("📊")
	case "books":
		return Cyan("📚")
	case "readers":
		return Magenta("👥")
	case "cleanup":
		return BrightYellow("🧹")
	case "ghost":
		return Magenta("👻")
	case "orphaned":
		return Yellow("❌")
	default:
		return status
	}
}
