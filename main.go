package gologger

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
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

type AppLoggerOptions struct {
	LogFileRotateDays int
	DisabledTags      []string
	LogLevel          string
}

type AppLogger struct {
	EnabledTags []string
	FileLogger  *logrus.Logger
	LastLogFile *LogFileInfo
	OsLogger    *logrus.Logger
	config      *AppLoggerOptions
}

type LogFileInfo struct {
	Name string
	File *os.File
}

type CustomFormatter struct {
	LevelColors *LevelColors
	LogToFile   bool
}

func (f *CustomFormatter) getFieldsAsString(fields logrus.Fields) string {
	// Customize the log entry format
	var result string
	var fieldCount = 0

	keys := make([]string, 0, len(fields))

	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		fieldCount += 1
		pre := ""
		if fieldCount > 1 {
			pre = ", "
		}
		result = result + fmt.Sprintf("%s%s: %s", pre, ConvertToString(key), ConvertToString(fields[key]))
	}

	return result
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

type customEntryInfo struct {
	tag        string
	customTag  string
	levelColor string
	fields     string
}

func (f *CustomFormatter) getCustomEntryInfo(entry *logrus.Entry) *customEntryInfo {

	info := &customEntryInfo{}

	for i, v := range entry.Data {
		if i == "tag" {
			info.tag = ConvertToString(v)
			delete(entry.Data, "tag")
		}
		if i == "customTag" {
			info.customTag = ConvertToString(v)
			delete(entry.Data, "customTag")
		}
	}

	// this must stay here, will make all the terminal colored if is done before the "delete" above
	if !f.LogToFile {
		info.levelColor = f.getLevelColor(entry.Level)
	}

	info.fields = f.getFieldsAsString(entry.Data)

	return info
}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Customize the log entry format

	info := f.getCustomEntryInfo(entry)

	customTag := ""
	if info.customTag != "" {
		customTag = f.getLevelColor(entry.Level) + CompleteString("["+info.customTag+"]"+resetColor, 20, " ") + " "
	}

	if f.LogToFile {
		return []byte(fmt.Sprintf(
			CompleteString("["+entry.Level.String()+"]", 15, " ") +
				CompleteString(info.tag, 20, " ") + " " +
				customTag +
				entry.Time.Format("2006-01-02 15:04:05") + "  " +
				CompleteString(entry.Message, 50, " ") + " " +
				info.fields +
				" \n")), nil
	} else {
		return []byte(fmt.Sprintf(
			f.getLevelColor(entry.Level) +
				CompleteString("["+entry.Level.String()+"]"+resetColor, 15, " ") +
				CompleteString(info.tag, 20, " ") + " " +
				customTag +
				entry.Time.Format("2006-01-02 15:04:05") + "  " +
				CompleteString(entry.Message, 50, " ") + " " +
				info.fields +
				" \n")), nil
	}

}

func NewMainLogger() *AppLogger {
	// initial logger params with basic configuration, will be updated by the config
	logger := &AppLogger{
		config: &AppLoggerOptions{
			LogFileRotateDays: 30,
		},
	}

	fileLogger := logrus.New()
	osLogger := logrus.New()

	// file logger
	logFile := logger.getLogFileName()
	file := logger.createLogFile(logFile)
	fileLogger.SetOutput(file)
	fileLogger.SetLevel(logrus.TraceLevel)
	fileLogger.SetFormatter(&CustomFormatter{
		LogToFile: true,
	})

	// os logger
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

	logger.FileLogger = fileLogger
	logger.LastLogFile = &LogFileInfo{
		Name: logFile,
		File: file,
	}
	logger.OsLogger = osLogger
	logger.rotateLogFileLoop()

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

		l.Log(logrus.InfoLevel, libTag, "New log file set", logrus.Fields{
			"fileName": logFile,
		})
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
					l.Log(logrus.ErrorLevel, libTag, "Error removing log file", logrus.Fields{
						"err":      err.Error(),
						"fileName": info.Name(),
					})
				} else {
					l.Log(logrus.TraceLevel, libTag, "Log file removed", logrus.Fields{
						"fileName": info.Name(),
					})
				}

			}
		}

		return nil
	})

	if err != nil {
		l.Log(logrus.ErrorLevel, libTag, "Error cleaning older logs", logrus.Fields{
			"err": err.Error(),
		})
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

func (l *AppLogger) Log(level logrus.Level, tag string, message string, fields logrus.Fields) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["tag"] = tag
	l.FileLogger.WithFields(fields).Log(level, message)
	l.OsLogger.WithFields(fields).Log(level, message)
}

