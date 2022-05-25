package main

import (
	"flag"
	"fttf/src/cfg"
	"fttf/src/crontab"
	"fttf/src/http"
	"fttf/src/logimp"
	"fttf/src/scheduler"
	"fttf/src/socket"
	"log"
	"os"
	"sync"
)

// 声明全局等待组变量
var wg sync.WaitGroup
var mylog *log.Logger
var fw *os.File
var logname string = "main.log"

var configmap map[string]*cfg.Config

func main() {

	mylog, fw = logimp.InitLog(logname)
	defer fw.Close()
	logimp.Info(mylog, "%s\n", startInfo())

	cfg.Init(mylog)
	configmap, _ = cfg.ReadAllConfig()

	http.Init(mylog, configmap)

	socket.Init(mylog, configmap)

	scheduler.Init(configmap)

	crontab.Init(mylog)
	crontab.ReadAllCrontab()

	var sport int
	var hport int
	flag.IntVar(&sport, "sport", 32666, "数据传输端口")
	flag.IntVar(&hport, "hport", 32555, "管理端口")
	flag.Parse()

	logimp.Info(mylog, "sport=%d,hport=%d\n", sport, hport)

	wg.Add(1) // 登记1个goroutine
	go goRunHttpServer(hport)

	wg.Add(1) // 登记1个goroutine
	go goRunSocketServer(sport)

	wg.Add(1) // 登记1个goroutine
	go goRunCrontab()

	wg.Wait() // 阻塞等待登记的goroutine完成

}

func goRunHttpServer(hport int) {
	defer wg.Done() // goroutine结束就登记-1
	http.RunHttpServer(hport)
}

func goRunSocketServer(sport int) {
	defer wg.Done() // goroutine结束就登记-1
	socket.RunSocketServer(sport)
}

func goRunCrontab() {
	defer wg.Done() // goroutine结束就登记-1
	crontab.RunCrontab()
}

func startInfo() string {
	str := `

    ____________   __    __       ____________
   / ____/_  __/  / /    \ \     /_  __/ ____/
  / /_    / /    / / _____\ \     / / / /_    
 / __/   / /     \ \/_____/ /    / / / __/    
/_/     /_/       \_\    /_/    /_/ /_/
`
	return str
}
