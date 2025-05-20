package log

import "go.uber.org/zap"

var Logger *zap.Logger
var SugarLogger *zap.SugaredLogger

func init() {
	Logger, _ = zap.NewDevelopment()
	SugarLogger = Logger.Sugar()
}
