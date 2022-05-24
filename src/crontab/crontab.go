package crontab

import (
	"encoding/json"
	"errors"
	"fttf/src/logimp"
	"fttf/src/scheduler"
	"github.com/robfig/cron/v3"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

type CronTab struct {
	CrontabName string
	CrontabExp  string
	RuleName	string
	SrcPath 	string
	Describe 	string
	CreateTime	string
	Enable		bool
}

func (this CronTab)Run(){
	logimp.Info(mylog,"自动任务[%s]开始本次运行,%v\n",this.CrontabName,this)
	logimp.Info(mylog,"自动任务[%s]开始本次运行，创建任务task",this.CrontabName)
	taskno, err := scheduler.CreateTask(this.RuleName, this.SrcPath)
	if err!=nil{
		logimp.Warn(mylog,"自动任务[%s]开始本次运行，创建task失败,err=%v\n",this.CrontabName,err)
	}else{
		scheduler.GetTaskMap()[taskno].AutoMod=true
		logimp.Info(mylog,"自动任务[%s]开始本次运行，TaskNo=%s\n",this.CrontabName,taskno)
	}




	err=scheduler.RunTaskWithTaskNO(taskno)
	if err!=nil {
		logimp.Warn(mylog,"自动任务[%s]开始本次运行，执行失败,err=%v\n",this.CrontabName,err)
	}else{
		logimp.Info(mylog,"自动任务[%s]本次运行完成\n",this.CrontabName)
	}
}

var crontabmap map[string]*CronTab
var mxlock sync.Mutex // 互斥锁
var mylog *log.Logger
var fw *os.File

var GCron *cron.Cron

var croninstancemap map[string]int

func Init(a *log.Logger) {
	mylog = a
	_, errs := os.Stat("crontabs")
	if os.IsNotExist(errs) {
		// 创建文件夹
		os.MkdirAll("crontabs", os.ModePerm)
	}

	croninstancemap= make(map[string]int, 20)
	GCron=cron.New()

}

func SaveCrontab(c CronTab) (ok bool, er error) {
	data, err := json.Marshal(c)
	if err != nil {
		logimp.Warn(mylog, "struct 序列化失败。%v", err)
		return false, err
	}

	mxlock.Lock()
	defer mxlock.Unlock()

	var filename = c.CrontabName
	fw, err := os.OpenFile("crontabs"+string(filepath.Separator)+filename+".cro", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer fw.Close()

	if err != nil {
		logimp.Warn(mylog, "创建定时任务配置文件%s失败。%v", filename, err)
		return false, err
	}
	fw.Write(data)

	return true, nil

}

func ReadAllCrontab() (map[string]*CronTab,error) {
	fs, err := ioutil.ReadDir("crontabs")
	if err != nil {
		logimp.Warn(mylog, "crontabs。%v", err)
		return nil,err
	}

	crontabmap= make(map[string]*CronTab, len(fs))

	for _, f := range fs {
		if f.IsDir() {
			continue
		}
		filename := f.Name()
		keyname := strings.ReplaceAll(filename, ".cro", "")
		rc := ReadOneCrontab(filename)
		if rc != nil {
			crontabmap[keyname] = rc
		}

	}

	return crontabmap,nil

}

func ReadOneCrontab(filename string) (c *CronTab) {
	//判断配置文件是否存在
	_, errs := os.Stat("crontabs" + string(filepath.Separator) + filename)
	if os.IsNotExist(errs) {
		return nil
	}

	data, err := ioutil.ReadFile("crontabs" + string(filepath.Separator) + filename)
	if err != nil {
		logimp.Warn(mylog, "读取crontab配置文件%s失败。%v", filename, err)
		return nil
	}

	c = &CronTab{}
	err = json.Unmarshal(data, c)
	if err != nil {
		logimp.Warn(mylog, "读取crontab配置文件%s成功，反序列化失败。%v", filename, err)
		return nil
	}
	if c.CrontabName!=strings.ReplaceAll(filename,".cro","") {
		logimp.Warn(mylog,"读取crontab配置文件%s成功，反序列化后的crontab和配置文件名(不包括扩展名)不一致,略过。",filename)
		return nil
	}
	logimp.Info(mylog, "读取crontab配置文件%s,反序列化成功", filename)

	return c

}

func DeleteCrontab(m map[string]*CronTab, cn string) (bool, error) {
	mxlock.Lock()
	defer mxlock.Unlock()

	e := os.Remove("crontabs" + string(filepath.Separator) + cn + ".cro")
	if e != nil {
		return false, e
	}

	_, err := m[cn]
	if err {
		delete(m, cn)
	}

	return true, nil
}


func GetCrontab()map[string]*CronTab{
	return crontabmap
}

func RunCrontab() {

	GCron.Start()

	select {
	}
}


var cronrun_param CronTab

func AddCronInstance(c CronTab) error{
	_,ok:=croninstancemap[c.CrontabName]
	if ok==true{ //已经存在
		return errors.New("cron instance exist!")
	}
	cronrun_param=c
	cid,errc:=GCron.AddJob(c.CrontabExp,c)
	if errc!=nil{
		return errc
	}
	croninstancemap[c.CrontabName]=(int)(cid)
	return nil
}

func RemoveCronInstance(cname string) error{
	cid,ok:=croninstancemap[cname]
	if ok==false{ //不存在,直接返回成功
		return nil
	}
	GCron.Remove(cron.EntryID(cid))
	return nil
}




