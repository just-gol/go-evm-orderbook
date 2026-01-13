package logger

import "github.com/sirupsen/logrus"

func Init() {
	logrus.SetFormatter(&logrus.JSONFormatter{ // json格式
		TimestampFormat: "2006-01-02 15:04:05",
		PrettyPrint:     true, // 开着会更好看,控制台打印json格式，但不适合高并发场景
	})
	logrus.SetLevel(logrus.InfoLevel)
}

func WithModule(module string) *logrus.Entry {
	return logrus.WithField("module", module)
}
