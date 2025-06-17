package gologger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// used to reset the terminal color
const resetColor = "\033[0m"
const fileNameTimeFormat = "2006-01-02"
const libTag = "GOLOGGER"

type LevelColors struct {
	Log   string
	Warn  string
	Info  string
	Error string
	Debug string
	Panic string
	Trace string
	Fatal string
}

type MessageOptions struct {
	DisableSpacing         bool   `desc:"Disable spacing between log fields"`
	ContentDelimiter       string `desc:"Character used to separate log fields"`
	TextFiller             string `desc:"Character used to fill the prealocated space for message and tags if DisableSpacing is false "`
	MessagePrealocatedSize int    `desc:"Minimum space reserved for the message"`
	TagPrealocatedSize     int    `desc:"Minimum space reserved for the tag/subtags"`
	DateFormat             string `desc:"Date format"`
	UseJsonOutput          bool   `desc:"If you want to use the json output format, overwrite other options"`
}

type AppLoggerOptions struct {
	LogFileRotateDays int             `desc:"Total of days a log file is kept"`
	FileLogDisabled   bool            `desc:"Disable file logging"`
	DisabledTags      []string        `desc:"Used to prevent logs from being printed"`
	LogLevel          string          `desc:"Starting log level, wil hide all levels bellow it"`
	MessageOptions    *MessageOptions `desc:"Message configuration"`
}

type AppLogger struct {
	FileLogger  *logrus.Logger
	LastLogFile *LogFileInfo
	OsLogger    *logrus.Logger
	config      *AppLoggerOptions
}

type Logger struct {
	tag       string
	appLogger *AppLogger
}

type LogFileInfo struct {
	Name string
	File *os.File
}

type CustomFormatter struct {
	LevelColors *LevelColors
	LogToFile   bool
}

type LogLevelType = logrus.Level

const (
	InfoLevel  LogLevelType = logrus.InfoLevel
	WarnLevel  LogLevelType = logrus.WarnLevel
	ErrorLevel LogLevelType = logrus.ErrorLevel
	DebugLevel LogLevelType = logrus.DebugLevel
	TraceLevel LogLevelType = logrus.TraceLevel
	PanicLevel LogLevelType = logrus.PanicLevel
	FatalLevel LogLevelType = logrus.FatalLevel
)

type customEntryInfo struct {
	tag                    string
	subTags                string
	levelColor             string
	content                string
	contentDelimiter       string
	disableSpacing         bool
	textFiller             string
	messagePrealocatedSize int
	tagPrealocatedSize     int
	dateFormat             string
	useJsonOutput          bool
}

type JsonOutput struct {
	Time    string `json:"time"`
	Tag     string `json:"tag"`
	Level   string `json:"level"`
	Message string `json:"message"`
	SubTags string `json:"subTags,omitempty"`
	Content string `json:"content,omitempty"`
}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Customize the log entry format
	info := f.getCustomEntryInfo(entry)
	var output []byte
	dateFormat := "2006-01-02 15:04:05.000"
	if info.dateFormat != "" {
		dateFormat = info.dateFormat
	}

	if info.useJsonOutput || f.LogToFile {
		jsonLog := &JsonOutput{
			Time:    entry.Time.Format(dateFormat),
			Tag:     info.tag,
			SubTags: info.subTags,
			Level:   entry.Level.String(),
			Message: entry.Message,
			Content: info.content,
		}

		output, _ = json.Marshal(jsonLog)
		output = bytes.Join([][]byte{output, []byte("\r\n")}, []byte(""))
	} else {
		messageSize := 50
		tagSize := 25
		levelSize := 10
		contentDelimiter := " "
		subTags := info.subTags

		if info.messagePrealocatedSize > 0 {
			messageSize = info.messagePrealocatedSize
		}

		if info.tagPrealocatedSize > 0 {
			tagSize = info.tagPrealocatedSize
		}

		if info.contentDelimiter != "" {
			contentDelimiter = info.contentDelimiter
		}

		endContent := info.textFiller + contentDelimiter
		finalLogText := CompleteString(entry.Time.Format(dateFormat), len(dateFormat)+1, info.textFiller, info.disableSpacing) + endContent
		finalLogText += f.getLevelColor(entry.Level)
		finalLogText += CompleteString("["+entry.Level.String()+"]", levelSize, info.textFiller, info.disableSpacing) + endContent
		finalLogText += CompleteString(info.tag+contentDelimiter+subTags+resetColor, tagSize, info.textFiller, info.disableSpacing) + endContent
		finalLogText += CompleteString(entry.Message, messageSize, info.textFiller, info.disableSpacing) + endContent
		finalLogText += info.content + " \n"
		output = []byte(finalLogText)
	}

	return output, nil

}

