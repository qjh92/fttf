package crontab

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"log"
	"os"
	"time"
)

/*
github.com/robfig/cron/v3相关说明


 ┌───────────── min (0 - 59)
 │ ┌────────────── hour (0 - 23)
 │ │ ┌─────────────── day of month (1 - 31)
 │ │ │ ┌──────────────── month (1 - 12)
 │ │ │ │ ┌───────────────── day of week (0 - 6) (0 to 6 are Sunday to
 │ │ │ │ │                  Saturday)
 │ │ │ │ │
 │ │ │ │ │
 * * * * *


*/





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

	pr:=cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_,e:=pr.Parse("*/62 * * * *")
	fmt.Printf("e=%v\n",e)

	c := cron.New()
	//a,err:=c.AddFunc("* * * * * *", func() { fmt.Println("Every sec ") })
	//fmt.Printf("a=%d,err=%v\n",a,err)
	//c.AddFunc("*/5 * * * * *",      func() { fmt.Println("Every 5 sec") })
	a,err:=c.AddFunc("* * * * *", func() { fmt.Printf("Every min,%v\n",time.Now()) })
	fmt.Printf("a=%d,err=%v\n",a,err)
	a,err=c.AddFunc("*/68 * * * *", func() { fmt.Printf("Every 68 min,%v\n",time.Now()) })
	fmt.Printf("a=%d,err=%v\n",a,err)
	a,err=c.AddFunc("*/2 * * * *", func() { fmt.Printf("Every 2 mins,%v\n",time.Now()) })
	fmt.Printf("a=%d,err=%v\n",a,err)
	c.Start()

	time.Sleep(65*time.Second)
	c.Remove(1)

	select {
	}
}
