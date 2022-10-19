package main

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

const JS_EXT = ".js"
const HTML_EXT = ".html"
const DATA_MC_TRANSLATE = "data-mc-translate"
const MESSAGE_ID = "<Message id="
const LOGDIRECTORY = "dirwalker_logs"
const LOG_FILE_NAME = "dirwalker.log"
const NODE_MODULES_FOLDER = "node_modules"
const BUILD_FOLDER = "build"
const PUBLIC_FOLDER = "public"
const MAXBACKUPS = 10
const MAXSIZE = 10
const MAXAGE = 10
const TEST_FILE_STRING = "_spec"

const VERSION = "1.0.0"

var logger zerolog.Logger
var foundFiles = []string{}

type Model struct {
	textInput textinput.Model
	spinner   spinner.Model

	typing   bool
	loading  bool
	err      error
	location string
}

type Results struct {
	Err      error
	Location string
}

func generateWelcomeHeader() {
	pterm.DefaultCenter.WithCenterEachLineSeparately().Println("Strings!\n" + VERSION)
	// Generate BigLetters
	s, _ := pterm.DefaultBigText.WithLetters(putils.LettersFromString("Strings")).Srender()
	pterm.DefaultCenter.Println(s) // Print BigLetters with the default CenterPrinter

	pterm.DefaultCenter.WithCenterEachLineSeparately().Println("👋 Please garb the location where you find the strings.")
}

func (m Model) startWork(dirPath string) tea.Cmd {

	return func() tea.Msg {
		err := walkDir(dirPath)
		// loc, err := walkDir(context.Background(), dirPath)
		if err != nil {
			return Results{Err: err}
		}

		return Results{Location: dirPath}
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.typing {
				query := strings.TrimSpace(m.textInput.Value())
				if query != "" {
					m.typing = false
					m.loading = true
					return m, tea.Batch(
						spinner.Tick,
						m.startWork(query),
					)
				}
			}

		case "esc":
			if !m.typing && !m.loading {
				m.typing = true
				m.err = nil
				foundFiles = []string{} // clear our slice , reset
				return m, nil
			}
		}

	case Results:
		m.loading = false

		if err := msg.Err; err != nil {
			m.err = err
			return m, nil
		}

		m.location = msg.Location
		return m, nil
	}

	if m.typing {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.typing {
		return fmt.Sprintf("Enter Directory Path :\n%s", m.textInput.View())
	}

	if m.loading {
		return fmt.Sprintf("%s Please wait while the 🧝 sort ..", m.spinner.View())
	}

	if err := m.err; err != nil {
		return fmt.Sprintf("An error was encountered: %v", err)
	}

	return fmt.Sprintf(strconv.FormatInt(int64(len(foundFiles)), 10) + " files found with translation content.\nPlease check the log file for more details.\nPress CTRL+C to exit.\nPress ESC to start again.\n")
}

func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	currentWorkingDirectory, _ := os.Getwd()
	loggerPath := path.Join(currentWorkingDirectory, LOGDIRECTORY)

	customLogger := lumberjack.Logger{
		Filename:   path.Join(loggerPath, LOG_FILE_NAME),
		MaxBackups: MAXBACKUPS, // files
		MaxSize:    MAXSIZE,    // megabytes
		MaxAge:     MAXAGE,     // days
	}
	logger = zerolog.New(&customLogger).With().Timestamp().Logger()
	logger.Info().Msg("👋 Welcome ")
}

func readFile(filePath string, fileName string) error {
	file, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error().Msg(string(err.Error()))
		return fmt.Errorf("error reading file %s", filePath)
	}
	contents := string(file)
	if strings.Contains(contents, DATA_MC_TRANSLATE) || strings.Contains(contents, MESSAGE_ID) {
		logger.Info().Msg("Matched entry in file → " + filePath)
		foundFiles = append(foundFiles, fileName)
	}
	return nil
}

func walkDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Error().Msg(string(err.Error()))
		return fmt.Errorf("error reading directory: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() == NODE_MODULES_FOLDER || entry.Name() == BUILD_FOLDER || entry.Name() == PUBLIC_FOLDER {
			logger.Log().Msg("❌ Skipping folder: " + entry.Name())
			continue
		}
		// log.Println("Current Entry : " + entry.Name())
		if entry.IsDir() {
			subdir := path.Join(dir, entry.Name())
			walkDir(subdir)
		} else {
			filePath := path.Join(dir, entry.Name())
			fileExtension := path.Ext(filePath)
			// we only look at the files where the content is supposed to be translated
			// for angularjs code we are looking at .HTML files and for react components we are looking at .JS files for the content
			// test files are also .JS files, but they have _spec in their names, which is why we are not considering them at this point in time.
			if (fileExtension == JS_EXT || fileExtension == HTML_EXT) && !strings.Contains(filePath, TEST_FILE_STRING) {
				// log.Println("Reading file → " + filePath)
				err := readFile(filePath, entry.Name())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil

}

func main() {
	setupLogger()
	generateWelcomeHeader()

	t := textinput.NewModel()
	t.Focus()

	s := spinner.NewModel()
	s.Spinner = spinner.Dot

	initialModel := Model{
		textInput: t,
		spinner:   s,
		typing:    true,
	}
	err := tea.NewProgram(initialModel).Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
