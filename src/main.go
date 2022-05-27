package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"fttf/src/cfg"
	"fttf/src/crontab"
	"fttf/src/http"
	"fttf/src/logimp"
	"fttf/src/scheduler"
	"fttf/src/socket"
	"io/ioutil"
	"log"
	nhttp "net/http"
	"os"
	"sync"
	"time"
)

// 声明全局等待组变量
var wg sync.WaitGroup
var mylog *log.Logger
var fw *os.File
var logname string = "main.log"

var configmap map[string]*cfg.Config

/*
fttf有两种工作模式
一种是服务端运行, 需要指定sport和hport
一种是客户端模式，需要指定config和path参数
程序通过判断是否具备config和path参数，来决定用哪种方式运行
fttf -config atob -path qjh -ip x.x.x.x -port xxxx
*/

func main() {

	//传输时使用的配置参数
	var config string
	//传输时的路径
	var path string
	//连接fttf服务的ip
	var ip string
	//连接fttf服务的port
	var port int

	//socket 传输端口
	var sport int
	//http管理端口
	var hport int

	flag.StringVar(&config, "config", "", "传输时使用的配置参数")
	flag.StringVar(&path, "path", "", "传输时的路径")
	flag.StringVar(&ip, "ip", "localhost", "连接fttf服务的ip")
	flag.IntVar(&port, "port", 32555, "连接fttf服务的port")
	flag.IntVar(&sport, "sport", 32666, "数据传输端口")
	flag.IntVar(&hport, "hport", 32555, "管理端口")
	flag.Parse()

	//客户端模式
	if config != "" {
		url := fmt.Sprintf("http://%s:%d/go?RuleName=%s&SrcPath=%s", ip, port, config, path)

		b, _ := goClient(url)
		if b {
			os.Exit(0) //成功退出
		} else {
			os.Exit(1) //异常退出
		}
	}

	mylog, fw = logimp.InitLog(logname)
	defer fw.Close()
	logimp.Info(mylog, "%s\n", startInfo())
	logimp.Info(mylog, "sport=%d,hport=%d\n", sport, hport)

	cfg.Init(mylog)

	configmap, _ = cfg.ReadAllConfig()

	http.Init(mylog, configmap)

	socket.Init(mylog, configmap)

	scheduler.Init(configmap)

	crontab.Init(mylog)
	crontab.ReadAllCrontab()

	wg.Add(1) // 登记1个goroutine
	go goRunHttpServer(hport)

	wg.Add(1) // 登记1个goroutine
	go goRunSocketServer(sport)

	wg.Add(1) // 登记1个goroutine
	go goRunCrontab()

	wg.Wait() // 阻塞等待登记的goroutine完成

}

func goClient(url string) (bool, error) {

	// 创建 client 和 resp 对象
	var client nhttp.Client
	defer client.CloseIdleConnections()

	// 这里博主设置了10秒钟的超时
	client = nhttp.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(url)

	defer resp.Body.Close()
	if err != nil {
		return false, err
	}

	body, errb := ioutil.ReadAll(resp.Body)
	if errb != nil {
		return false, errb
	}

	var res http.Rsp
	errj := json.Unmarshal(body, &res)
	if errj != nil {
		return false, errj
	}

	if res.RspCode != cfg.OK {
		return false, errors.New(fmt.Sprintf("%s", res.RspMsg))
	}
	return true, nil

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
