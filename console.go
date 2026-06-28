package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"
)

// ANSI escape codes. We use a global useColor flag so all helpers become no-ops
// when the terminal doesn't support color (pipes, redirected output, old Windows).
var useColor bool

func init() {
	useColor = stdoutIsTerminal()
	// On Windows 10+ we need to enable VT processing explicitly.
	if useColor && runtime.GOOS == "windows" {
		enableWindowsANSI()
	}
}

// stdoutIsTerminal checks whether stdout is a real interactive terminal.
func stdoutIsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Color codes
const (
	cReset   = "\033[0m"
	cBold    = "\033[1m"
	cDim     = "\033[2m"
	cCyan    = "\033[36m"
	cGreen   = "\033[32m"
	cYellow  = "\033[33m"
	cRed     = "\033[31m"
	cGray    = "\033[90m"
	cBgCyan  = "\033[46m"
	cBgGreen = "\033[42m"
)

// wrap wraps text in an ANSI code if color is enabled.
func wrap(code, text string) string {
	if !useColor {
		return text
	}
	return code + text + cReset
}

// --- Formatted output helpers ---

// printBanner prints the SatisFacts ASCII banner in cyan.
func printBanner() {
	banner := []string{
		"╔══════════════════════════════════════════════╗",
		"║            SatisFacts  by Physox             ║",
		"║      Satisfactory Save File Analyzer         ║",
		"╚══════════════════════════════════════════════╝",
	}
	for _, line := range banner {
		fmt.Println(wrap(cCyan, line))
	}
	fmt.Println()
}

// printSectionHeader prints a ┌─ Section ─── header line in cyan.
func printSectionHeader(title string) {
	prefix := "┌─ " + title + " "
	// Use rune count, not byte length — box-drawing chars are multi-byte UTF-8.
	// Footer is └ + 45×─ + ┘ = 47 display chars. Header must match.
	padding := 46 - utf8.RuneCountInString(prefix)
	if padding < 0 {
		padding = 0
	}
	fmt.Println(wrap(cCyan, prefix+strings.Repeat("─", padding)+"┐"))
}

// printSectionFooter prints the └─────┘ closing line in cyan.
func printSectionFooter() {
	fmt.Println(wrap(cCyan, "└─────────────────────────────────────────────┘"))
}

// printDone prints a green ✓ line: "│ ✓ message"
func printDone(msg string) {
	fmt.Printf("│ %s %s\n", wrap(cGreen, "✓"), msg)
}

// printDonef is the formatted version of printDone.
func printDonef(format string, args ...interface{}) {
	fmt.Printf("│ %s %s\n", wrap(cGreen, "✓"), fmt.Sprintf(format, args...))
}

// printStep prints a neutral step line: "│ message"
func printStep(msg string) {
	fmt.Printf("│ %s\n", msg)
}

// printStepf is the formatted version of printStep.
func printStepf(format string, args ...interface{}) {
	fmt.Printf("│ %s\n", fmt.Sprintf(format, args...))
}

// printDetail prints a dimmed detail line: "│   message"
func printDetail(msg string) {
	fmt.Printf("│   %s\n", wrap(cDim, msg))
}

// printDetailf is the formatted version of printDetail.
func printDetailf(format string, args ...interface{}) {
	fmt.Printf("│   %s\n", wrap(cDim, fmt.Sprintf(format, args...)))
}

// printWarn prints a yellow warning line.
func printWarn(msg string) {
	fmt.Printf("│ %s %s\n", wrap(cYellow, "⚠"), msg)
}

// printWarnf is the formatted version of printWarn.
func printWarnf(format string, args ...interface{}) {
	fmt.Printf("│ %s %s\n", wrap(cYellow, "⚠"), fmt.Sprintf(format, args...))
}

// printError prints a red error message.
func printError(msg string) {
	fmt.Printf("\n%s %s\n", wrap(cRed, "✗"), wrap(cRed, msg))
}

// printErrorf is the formatted version of printError.
func printErrorf(format string, args ...interface{}) {
	printError(fmt.Sprintf(format, args...))
}

// printSummaryLine prints a summary row with a dimmed label and bold value.
// Example: "│ Total objects:     56,532"
func printSummaryLine(label string, value string) {
	fmt.Printf("│ %-20s %s\n", label, wrap(cBold, value))
}

// printSummaryLinef is the formatted version of printSummaryLine.
func printSummaryLinef(label string, format string, args ...interface{}) {
	printSummaryLine(label, fmt.Sprintf(format, args...))
}

// printSubHeader prints a sub-section header inside a section (e.g. "── Collectibles ──").
func printSubHeader(title string) {
	fmt.Printf("│ %s\n", wrap(cYellow, "── "+title+" ──"))
}

// printKeyValue prints a key-value pair in the interactive menus.
func printMenuItem(num int, label string, desc string) {
	if desc != "" {
		fmt.Printf("│  %s. %-8s %s\n",
			wrap(cBold, fmt.Sprintf("%d", num)),
			label,
			wrap(cDim, desc))
	} else {
		fmt.Printf("│  %s. %s\n", wrap(cBold, fmt.Sprintf("%d", num)), label)
	}
}

// printRecommended prints a menu item with a green "(recommended)" tag.
func printRecommended(num int, label string, desc string) {
	fmt.Printf("│  %s. %-8s %s %s\n",
		wrap(cBold, fmt.Sprintf("%d", num)),
		label,
		desc,
		wrap(cGreen, "← recommended"))
}