func NewMainLogger(config *AppLoggerOptions) *AppLogger {
	// initial logger params with basic configuration, will be updated by the config
	if config.MessageOptions == nil {
		config.MessageOptions = &MessageOptions{}
	}

	logger := &AppLogger{
		config: config,
	}

	// os logger
	osLogger := logrus.New()
	osLogger.SetOutput(os.Stdout)
	osLogger.SetLevel(logrus.TraceLevel)
	osLogger.SetFormatter(&CustomFormatter{
		LogToFile: false,
		LevelColors: &LevelColors{
			// font color
			Log:   "\033[37m",
			Info:  "\033[34m",
			Warn:  "\033[33m",
			Error: "\033[31m",
			Debug: "\033[37m",
			Trace: "\033[36m",
			Panic: "\033[35m",
			Fatal: "\033[31m",
		},
	})
	logger.OsLogger = osLogger

	// file logger
	if !config.FileLogDisabled {
		fileLogger := logrus.New()
		logFile := logger.getLogFileName()
		file := logger.createLogFile(logFile)
		fileLogger.SetOutput(file)
		fileLogger.SetLevel(logrus.TraceLevel)
		fileLogger.SetFormatter(&CustomFormatter{
			LogToFile: true,
		})
		logger.FileLogger = fileLogger
		logger.LastLogFile = &LogFileInfo{
			Name: logFile,
			File: file,
		}
		logger.rotateLogFileLoop()
	}

	logger.SetConfig(*logger.config)

	return logger
}

func (l *AppLogger) getLogFileName() string {
	logFile := fmt.Sprintf("logs_%s", time.Now().UTC().Format(fileNameTimeFormat))
	logFolder := "./logs/"
	CreateFolderIfNotExists(logFolder)
	return fmt.Sprintf("%s%s.log", logFolder, logFile)
}

func (l *AppLogger) createLogFile(filePath string) *os.File {
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	return f
}

func (l *AppLogger) checkAndSetLogFile() {
	logFile := l.getLogFileName()
	if logFile != l.LastLogFile.Name {

		// closing last file reader
		l.LastLogFile.File.Close()

		// create new log file
		file := l.createLogFile(logFile)
		l.FileLogger.SetOutput(file)
		l.LastLogFile.Name = logFile
		l.LastLogFile.File = file

		l.Log(logrus.InfoLevel, libTag, nil, "New log file set", logFile)
	}
}

func (l *AppLogger) clearOldLogs() {
	folderPath := "./logs/"
	currentTime := time.Now().UTC()

	// Go through the folder and filter files older than x days
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasPrefix(info.Name(), "logs_") && strings.HasSuffix(info.Name(), ".log") {
			dateStr := strings.TrimPrefix(info.Name(), "logs_")
			dateStr = strings.TrimSuffix(dateStr, ".log")
			fileDate, err := time.Parse(fileNameTimeFormat, dateStr)

			if err != nil {
				return err
			}

			diffInDays := math.Floor(currentTime.Sub(fileDate).Hours() / 24)

			// check if the file is older than configured limit
			if diffInDays > float64(l.config.LogFileRotateDays) {
				err := os.Remove(folderPath + info.Name())
				if err != nil {
					l.Log(logrus.ErrorLevel, libTag, nil, "Error removing log file", err.Error(), info.Name())
				} else {
					l.Log(logrus.TraceLevel, libTag, nil, "Log file removed", info.Name())
				}

			}
		}

		return nil
	})

	if err != nil {
		l.Log(logrus.ErrorLevel, libTag, nil, "Error cleaning older logs", err.Error())
	}

}

