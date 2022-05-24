package http

import (
	"embed"
	"encoding/json"
	"fmt"
	"fttf/src/cfg"
	"fttf/src/crontab"
	"fttf/src/logimp"
	"fttf/src/scheduler"
	"fttf/src/tool"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Rsp struct {
	RspCode    int
	RspMsg     interface{} //返回主要的数据
	RspExtMsg  interface{} //根据情况，返回的附加数据
	PageUp     bool
	PageDown   bool
	TotalCount int
	StartIndex int
	EndIndex   int
}

/*
嵌入静态资源，go1.16之后支持
对于想要嵌入进程序的资源，需要使用//go:embed指令进行声明，注意//之后不能有空格
*/

//go:embed website
var websitefile embed.FS

var mxlock sync.Mutex // 互斥锁
var mylog *log.Logger
var configmap map[string]*cfg.Config

func Init(a *log.Logger, b map[string]*cfg.Config) {
	mylog = a
	configmap = b
}

func RunHttpServer(hport int) {

	server := http.Server{
		Addr: "0.0.0.0:" + strconv.Itoa(hport),
	}

	logimp.Info(mylog, "启动httpserver......hport=%d\n", hport)

	http.HandleFunc("/", index)

	http.HandleFunc("/static/", staticResource) //web静态资源，已经内嵌入程序包中
	http.HandleFunc("/log", logsResource)       //运行过程中生成的日志文件

	http.HandleFunc("/addconfig", addConfig)
	http.HandleFunc("/listconfig", listConfig)
	http.HandleFunc("/queryallconfig", queryAllConfig)
	http.HandleFunc("/queryconfig", queryConfig)
	http.HandleFunc("/deleteconfig", deleteConfig)
	http.HandleFunc("/queryalltask", queryalltask)
	http.HandleFunc("/queryonetask", queryonetask)
	http.HandleFunc("/flushconfig", flushconfig)

	http.HandleFunc("/addcrontab", addcrontab)
	http.HandleFunc("/queryallcrontab", queryAllCrontab)
	http.HandleFunc("/deletecrontab", deletecrontab)
	http.HandleFunc("/changecrontabstat", changecrontabstat)


	http.HandleFunc("/gotest", goTest)

	err := server.ListenAndServe()
	if err != nil {
		logimp.ErrorQuit(mylog, "启动httpserver失败,hport=%d,error=%v \n", hport, err)
		return
	}

}

//处理html页面
func index(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	var tp *template.Template
	var templatePath string

	if r.URL.Path == "/" {
		templatePath = "website/index.html"
	} else {
		templatePath = "website" + r.URL.Path
	}

	data, errd := websitefile.ReadFile(templatePath)
	if errd != nil {
		ResponseERROR(w, errd)
		return
	}

	tp, errd = template.New(templatePath).Parse(fmt.Sprintf("%s", data))
	if errd != nil {
		ResponseERROR(w, errd)
		return
	}
	tp.Execute(w, "")

}

//处理static下的静态资源请求
func staticResource(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	data, _ := websitefile.ReadFile("website" + r.URL.Path)
	path := r.URL.Path
	contentType := "text/plain"

	if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(path, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(path, ".jpg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(path, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "text/javascript"
	} else if strings.HasSuffix(path, ".html") {
		contentType = "text/html"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

//处理/logs/下的资源请求
func logsResource(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form
	var seqno string
	v, ok := params["seqno"]
	if ok && len(v[0]) > 0 {
		seqno = v[0]
	} else {
		ResponseERROR(w, "seqno参数不合法!")
		return
	}

	filename := "c_" + seqno + ".log"
	f, err := os.Open("logs" + string(filepath.Separator) + "client" + string(filepath.Separator) + filename)
	if err != nil {
		ResponseERRORFormat(w, "%v", err)
		return
	}
	defer f.Close()

	data, errd := ioutil.ReadAll(f)
	if errd != nil {
		ResponseERRORFormat(w, "%v", errd)
		return
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}

//添加配置
func addConfig(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	var config cfg.Config

	v, ok := params["RuleName"]
	if ok && len(v[0]) > 0 {
		config.RuleName = v[0]
	} else {
		ResponseERROR(w, "规则名称参数不合法!")
		return
	}

	v, ok = params["PGmod"]
	if ok && len(v[0]) > 0 {
		config.PGmod = v[0]
	} else {
		ResponseERROR(w, "PGmod参数不合法!")
		return
	}

	v, ok = params["LocalPath"]
	if ok && len(v[0]) > 0 {
		config.LocalPath = tool.FormatPath(v[0])
	} else {
		ResponseERROR(w, "LocalPath参数不合法!")
		return
	}

	v, ok = params["RemoteIP"]
	if ok && len(v[0]) > 0 {
		config.RemoteIP = v[0]
	} else {
		ResponseERROR(w, "RemoteIP参数不合法!")
		return
	}

	v, ok = params["RemotePort"]
	if ok && len(v[0]) > 0 {
		config.RemotePort = v[0]
	} else {
		ResponseERROR(w, "RemotePort参数不合法!")
		return
	}

	v, ok = params["RemotePath"]
	if ok && len(v[0]) > 0 {
		config.RemotePath = tool.FormatPath(v[0])
	} else {
		ResponseERROR(w, "RemotePath参数不合法!")
		return
	}

	//windows路径和文件名不区分大小写，linux区分大小写
	if strings.ToLower(config.LocalPath) == strings.ToLower(config.RemotePath) {
		ResponseERROR(w, "LocalPath==RemotePath不合法!")
		return
	}

	v, ok = params["OverWrite"]
	if ok && len(v[0]) > 0 {
		config.OverWrite, _ = strconv.ParseBool(v[0])
	} else {
		ResponseERROR(w, "OverWrite参数不合法!")
		return
	}

	v, ok = params["Describle"]
	if ok {
		config.Describle = v[0]
	}

	config.DateTime = time.Now().Format("2006-01-02 15:04:05")

	logimp.Printf("%#v\n", config)

	//配置是否已经存在，存在返回异常
	_, ok = configmap[config.RuleName]
	if ok {
		logimp.Printf("配置信息 %s 已存在,不能重复添加!", config.RuleName)
		ResponseERRORFormat(w, "配置信息 %s 已存在,不能重复添加!", config.RuleName)
		return
	}

	//保存配置信息
	rtnstat, err := cfg.SaveConfig(config)
	if rtnstat == false {
		logimp.Warn(mylog, "保存配置信息失败!%v", err)
		ResponseERRORFormat(w, "保存配置信息失败!%v", err)
		return
	}

	//加锁，对configmap进行处理，如果已经存在，则刷新；如果没有，则新增
	mxlock.Lock()
	configmap[config.RuleName] = &config
	mxlock.Unlock()

	logimp.Printf("%#v\n", configmap)

	ResponseOK(w, "ok")

}

//返回配置规则名称json数组
func listConfig(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	var confignames []string
	for key := range configmap {
		confignames = append(confignames, key)
	}
	ResponseOK(w, confignames)
}

//根据规则名，查询详细配置并返回
func queryConfig(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	var rulname string

	v, ok := params["rulename"]
	if ok && len(v[0]) > 0 {
		rulname = v[0]
	} else {
		ResponseERROR(w, "参数不合法!")
		return
	}

	vl, err := configmap[rulname]
	if err {
		ResponseOK(w, vl)
	} else {
		ResponseERRORFormat(w, "没有查询到相关信息%s", rulname)
	}

}

//查询所有详细配置并返回
func queryAllConfig(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	v, _ := params["min_key"]
	min_key := v[0]
	v, _ = params["max_key"]
	max_key := v[0]
	v, _ = params["up_down"]
	up_down := v[0]

	fmt.Printf("min=%s,max=%s,ud=%s\n", min_key, max_key, up_down)

	//取出map中的所有key存入切片keys
	var keys_con = make([]string, 0, 200)
	for k, v := range configmap {
		nk := v.DateTime + "|" + k
		keys_con = append(keys_con, nk)
	}

	//对切片进行排序
	sort.Strings(keys_con)                  //[x1,x2,x3,...] 排序后，为正向；
	var keys_con_2 = make([]string, 0, 200) //keys_con_2是逆向排序，由大到小
	for i := len(keys_con); i > 0; i-- {
		keys_con_2 = append(keys_con_2, keys_con[i-1])
	}

	startindex, endindex := tool.PageCompute(keys_con_2, min_key, max_key, up_down)

	var rsp = Rsp{
		RspCode:    cfg.OK,
		RspMsg:     configmap,
		RspExtMsg:  keys_con_2[startindex : endindex+1],
		PageUp:     startindex > 0,
		PageDown:   endindex < len(keys_con_2)-1,
		TotalCount: len(keys_con_2),
		StartIndex: startindex,
		EndIndex:   endindex,
	}

	ResponseOKWithStruct(w, rsp)
}

//删除配置，一是map，而是文件
func deleteConfig(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	var rulname string

	v, ok := params["rulename"]
	if ok && len(v[0]) > 0 {
		rulname = v[0]
	} else {
		ResponseERROR(w, "参数不合法!")
		return
	}

	a, e := cfg.DeleteConfig(configmap, rulname)
	if a {
		ResponseOK(w, "ok")
	} else {
		ResponseERROR(w, e)
	}

}

func goTest(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	//scheduler.RunTask("11","F:\\a\\4.zip")
	r.ParseForm()
	params := r.Form

	var rulename string

	v, ok := params["RuleName"]
	if ok && len(v[0]) > 0 {
		rulename = v[0]
	} else {
		ResponseERROR(w, "RuleName参数不合法!")
		return
	}

	var srcfileordirpath string

	v, ok = params["SrcFileOrDirPath"]
	if ok && len(v[0]) > 0 {
		srcfileordirpath = v[0]
	} else {
		ResponseERROR(w, "SrcFileOrDirPath参数不合法!")
		return
	}

	taskno, err := scheduler.CreateTask(rulename, srcfileordirpath)

	err = scheduler.RunTaskWithTaskNO(taskno)
	if err != nil {
		ResponseERRORFormat(w, "taskno=%s,error=%v", taskno, err)
	} else {
		ResponseOK(w, "ok! taskno="+taskno)
	}

}

//根据参数查询任务，日期必送，rulename、是否自动任务等条件选送
func queryalltask(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	var dt string
	var rulename string
	var automod string

	v, ok := params["dt"]
	if ok && len(v[0]) > 0 {
		dt = strings.ReplaceAll(v[0], "-", "")
	} else {
		ResponseERROR(w, "日期参数不合法!")
		return
	}

	v, ok = params["RuleName"]
	rulename = v[0]

	v, ok = params["AutoMod"]
	automod = v[0]
	v, _ = params["min_key"]
	min_key := v[0]
	v, _ = params["max_key"]
	max_key := v[0]
	v, _ = params["up_down"]
	up_down := v[0]

	ss, errss := scheduler.GetTaskList(dt)
	if errss != nil {
		ResponseERRORFormat(w, "%v", errss)
	}

	sort.Strings(ss)

	var tasks_ss = make([]string, 0)
	var tasks = make([]scheduler.Task, 0)
	for _, v := range ss {
		t, errt := scheduler.ReadTaskFile(v)
		if errt != nil {
			ResponseERRORFormat(w, "%v", errt)
			return
		}
		addflag := true

		if rulename != "" && t.RuleName != rulename {
			addflag = false
		}
		if automod != "all" && strconv.FormatBool(t.AutoMod) != automod {
			addflag = false
		}

		if addflag {
			tasks = append(tasks, t)
			tasks_ss = append(tasks_ss, v)
		}

	}

	startindex, endindex := tool.PageCompute(tasks_ss, min_key, max_key, up_down)

	var rsp = Rsp{
		RspCode:    cfg.OK,
		RspMsg:     tasks[startindex : endindex+1],
		RspExtMsg:  nil,
		PageUp:     startindex > 0,
		PageDown:   endindex < len(tasks)-1,
		TotalCount: len(tasks),
		StartIndex: startindex,
		EndIndex:   endindex,
	}

	ResponseOKWithStruct(w, rsp)

}

func queryonetask(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	var filename string

	v, ok := params["filename"]
	if ok && len(v[0]) > 0 {
		filename = v[0]
	} else {
		ResponseERROR(w, "参数不合法!")
		return
	}

	t, errt := scheduler.ReadTaskFile(filename)
	if errt != nil {
		ResponseERRORFormat(w, "%v", errt)
		return
	}

	ResponseOK(w, t)

}

func flushconfig(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)
	cm,err:=cfg.FlushConfig()

	if err != nil {
		ResponseERRORFormat(w, "%v", err)
		return
	}else{
		configmap=cm
		ResponseOK(w, "ok")
	}

}

//////////////////////////////////////////////////////////////////////////////////////////////////////////

//添加配置
func addcrontab(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	crontabmap:=crontab.GetCrontab()

	var cron crontab.CronTab

	v, ok := params["CrontabName"]
	if ok && len(v[0]) > 0 {
		cron.CrontabName = v[0]
	} else {
		ResponseERROR(w, "任务名称参数不合法!")
		return
	}

	v, ok = params["CrontabExp"]
	if ok && len(v[0]) > 0 {
		cron.CrontabExp = v[0]
	} else {
		ResponseERROR(w, "crontab表达式不合法!")
		return
	}

	v, ok = params["RuleName"]
	if ok && len(v[0]) > 0 {
		rn:=v[0]
		if _,b:=configmap[rn];b==false{
			ResponseERROR(w, "RuleName不存在!")
			return
		}
		cron.RuleName=v[0]
	} else {
		ResponseERROR(w, "RuleName参数不合法!")
		return
	}

	v, ok = params["SrcPath"]
	if ok && len(v[0]) > 0 {
		cron.SrcPath = v[0]
	} else {
		ResponseERROR(w, "SrcPath参数不合法!")
		return
	}

	v, ok = params["Describle"]
	if ok {
		cron.Describe = v[0]
	}

	cron.CreateTime = time.Now().Format("2006-01-02 15:04:05")

	logimp.Printf("%#v\n", cron)


	logimp.Printf("%#v\n", crontabmap)
	//配置是否已经存在，存在返回异常
	_, ok = crontabmap[cron.CrontabName]
	if ok {
		logimp.Printf("crontab配置信息 %s 已存在,不能重复添加!", cron.CrontabName)
		ResponseERRORFormat(w, "crontab配置信息 %s 已存在,不能重复添加!", cron.CrontabName)
		return
	}

	//保存配置信息
	rtnstat, err :=crontab.SaveCrontab(cron)
	if rtnstat == false {
		logimp.Warn(mylog, "保存crontab配置信息失败!%v", err)
		ResponseERRORFormat(w, "保存crontab配置信息失败!%v", err)
		return
	}

	//加锁，对crontabmap进行处理，如果已经存在，则刷新；如果没有，则新增
	mxlock.Lock()
	crontabmap[cron.CrontabName] = &cron
	mxlock.Unlock()

	logimp.Printf("%#v\n", crontabmap)

	ResponseOK(w, "ok")

}
func queryAllCrontab(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	v, _ := params["min_key"]
	min_key := v[0]
	v, _ = params["max_key"]
	max_key := v[0]
	v, _ = params["up_down"]
	up_down := v[0]

	fmt.Printf("min=%s,max=%s,ud=%s\n", min_key, max_key, up_down)

	crontabmap:=crontab.GetCrontab()

	//取出map中的所有key存入切片keys
	var keys_con = make([]string, 0, 200)
	for k, v := range crontabmap {
		nk := v.CreateTime + "|" + k
		keys_con = append(keys_con, nk)
	}

	//对切片进行排序
	sort.Strings(keys_con)                  //[x1,x2,x3,...] 排序后，为正向；
	var keys_con_2 = make([]string, 0, 200) //keys_con_2是逆向排序，由大到小
	for i := len(keys_con); i > 0; i-- {
		keys_con_2 = append(keys_con_2, keys_con[i-1])
	}

	startindex, endindex := tool.PageCompute(keys_con_2, min_key, max_key, up_down)

	var rsp = Rsp{
		RspCode:    cfg.OK,
		RspMsg:     crontabmap,
		RspExtMsg:  keys_con_2[startindex : endindex+1],
		PageUp:     startindex > 0,
		PageDown:   endindex < len(keys_con_2)-1,
		TotalCount: len(keys_con_2),
		StartIndex: startindex,
		EndIndex:   endindex,
	}

	ResponseOKWithStruct(w, rsp)
}


//删除配置，一是map，而是文件
func deletecrontab(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	var crontabname string

	v, ok := params["CrontabName"]
	if ok && len(v[0]) > 0 {
		crontabname = v[0]
	} else {
		ResponseERROR(w, "CrontabName参数不合法!")
		return
	}

	a, e := crontab.DeleteCrontab(crontab.GetCrontab(),crontabname)
	if a {
		crontab.RemoveCronInstance(crontabname)
		ResponseOK(w, "ok")
	} else {
		ResponseERROR(w, e)
	}

}


func changecrontabstat(w http.ResponseWriter, r *http.Request) {

	logimp.Printf("req path=%s\n", r.URL.Path)

	r.ParseForm()
	params := r.Form

	var Enable string
	var CrontabName string

	v, ok := params["Enable"]
	if ok && len(v[0]) > 0 {
		Enable = v[0]
	} else {
		ResponseERROR(w, "Stat参数不合法!")
		return
	}

	v, ok = params["CrontabName"]
	if ok && len(v[0]) > 0 {
		CrontabName = v[0]
	} else {
		ResponseERROR(w, "CrontabName参数不合法!")
		return
	}

	b,eb:=strconv.ParseBool(Enable)
	if eb!=nil{
		ResponseERROR(w, "Stat参数不合法!")
		return
	}

	crontabmap:=crontab.GetCrontab()
	_,ok=crontabmap[CrontabName]
	if ok==false{
		ResponseERROR(w, "自动任务不存在!"+CrontabName)
		return
	}

	crontabmap[CrontabName].Enable=b
	crontab.SaveCrontab(*crontabmap[CrontabName])

	if b{
		crontab.AddCronInstance(*crontabmap[CrontabName])
	}else{
		crontab.RemoveCronInstance(CrontabName)
	}

	ResponseOK(w, "ok")

}


///////////////////////////////////////////////////////////////////////////////////////////////////////////

func Response(w http.ResponseWriter, rsp Rsp) {
	data, _ := json.Marshal(rsp)
	fmt.Fprintf(w, "%s", data)

}

func ResponseOK(w http.ResponseWriter, rspmsg interface{}) {
	var rsp Rsp = Rsp{
		RspCode:   cfg.OK,
		RspMsg:    rspmsg,
		RspExtMsg: nil,
	}
	Response(w, rsp)
}

func ResponseOKWithStruct(w http.ResponseWriter, rsp Rsp) {
	Response(w, rsp)
}

func ResponseERROR(w http.ResponseWriter, rspmsg interface{}) {
	var rsp Rsp = Rsp{
		RspCode:   cfg.ERROR,
		RspMsg:    rspmsg,
		RspExtMsg: nil,
	}
	Response(w, rsp)
}

func ResponseERRORFormat(w http.ResponseWriter, format string, rspmsg ...interface{}) {
	var rsp Rsp = Rsp{
		RspCode:   cfg.ERROR,
		RspMsg:    fmt.Sprintf(format, rspmsg...),
		RspExtMsg: nil,
	}
	Response(w, rsp)
}
