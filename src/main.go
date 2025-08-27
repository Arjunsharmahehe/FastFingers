package main

import (
	"crypto/rand"
	"encoding/csv"
	"flag"
	"fmt"
	"math/big"
	"os"
	"time"
	"unicode"

	"github.com/nsf/termbox-go"
)

var paragraphIdx int
var testText string

// A struct to hold the state of each character in the test.
type charState struct {
	char     rune
	correct  bool
	typed    bool
	isCursor bool
}

// Global state for the application
var (
	startTime     time.Time
	testStarted   bool
	testCompleted bool
	states        []charState
	typedChars    int
	errors        int
	cursorPos     int
	elapsedTime   time.Duration
)

func writeToCSV(filename string, wpm float64, accuracy float64) error {

	// Check if file exists (does it need a new header)
	fileExists := true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fileExists = false
	}

	// Open file for writing (create if doesn't exist, append if it does)
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644) // 0644 means read/write for owner, read for group/others
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header if file is new
	if !fileExists {
		header := []string{"WPM", "Accuracy"}
		if err := writer.Write(header); err != nil {
			return err
		}
	}

	// Write the current record
	record := []string{
		fmt.Sprintf("%.0f", wpm),
		fmt.Sprintf("%.1f", accuracy),
	}

	return writer.Write(record)
}

// setup initial state of the application
func setup(pythonFlag *bool) {
	// Re-select a random paragraph for restart
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(paragraphs))))
	if err != nil {
		panic("Error selecting a test paragraph")
	}
	paragraphIdx = int(n.Int64())
	if *pythonFlag {
		testText = pythonParagraph[paragraphIdx]
	} else {
		testText = paragraphs[paragraphIdx]
	}

	// Initialize the states slice based on the testText
	states = make([]charState, len(testText))
	for i, char := range testText {
		states[i] = charState{char: char}
	}
	// Set the initial cursor position
	if len(states) > 0 {
		states[0].isCursor = true
	}
	cursorPos = 0
	typedChars = 0
	errors = 0
	testStarted = false
	testCompleted = false
	elapsedTime = 0
}