func (l *AppLogger) rotateLogFileLoop() {
	go func() {
		// Create a ticker for the periodic task
		tickerClearLogs := time.NewTicker(2 * time.Hour)
		defer tickerClearLogs.Stop()

		tickerCheckLogFile := time.NewTicker(30 * time.Minute)
		defer tickerCheckLogFile.Stop()

		for {
			select {
			case <-tickerClearLogs.C:
				l.clearOldLogs()
			case <-tickerCheckLogFile.C:
				l.checkAndSetLogFile()
			}
		}

	}()
}

func (l *AppLogger) Log(level LogLevelType, tag string, subTags []string, message string, content ...string) {
	fields := map[string]any{}
	fields["tag"] = tag
	fields["subTags"] = subTags
	fields["contentDelimiter"] = l.config.MessageOptions.ContentDelimiter
	fields["disableSpacing"] = l.config.MessageOptions.DisableSpacing
	fields["textFiller"] = l.config.MessageOptions.TextFiller
	fields["messagePrealocatedSize"] = l.config.MessageOptions.MessagePrealocatedSize
	fields["tagPrealocatedSize"] = l.config.MessageOptions.TagPrealocatedSize
	fields["dateFormat"] = l.config.MessageOptions.DateFormat
	fields["useJsonOutput"] = l.config.MessageOptions.UseJsonOutput
	fields["content"] = content

	l.OsLogger.WithFields(fields).Log(level, message)

	if !l.config.FileLogDisabled {
		l.FileLogger.WithFields(fields).Log(level, message)
	}
}

func (l *AppLogger) SetConfig(opts AppLoggerOptions) {

	l.config.DisabledTags = opts.DisabledTags
	l.config.LogLevel = opts.LogLevel
	l.config.FileLogDisabled = opts.FileLogDisabled
	l.config.MessageOptions = opts.MessageOptions

	log.Println(l.config.MessageOptions)
	log.Println(opts.MessageOptions)

	if opts.LogFileRotateDays != l.config.LogFileRotateDays && opts.LogFileRotateDays > 0 {
		l.config.LogFileRotateDays = opts.LogFileRotateDays
	}

	switch opts.LogLevel {
	case "info":
		if !l.config.FileLogDisabled {
			l.FileLogger.SetLevel(logrus.InfoLevel)
		}
		l.OsLogger.SetLevel(logrus.InfoLevel)
	case "warn":
		if !l.config.FileLogDisabled {
			l.FileLogger.SetLevel(logrus.WarnLevel)
		}
		l.OsLogger.SetLevel(logrus.WarnLevel)
	case "error":
		if !l.config.FileLogDisabled {
			l.FileLogger.SetLevel(logrus.ErrorLevel)
		}
		l.OsLogger.SetLevel(logrus.ErrorLevel)
	case "debug":
		if !l.config.FileLogDisabled {
			l.FileLogger.SetLevel(logrus.DebugLevel)
		}
		l.OsLogger.SetLevel(logrus.DebugLevel)
	case "trace":
		if !l.config.FileLogDisabled {
			l.FileLogger.SetLevel(logrus.TraceLevel)
		}
		l.OsLogger.SetLevel(logrus.TraceLevel)
	case "panic":
		if !l.config.FileLogDisabled {
			l.FileLogger.SetLevel(logrus.PanicLevel)
		}
		l.OsLogger.SetLevel(logrus.PanicLevel)
	case "fatal":
		if !l.config.FileLogDisabled {
			l.FileLogger.SetLevel(logrus.FatalLevel)
		}
		l.OsLogger.SetLevel(logrus.FatalLevel)
	default:
		if !l.config.FileLogDisabled {
			l.config.LogLevel = "trace"
		}
		l.Log(logrus.WarnLevel, libTag, nil, "Unknown log level on log config update, trace level set", opts.LogLevel)
	}

	jsonConfig, _ := json.Marshal(l.config)
	l.Log(logrus.InfoLevel, libTag, nil, "New logger config set", string(jsonConfig))

}

