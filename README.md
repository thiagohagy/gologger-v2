
#### Gologger

A golang custom logger based on the loggrus logger:
https://github.com/sirupsen/logrus

It is a private project , but once gained access , to install you should use

```go get github.com/thiagohagy/gologger```

Created to make possible to have logs separated by type and module, where you can enable/disable using the tags and logs types

You will have two types of structure, app and module logger
The module loggers will have its own tag

This package will log to your terminal and to a file(one file per day) under a /logs folder in your project root folder

You can configure a log rotate (30 days by default)

#### Creating a custom logger for your app:
To use it as inttended, you should create a appLogger, and use it to create your module loggers

On you main.go file:

~~~go
// creating the main app logger
appLogger := gologger.NewMainLogger()

// creating a module logger
logger := gologger.NewModuleLogger("YOUR_MODULE_LOG_TAG", params.AppLogger)


// using your module logger
logger.Info("A informative log message", logrus.Fields{
    "data": someDataYouWantToShow
})


// Setting a custom config
appLogger.SetConfig(
    gologger.AppLoggerOptions{
        DisabledTags:      arrayOfDisabledTags,
        LogFileRotateDays: 10,
        LogLevel:          "info", //can be trace, debug, info, waning , fatal, panic
    },
)
~~~

You can use the log functions by calling: 
- appLogger.Log(logrus.InfoLevel, "YOUR_TAG", "Message", logrus.Fields{})
- moduleLogger.Trace("Message", logrus.Fields{})
- moduleLogger.Debug("Message", logrus.Fields{})
- moduleLogger.Info("Message", logrus.Fields{})
- moduleLogger.Warn("Message", logrus.Fields{})
- moduleLogger.Fatal("Message", logrus.Fields{})
- moduleLogger.Panic("Message", logrus.Fields{})