// redrawScreen clears the terminal and redraws the entire UI.
func redrawScreen(timeLimit *int) {

	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	width, height := termbox.Size()

	// Draw the stats at the bottom
	statsY := 0
	// if statsY < y+2 { // Ensure stats don't overlap with text
	// 	statsY = y + 2
	// }

	var wpm float64
	var accuracy float64

	if testStarted {
		elapsedTime = time.Since(startTime)
		elapsedMinutes := elapsedTime.Minutes()
		if elapsedMinutes > 0 {
			// WPM = (all typed characters / 5) / time in minutes
			if elapsedTime.Seconds() > float64(*timeLimit) {
				wpm = (float64(typedChars-errors) / 5.0) / (float64(*timeLimit) / 60.0)
			} else {
				wpm = (float64(typedChars-errors) / 5.0) / elapsedMinutes
			}
		}
		if typedChars > 0 {
			accuracy = (float64(typedChars-errors) / float64(typedChars)) * 100
		}
	}

	statsTime := fmt.Sprintf("%.1fs", min(elapsedTime.Seconds(), float64(*timeLimit)))
	statsWPM := fmt.Sprintf("%.0f WPM", wpm)
	statsAccuracy := fmt.Sprintf("%.1f%% acc", accuracy)
	statsX := (width - len(statsTime) - len(statsWPM) - len(statsAccuracy) - 6) / 2

	for i, char := range statsTime {
		termbox.SetCell(i+statsX, statsY, char, termbox.ColorBlue|termbox.AttrBold, termbox.ColorDefault)
	}
	for i, char := range " | " {
		termbox.SetCell(i+statsX+len(statsTime), statsY, char, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, char := range statsWPM {
		termbox.SetCell(i+statsX+len(statsTime)+len(" | "), statsY, char, termbox.ColorLightYellow|termbox.AttrBold, termbox.ColorDefault)
	}
	for i, char := range " | " {
		termbox.SetCell(i+statsX+len(statsTime)+len(statsWPM)+len(" | "), statsY, char, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, char := range statsAccuracy {
		termbox.SetCell(i+statsX+len(statsTime)+len(statsWPM)+len(" | ")+len(" | "), statsY, char, termbox.ColorLightGreen|termbox.AttrBold, termbox.ColorDefault)
	}

	// Draw the typing text
	x, y := 4, 2
	for _, s := range states {
		fg := termbox.ColorDarkGray
		bg := termbox.ColorDefault

		if s.isCursor {
			// Highlight the cursor position
			bg = termbox.ColorWhite
			fg = termbox.ColorBlack
		} else if s.typed {
			if s.correct {
				fg = termbox.ColorGreen
			} else {
				fg = termbox.ColorRed
			}
		}

		// Handle word wrapping (manual text wrapping go burrr)
		if x >= width-4 {
			x = 4
			y++
		}
		termbox.SetCell(x, y, s.char, fg, bg)

		// Set the physical cursor position to follow the character cursor
		if s.isCursor {
			termbox.SetCursor(x, y)
		}

		x++
	}

	// Draw instructions if the test hasn't started
	if !testStarted {
		instructions := "Start typing to begin the test."
		startX := (width - len(instructions)) / 2
		for i, char := range instructions {
			termbox.SetCell(startX+i, height/2, char, termbox.ColorYellow, termbox.ColorDefault)
		}
	}

	//Controls guide
	controlY := height - 2
	if (controlY) <= y+2 {
		// Ensure controls don't overlap with text
		controlY = y + 2
	}

	controls := "-- press ESC to exit --"
	controlsX := (width - len(controls)) / 2
	for i, char := range controls {
		termbox.SetCell(i+controlsX, controlY, char, termbox.ColorDarkGray, termbox.ColorDefault)
	}

	// Draw final results if the test is complete
	resultY := ((height - 4) / 2) + 2
	if resultY <= y {
		resultY = y + 2
	}
	if cursorPos >= len(testText) || elapsedTime.Seconds() >= float64(*timeLimit) {
		resultLine1 := "Test Complete!"
		resultLine2 := fmt.Sprintf("Final WPM: %.0f | Final Accuracy: %.1f%%", wpm, accuracy)
		resultLine3 := "Press ESC to exit or R to restart."

		startX1 := (width - len(resultLine1)) / 2
		startX2 := (width - len(resultLine2)) / 2
		startX3 := (width - len(resultLine3)) / 2

		for i, char := range resultLine1 {
			termbox.SetCell(startX1+i, resultY, char, termbox.ColorCyan|termbox.AttrBold, termbox.ColorDefault)
		}
		for i, char := range resultLine2 {
			termbox.SetCell(startX2+i, resultY+1, char, termbox.ColorCyan, termbox.ColorDefault)
		}
		for i, char := range resultLine3 {
			termbox.SetCell(startX3+i, resultY+2, char, termbox.ColorDarkGray, termbox.ColorDefault)
		}

		// Only write to CSV once when test completes
		if !testCompleted {
			writeToCSV("results.csv", wpm, accuracy)
			testCompleted = true
		}
	}

	// Flush the buffer to the screen (renders it on the terminal)
	termbox.Flush()
}

func main() {

	var (
		pythonFlag = flag.Bool("py", false, "Test on Python code like text")
		timeLimit  = flag.Int("time", 60, "Time limit for the test in seconds")
	)

	flag.Parse()
	// Initialize termbox
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	// Ensure termbox is closed properly on exit
	defer termbox.Close()

	setup(pythonFlag)

	// Main event loop
	for {

		redrawScreen(timeLimit)

		// Wait for an event
		ev := termbox.PollEvent()

		if ev.Type == termbox.EventKey {
			// Handle Quitting
			if ev.Key == termbox.KeyEsc || ev.Key == termbox.KeyCtrlC {
				return // Exit the loop and the program
			}

			// Handle Restarting
			if (ev.Ch == 'r' || ev.Ch == 'R') && (cursorPos >= len(testText) || elapsedTime.Seconds() >= float64(*timeLimit)) {
				setup(pythonFlag)
				continue
			}

			// If test is finished, ignore other key presses
			if cursorPos >= len(testText) || elapsedTime.Seconds() >= float64(*timeLimit) {
				continue
			}

			// Handle Backspace
			if (ev.Key == termbox.KeyBackspace || ev.Key == termbox.KeyBackspace2) && elapsedTime.Seconds() < float64(*timeLimit) {
				if cursorPos > 0 {
					// Move cursor back
					states[cursorPos].isCursor = false
					cursorPos--
					states[cursorPos].isCursor = true

					// Reset the state of the character being erased
					if states[cursorPos].typed {
						if !states[cursorPos].correct {
							errors--
						}
						typedChars--
						states[cursorPos].typed = false
						states[cursorPos].correct = false
					}
				}
			} else if (ev.Ch != 0 || ev.Key == termbox.KeySpace) && elapsedTime.Seconds() < float64(*timeLimit) {
				// Handle Character Input

				// Start the timer on the first keypress
				if !testStarted {
					startTime = time.Now()
					testStarted = true
				}

				// Use a space character if the spacebar was pressed
				typedChar := ev.Ch
				if ev.Key == termbox.KeySpace {
					typedChar = ' '
				}

				// Only process printable characters
				if unicode.IsPrint(typedChar) {
					states[cursorPos].typed = true
					typedChars++

					if typedChar == states[cursorPos].char {
						states[cursorPos].correct = true
					} else {
						states[cursorPos].correct = false
						errors++
					}

					// Move cursor forward
					states[cursorPos].isCursor = false
					cursorPos++
					if cursorPos < len(testText) {
						states[cursorPos].isCursor = true
					}
				}
			}
		}
	}
}