// LOGGER

func NewLogger(tag string, appLogr *AppLogger) *Logger {
	logger := &Logger{
		tag:       tag,
		appLogger: appLogr,
	}

	return logger
}

func (l *Logger) setHelperValues(subTags []string, content []string) (map[string]any, string) {
	// we set the custom values here to use it in the custom formatter
	argsMap := map[string]any{}
	// required
	argsMap["tag"] = l.tag
	// optional
	argsMap["subTags"] = subTags
	argsMap["contentDelimiter"] = l.appLogger.config.MessageOptions.ContentDelimiter
	argsMap["disableSpacing"] = l.appLogger.config.MessageOptions.DisableSpacing
	argsMap["textFiller"] = l.appLogger.config.MessageOptions.TextFiller
	argsMap["messagePrealocatedSize"] = l.appLogger.config.MessageOptions.MessagePrealocatedSize
	argsMap["tagPrealocatedSize"] = l.appLogger.config.MessageOptions.TagPrealocatedSize
	argsMap["dateFormat"] = l.appLogger.config.MessageOptions.DateFormat
	argsMap["useJsonOutput"] = l.appLogger.config.MessageOptions.UseJsonOutput
	argsMap["content"] = content
	return argsMap, l.tag
}

func (l *Logger) Log(level logrus.Level, message string, subTags []string, args ...string) {
	logArgs, tag := l.setHelperValues(subTags, args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	if !l.appLogger.config.FileLogDisabled {
		l.appLogger.FileLogger.WithFields(logArgs).Log(level, message)
	}
	l.appLogger.OsLogger.WithFields(logArgs).Log(level, message)
}

func (l *Logger) Warn(message string, subTags []string, content ...string) {
	args, tag := l.setHelperValues(subTags, content)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	if !l.appLogger.config.FileLogDisabled {
		l.appLogger.FileLogger.WithFields(args).Warn(message)
	}
	l.appLogger.OsLogger.WithFields(args).Warn(message)
}

func (l *Logger) Info(message string, subTags []string, content ...string) {
	args, tag := l.setHelperValues(subTags, content)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	if !l.appLogger.config.FileLogDisabled {
		l.appLogger.FileLogger.WithFields(args).Info(message)
	}
	l.appLogger.OsLogger.WithFields(args).Info(message)
}

func (l *Logger) Error(message string, subTags []string, content ...string) {
	args, tag := l.setHelperValues(subTags, content)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	if !l.appLogger.config.FileLogDisabled {
		l.appLogger.FileLogger.WithFields(args).Error(message)
	}
	l.appLogger.OsLogger.WithFields(args).Error(message)
}

func (l *Logger) Debug(message string, subTags []string, content ...string) {
	args, tag := l.setHelperValues(subTags, content)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	if !l.appLogger.config.FileLogDisabled {
		l.appLogger.FileLogger.WithFields(args).Debug(message)
	}
	l.appLogger.OsLogger.WithFields(args).Debug(message)
}

func (l *Logger) Panic(message string, subTags []string, content ...string) {
	args, tag := l.setHelperValues(subTags, content)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	if !l.appLogger.config.FileLogDisabled {
		l.appLogger.FileLogger.WithFields(args).Panic(message)
	}
	l.appLogger.OsLogger.WithFields(args).Panic(message)
}

func (l *Logger) Trace(message string, subTags []string, content ...string) {
	args, tag := l.setHelperValues(subTags, content)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	if !l.appLogger.config.FileLogDisabled {
		l.appLogger.FileLogger.WithFields(args).Trace(message)
	}
	l.appLogger.OsLogger.WithFields(args).Trace(message)
}

func (l *Logger) Fatal(message string, subTags []string, content ...string) {
	args, tag := l.setHelperValues(subTags, content)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	if !l.appLogger.config.FileLogDisabled {
		l.appLogger.FileLogger.WithFields(args).Fatal(message)
	}
	l.appLogger.OsLogger.WithFields(args).Fatal(message)
}

// util functions
func CreateFolderIfNotExists(folderPath string) error {
	// Check if the folder already exists
	_, err := os.Stat(folderPath)

	if os.IsNotExist(err) {
		// Create the folder if it does not exist
		err := os.MkdirAll(folderPath, os.ModePerm)
		if err != nil {
			return err
		}
	} else if err != nil {

		return err
	}

	return nil
}

func CompleteString(originalString string, targetLength int, fillChar string, disableSpacing bool) string {
	// fill the original string with the fillChar
	resultString := originalString
	// Customize the log entry format

	if !disableSpacing && len(resultString) < targetLength {
		if fillChar == "" {
			fillChar = " "
		}
		charsToAdd := targetLength - len(originalString)
		additionalChars := strings.Repeat(fillChar, charsToAdd)
		resultString = originalString + additionalChars
	}

	return resultString
}

func ConvertToString(value interface{}) string {

	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []string:
		strSlice := make([]string, len(v))
		for i, item := range v {
			strSlice[i] = ConvertToString(item)
		}
		return strings.Join(strSlice, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func InterfaceToStringSlice(input interface{}) []string {
	if slice, ok := input.([]string); ok {
		return slice
	}
	return []string{}
}

func (f *CustomFormatter) getLevelColor(level logrus.Level) string {
	var levelColor string
	// Compare the log level to specific logrus constants
	switch level {
	case logrus.InfoLevel:
		levelColor = f.LevelColors.Info
	case logrus.WarnLevel:
		levelColor = f.LevelColors.Warn
	case logrus.ErrorLevel:
		levelColor = f.LevelColors.Error
	case logrus.DebugLevel:
		levelColor = f.LevelColors.Debug
	case logrus.PanicLevel:
		levelColor = f.LevelColors.Panic
	case logrus.TraceLevel:
		levelColor = f.LevelColors.Trace
	case logrus.FatalLevel:
		levelColor = f.LevelColors.Fatal
	default:
		levelColor = f.LevelColors.Log
	}
	return levelColor
}

func (f *CustomFormatter) getCustomEntryInfo(entry *logrus.Entry) *customEntryInfo {

	info := &customEntryInfo{}

	for i, v := range entry.Data {
		switch i {
		case "tag":
			info.tag = ConvertToString(v)
			delete(entry.Data, "tag")
		case "content":
			info.content = ConvertToString(v)
			delete(entry.Data, "content")
		case "contentDelimiter":
			info.contentDelimiter = ConvertToString(v)
			delete(entry.Data, "contentDelimiter")
		case "dateFormat":
			info.dateFormat = ConvertToString(v)
			delete(entry.Data, "dateFormat")
		case "messagePrealocatedSize":
			valInt, _ := strconv.ParseInt(ConvertToString(v), 10, 0)
			info.messagePrealocatedSize = int(valInt)
			delete(entry.Data, "messagePrealocatedSize")
		case "tagPrealocatedSize":
			valInt, _ := strconv.ParseInt(ConvertToString(v), 10, 0)
			info.tagPrealocatedSize = int(valInt)
			delete(entry.Data, "tagPrealocatedSize")
		case "textFiller":
			info.textFiller = ConvertToString(v)
			delete(entry.Data, "textFiller")
		case "useJsonOutput":
			if v == true {
				info.useJsonOutput = true
			} else {
				info.useJsonOutput = false
			}
			delete(entry.Data, "useJsonOutput")
		case "disableSpacing":
			if v == true {
				info.disableSpacing = true
			} else {
				info.disableSpacing = false
			}
			delete(entry.Data, "disableSpacing")
		case "subTags":
			subTags := ConvertToString(v)
			if len(subTags) > 0 {
				info.subTags = subTags
			} else {
				info.subTags = ""
			}
			delete(entry.Data, "subTags")
		default:
		}
	}

	// this must stay here, will make all the terminal colored if is done before the "delete" above
	if !f.LogToFile {
		info.levelColor = f.getLevelColor(entry.Level)
	}

	return info
}