func (l *AppLogger) SetConfig(opts AppLoggerOptions) {

	l.config.DisabledTags = opts.DisabledTags
	l.config.LogLevel = opts.LogLevel

	if opts.LogFileRotateDays != l.config.LogFileRotateDays && opts.LogFileRotateDays > 0 {
		l.config.LogFileRotateDays = opts.LogFileRotateDays
	}

	switch opts.LogLevel {
	case "info":
		l.FileLogger.SetLevel(logrus.InfoLevel)
		l.OsLogger.SetLevel(logrus.InfoLevel)
	case "warn":
		l.FileLogger.SetLevel(logrus.WarnLevel)
		l.OsLogger.SetLevel(logrus.WarnLevel)
	case "error":
		l.FileLogger.SetLevel(logrus.ErrorLevel)
		l.OsLogger.SetLevel(logrus.ErrorLevel)
	case "debug":
		l.FileLogger.SetLevel(logrus.DebugLevel)
		l.OsLogger.SetLevel(logrus.DebugLevel)
	case "trace":
		l.FileLogger.SetLevel(logrus.TraceLevel)
		l.OsLogger.SetLevel(logrus.TraceLevel)
	case "panic":
		l.FileLogger.SetLevel(logrus.PanicLevel)
		l.OsLogger.SetLevel(logrus.PanicLevel)
	case "fatal":
		l.FileLogger.SetLevel(logrus.FatalLevel)
		l.OsLogger.SetLevel(logrus.FatalLevel)
	default:
		l.config.LogLevel = "trace"
		l.Log(logrus.WarnLevel, libTag, "Unknown log level on log config update, trace level set", logrus.Fields{
			"logLevel": opts.LogLevel,
		})
	}

	jsonConfig, _ := json.Marshal(l.config)
	l.Log(logrus.InfoLevel, libTag, "New logger config set", logrus.Fields{
		"newConfig": string(jsonConfig),
	})

}

// MODULE LOGGER
type ModuleLogger struct {
	tag       string
	appLogger *AppLogger
}

func NewModuleLogger(tag string, appLogr *AppLogger) *ModuleLogger {
	logger := &ModuleLogger{
		tag:       tag,
		appLogger: appLogr,
	}
	return logger
}

func (l *ModuleLogger) setTag(args logrus.Fields) (logrus.Fields, string) {
	if args == nil {
		args = logrus.Fields{}
	}

	if args["tag"] == nil {
		args["tag"] = l.tag
	}

	return args, l.tag
}

func (l *ModuleLogger) Log(level logrus.Level, message string, args logrus.Fields) {
	args, tag := l.setTag(args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	l.appLogger.FileLogger.WithFields(args).Log(level, message)
	l.appLogger.OsLogger.WithFields(args).Log(level, message)
}

func (l *ModuleLogger) Warn(message string, args logrus.Fields) {
	args, tag := l.setTag(args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	l.appLogger.FileLogger.WithFields(args).Warn(message)
	l.appLogger.OsLogger.WithFields(args).Warn(message)
}

func (l *ModuleLogger) Info(message string, args logrus.Fields) {
	args, tag := l.setTag(args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	l.appLogger.FileLogger.WithFields(args).Info(message)
	l.appLogger.OsLogger.WithFields(args).Info(message)
}

func (l *ModuleLogger) Error(message string, args logrus.Fields) {
	args, tag := l.setTag(args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	l.appLogger.FileLogger.WithFields(args).Error(message)
	l.appLogger.OsLogger.WithFields(args).Error(message)
}

func (l *ModuleLogger) Debug(message string, args logrus.Fields) {
	args, tag := l.setTag(args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	l.appLogger.FileLogger.WithFields(args).Debug(message)
	l.appLogger.OsLogger.WithFields(args).Debug(message)
}

func (l *ModuleLogger) Panic(message string, args logrus.Fields) {
	args, tag := l.setTag(args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	l.appLogger.FileLogger.WithFields(args).Panic(message)
	l.appLogger.OsLogger.WithFields(args).Panic(message)
}

func (l *ModuleLogger) Trace(message string, args logrus.Fields) {
	args, tag := l.setTag(args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	l.appLogger.FileLogger.WithFields(args).Trace(message)
	l.appLogger.OsLogger.WithFields(args).Trace(message)
}

func (l *ModuleLogger) Fatal(message string, args logrus.Fields) {
	args, tag := l.setTag(args)
	if slices.Contains(l.appLogger.config.DisabledTags, tag) {
		return
	}
	l.appLogger.FileLogger.WithFields(args).Fatal(message)
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

func CompleteString(originalString string, targetLength int, fillChar string) string {
	// fill the original string with the fillChar
	resultString := originalString
	// Customize the log entry format
	if len(resultString) < targetLength {
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
	default:
		return fmt.Sprintf("%v", v)
	}
}
