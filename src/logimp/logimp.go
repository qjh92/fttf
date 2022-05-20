package logimp

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
)

var Printf_flag bool = true

func InitLog(logname string) (*log.Logger, *os.File) {
	_, errs := os.Stat("logs")

	if os.IsNotExist(errs) {
		// 创建文件夹
		os.MkdirAll("logs", os.ModePerm)
	}

	_, errs = os.Stat("logs" + string(filepath.Separator) + "server")
	if os.IsNotExist(errs) {
		// 创建文件夹
		os.MkdirAll("logs"+string(filepath.Separator)+"server", os.ModePerm)
	}

	_, errs = os.Stat("logs" + string(filepath.Separator) + "client")
	if os.IsNotExist(errs) {
		// 创建文件夹
		os.MkdirAll("logs"+string(filepath.Separator)+"client", os.ModePerm)
	}

	_, errs = os.Stat("logs" + string(filepath.Separator) + "scheduler")
	if os.IsNotExist(errs) {
		// 创建文件夹
		os.MkdirAll("logs"+string(filepath.Separator)+"scheduler", os.ModePerm)
	}

	fw, err := os.OpenFile("logs"+string(filepath.Separator)+logname, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0755)
	if err != nil {
		log.Fatalf("create file log.txt failed: %v", err)
	}

	return log.New(io.MultiWriter(fw, os.Stdout), "", log.Ldate|log.Lmicroseconds), fw
}

//正常输出信息,同时保存到日志文件
func Info(lg *log.Logger, format string, v ...interface{}) {

	_, filename, lineNo, _ := getCallerInfo(2)
	format = fmt.Sprintf("%s:%d %s", filename, lineNo, format)

	lg.SetPrefix("INFO  ")
	lg.Printf(format, v...)

}

//异常情况信息,同时保存到日志文件
func Warn(lg *log.Logger, format string, v ...interface{}) {
	_, filename, lineNo, _ := getCallerInfo(2)
	format = fmt.Sprintf("%s:%d %s", filename, lineNo, format)

	lg.SetPrefix("WAR   ")
	lg.Printf(format, v...)
}

//严重错误，逐级调用defer，程序最后退出,,同时保存到日志文件
func ErrorQuit(lg *log.Logger, format string, v ...interface{}) {
	_, filename, lineNo, _ := getCallerInfo(2)
	format = fmt.Sprintf("%s:%d %s", filename, lineNo, format)

	lg.SetPrefix("ERR   ")
	lg.Panicf(format, v...)
}

//只是打印输出到stdout，等价于fmt的Printf
func Printf(format string, v ...interface{}) {
	if Printf_flag {
		_, filename, lineNo, _ := getCallerInfo(2)
		format = fmt.Sprintf("%s:%d %s", filename, lineNo, format)

		fmt.Printf(format, v...)
	}
}

func getCallerInfo(skip int) (funcName, fileName string, lineNo int, ok bool) {
	pc, file, lineNo, ok := runtime.Caller(skip)
	if !ok { //获取失败的时候，赋一个固定的值，xxx -999
		funcName = "xxx"
		fileName = "xxx"
		lineNo = -999
		return
	} else {
		funcName = runtime.FuncForPC(pc).Name()
		fileName = path.Base(file) // Base函数返回路径的最后一个元素
	}
	return
}
