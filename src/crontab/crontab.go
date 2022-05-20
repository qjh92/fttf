package crontab

import (
	"fmt"
	"github.com/robfig/cron"
	"log"
	"os"
)

var mylog *log.Logger
var fw *os.File

func Init(a *log.Logger) {
	mylog = a
	_, errs := os.Stat("crontabs")
	if os.IsNotExist(errs) {
		// 创建文件夹
		os.MkdirAll("crontabs", os.ModePerm)
	}
}

func RunCrontab() {
	_, b := cron.Parse("* * * * * *")
	fmt.Println(b)

	//c := cron.New()  // 新建一个定时任务对象
	//c.AddFunc("* * * * * *", func() {
	//	log.Println("hello world")
	//})  // 给对象增加定时任务
	//c.Start()
	//select {
	//}
}
