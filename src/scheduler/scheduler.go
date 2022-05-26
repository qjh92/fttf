package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"fttf/src/cfg"
	"fttf/src/logimp"
	"fttf/src/netcom"
	"fttf/src/tool"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	TASK_RUNNING = "running"
	TASK_FAILED  = "failed"
	TASK_OK      = "success"
	TASK_REDAY   = "ready"
)

type Task struct {
	TaskNo      string
	RuleName    string
	Stat        string //状态 TASK_RUNNING,TASK_FAILED,TASK_OK
	CReateTime  string
	StartTime   string
	StopTime    string
	AbsPath     string
	IsDirMod    bool   //是否文件夹模式
	AutoMod     bool   //是否自动任务方式
	CrontabName string //自动模式时，crontab配置名称
	FDCount     int    //文件和文件夹总数
	FDOKCount   int    //成功传输的数量
	ErrorMSG    string
	SubSeqno    []string //传输的流水号,可以根据流水号定位到传输日志
	SubFDpath   []string
}

var mylog *log.Logger
var fw *os.File
var configmap map[string]*cfg.Config
var taskmap map[string]*Task

func Init(b map[string]*cfg.Config) {
	mylog, fw = logimp.InitLog("scheduler" + string(filepath.Separator) + "scheduler.log")
	configmap = b
	taskmap = make(map[string]*Task)

	_, errs := os.Stat("tasks")
	if os.IsNotExist(errs) {
		// 创建文件夹
		os.MkdirAll("tasks", os.ModePerm)
	}
}

func GetTaskMap() map[string]*Task {
	return taskmap
}

func CreateTask(rulename string, path string) (taskno string, initerror error) {

	c, ok := configmap[rulename]
	if ok == false {
		logimp.Warn(mylog, "rulename [%s] not exist,error\n", rulename)
		return "", errors.New("rulename not exist,"+rulename)
	}

	tmppath := tool.FormatPath(path)
	if !filepath.IsAbs(tmppath) { //相对路径参数，转换为绝对路径
		if c.PGmod == "put" {
			tmppath = c.LocalPath + string(filepath.Separator) + tmppath
		} else {
			tmppath = c.RemotePath + string(filepath.Separator) + tmppath
		}
	} else { //绝对路径参数，判断在不在规则目录下
		if c.PGmod == "put" {
			if strings.Index(tmppath, c.LocalPath) != 0 {
				errms := fmt.Sprintf("file or dir path [%s] not under the config path[%s],rulename[%s],error\n", tmppath, c.LocalPath, rulename)
				logimp.Warn(mylog, errms)
				return "", errors.New(errms)
			}
		} else {
			if strings.Index(tmppath, c.RemotePath) != 0 {
				errms := fmt.Sprintf("file or dir path [%s] not under the config path[%s],rulename[%s],error\n", tmppath, c.RemotePath, rulename)
				logimp.Warn(mylog, errms)
				return "", errors.New(errms)
			}
		}
	}

	taskno = "T_" + tool.CreateSeqno()
	var task = Task{
		TaskNo:     taskno,
		RuleName:   rulename,
		Stat:       TASK_REDAY,
		CReateTime: tool.GetLongDT(),
		StartTime:  "",
		StopTime:   "",
		AbsPath:    tmppath,
		IsDirMod:   false,
		AutoMod:    false,
		FDCount:    0,
		FDOKCount:  0,
		ErrorMSG:   "",
		SubSeqno:   nil,
		SubFDpath:  nil,
	}

	_, errsf := SaveTaskFile(&task)
	if errsf != nil {
		return "", errsf
	}

	taskmap[taskno] = &task

	return taskno, nil
}

func SetTaskStopInfo(t *Task, errmsg string) {
	t.Stat = TASK_FAILED
	t.ErrorMSG = errmsg
	t.StopTime = tool.GetLongDT()

	SaveTaskFile(t)
	delete(taskmap, t.TaskNo)
}

func SetTaskStartInfo(t *Task, fdcount int) {
	t.Stat = TASK_RUNNING
	t.StartTime = tool.GetLongDT()
	t.FDCount = fdcount
	SaveTaskFile(t)
}

func SetTaskRunningInfo(t *Task, subseqno string, subfdpath string) {
	if t.SubSeqno == nil {
		t.SubSeqno = make([]string, 0)
	}
	if t.SubFDpath == nil {
		t.SubFDpath = make([]string, 0)
	}
	t.SubSeqno = append(t.SubSeqno, subseqno)
	t.SubFDpath = append(t.SubFDpath, subfdpath)
	t.FDOKCount++
	t.Stat = TASK_RUNNING
	SaveTaskFile(t)
}

func SetTaskCompleteOKInfo(t *Task) {
	t.StopTime = tool.GetLongDT()
	t.Stat = TASK_OK
	SaveTaskFile(t)
	delete(taskmap, t.TaskNo)
}

func SaveTaskFile(t *Task) (ok bool, er error) {
	data, err := json.Marshal(t)
	if err != nil {
		logimp.Warn(mylog, "struct 序列化失败。%v", err)
		return false, err
	}

	var filename = t.TaskNo
	fw, err := os.OpenFile("tasks"+string(filepath.Separator)+filename, os.O_WRONLY|os.O_CREATE, 0644)
	defer fw.Close()

	if err != nil {
		logimp.Warn(mylog, "创建Task文件%s失败。%v", filename, err)
		return false, err
	}
	fw.Write(data)

	return true, nil

}

func ReadTaskFile(filename string) (Task, error) {
	var t = Task{}
	//判断配置文件是否存在
	_, errs := os.Stat("tasks" + string(filepath.Separator) + filename)
	if os.IsNotExist(errs) {
		return t, errs
	}

	data, err := ioutil.ReadFile("tasks" + string(filepath.Separator) + filename)
	if err != nil {
		logimp.Warn(mylog, "读取Task文件%s失败。%v", filename, err)
		return t, err
	}

	err = json.Unmarshal(data, &t)
	if err != nil {
		logimp.Warn(mylog, "读取配置文件%s成功，反序列化失败。%v", filename, err)
		return t, err
	}

	return t, nil
}

func RunTaskWithTaskNO(taskno string) error {
	c, ok := taskmap[taskno]
	if ok == false {
		logimp.Warn(mylog, "taskno [%s] not exist\n", taskno)
		return errors.New(taskno + " not exist")
	}

	if c.Stat == TASK_OK {
		errmsg := fmt.Sprintf("taskno [%s] complete ok,can't redo\n", taskno)
		logimp.Warn(mylog, errmsg)
		return errors.New(errmsg)
	}

	if c.Stat == TASK_RUNNING {
		errmsg := fmt.Sprintf("taskno [%s] is RUNNING,can't restart\n", taskno)
		logimp.Warn(mylog, errmsg)
		return errors.New(errmsg)
	}

	if c.Stat == TASK_RUNNING {
		errmsg := fmt.Sprintf("taskno [%s] is RUNNING,can't restart\n", taskno)
		logimp.Warn(mylog, errmsg)
		return errors.New(errmsg)
	}

	return runTask(c)
}

//调用前要对参数中的路径进行合规性检查，如果不是abspath绝对路径，要先转换为绝对路径
func runTask(t *Task) error {

	logimp.Info(mylog, "开始执行Task任务,taskinfo=[%#v]\n", t)
	abspath := t.AbsPath
	rulename := t.RuleName

	c, ok := configmap[rulename]
	if ok == false {
		errmsg := fmt.Sprintf("rulename [%s] not exist\n", rulename)
		SetTaskStopInfo(t, errmsg)
		logimp.Warn(mylog, errmsg)
		return errors.New(errmsg)
	}

	if c.PGmod == "put" { //put模式时，abspath就是本地的路径
		logimp.Info(mylog, "当前请求为put模式\n")
		fd, fierr := os.Stat(abspath)
		if fierr != nil {
			errmsg := fmt.Sprintf("get os.stat error! abspath=%s,error=%v\n", abspath, fierr)
			SetTaskStopInfo(t, errmsg)
			logimp.Warn(mylog, errmsg)
			return fierr
		}

		if fd.IsDir() { //abspath是目录时候，遍历目录，循环传输
			list, _, errlist := tool.ListDir(abspath)
			if errlist != nil {
				return errlist
			}
			logimp.Info(mylog, "即将传输目录[%s]下的子目录和文件,数量[%d]\n", abspath, len(list))
			SetTaskStartInfo(t, len(list))
			t.IsDirMod = true
			for index, value := range list {
				logimp.Info(mylog, "开始[%d/%d]传输[%s]\n", index+1, len(list), value)
				if value == c.LocalPath {
					logimp.Info(mylog, "传输[%d/%d]略过[%s],文件路径等于配置的顶层目录，不需要传输\n", index+1, len(list), value)
					continue
				}
				sno, errto := TransOnePut(*c, value, c.LocalPath, c.RemotePath, c.OverWrite)
				if errto != nil {
					errmsg := fmt.Sprintf("传输[%d/%d]异常[%s],%v\n", index+1, len(list), value, errto)
					SetTaskStopInfo(t, errmsg)
					logimp.Info(mylog, errmsg)
					return errto
				}
				logimp.Info(mylog, "传输[%d/%d]成功[%s]\n", index+1, len(list), value)
				SetTaskRunningInfo(t, sno, value)
			}
			SetTaskCompleteOKInfo(t)
		} else { //传输文件
			SetTaskStartInfo(t, 1)
			sno, errto := TransOnePut(*c, abspath, c.LocalPath, c.RemotePath, c.OverWrite)
			SetTaskRunningInfo(t, sno, abspath)
			SetTaskCompleteOKInfo(t)
			return errto
		}

	} else { //get模式时，abspath是远程主机的源文件路径
		logimp.Info(mylog, "当前请求为get模式\n")
		logimp.Info(mylog, "查询远程主机上[%s]的文件及目录信息\n", abspath)
		SetTaskStartInfo(t, -1)
		sno, isdir, list, fderr := QueryRemote(*c, abspath, c.RemotePath, c.LocalPath, c.OverWrite)
		if fderr != nil {
			errmsg := fmt.Sprintf("远程主机返回异常结果 error=%#v\n", fderr)
			SetTaskStopInfo(t, errmsg)
			logimp.Warn(mylog, errmsg)
			return fderr
		}
		SetTaskRunningInfo(t, sno, abspath)
		if isdir == false {
			logimp.Info(mylog, "远程主机返回结果，这是一个文件[%s]，开始传输\n", abspath)
			sno, errtog := TransOneGut(*c, abspath, c.RemotePath, c.LocalPath, c.OverWrite)
			if errtog != nil {
				errmsg := fmt.Sprintf("传输异常 error=%#v\n", errtog)
				SetTaskStopInfo(t, errmsg)
				logimp.Warn(mylog, errmsg)
				return errtog
			}
			logimp.Info(mylog, "传输完成\n")
			SetTaskRunningInfo(t, sno, abspath)
			SetTaskCompleteOKInfo(t)
			return nil
		} else {
			logimp.Info(mylog, "远程主机返回结果，这是一个目录,数量[%d]，开始传输\n", len(list))
			t.IsDirMod = true
			for index, value := range list {
				logimp.Info(mylog, "开始[%d/%d]传输[%s]\n", index+1, len(list), value)
				if value == c.RemotePath {
					logimp.Info(mylog, "传输[%d/%d]略过[%s],文件路径等于配置的顶层目录，不需要传输\n", index+1, len(list), value)
					continue
				}
				sno, errto := TransOneGut(*c, value, c.RemotePath, c.LocalPath, c.OverWrite)
				if errto != nil {
					errmsg := fmt.Sprintf("传输[%d/%d]异常[%s],%v\n", index+1, len(list), value, errto)
					SetTaskStopInfo(t, errmsg)
					logimp.Info(mylog, errmsg)
					return errto
				}
				SetTaskRunningInfo(t, sno, value)
				logimp.Info(mylog, "传输[%d/%d]成功[%s]\n", index+1, len(list), value)
			}
			SetTaskCompleteOKInfo(t)
		}
	}

	return nil
}

func TransOnePut(c cfg.Config, abspath string, srcpath string, destconfigpath string, overwrite bool) (sn string, err error) {
	seqno := tool.CreateSeqno()
	clog, cf := logimp.InitLog("client" + string(filepath.Separator) + "c_" + seqno + ".log")
	defer cf.Close()

	ip, iperr := tool.HostToIP(c.RemoteIP)
	if iperr != nil {
		logimp.Warn(clog, "query host[%s] ip error %v\n", c.RemoteIP, iperr)
		return seqno, iperr
	}
	logimp.Info(clog, "query host[%s] ip[%s]\n", c.RemoteIP, ip)
	addr := fmt.Sprintf("%s:%s", ip, c.RemotePort)
	logimp.Info(clog, "建立连接 connect to %s\n", addr)
	conn, r, w, errconn := netcom.CreateConn(mylog, addr)
	if errconn != nil {
		return seqno, errconn
	}
	defer conn.Close()

	logimp.Info(clog, "建立连接并发送数据\n")
	return seqno, netcom.TransReq(clog, seqno, abspath, srcpath, destconfigpath, overwrite, conn, r, w)
}

func QueryRemote(c cfg.Config, abspath string, srcpath string, destconfigpath string, overwrite bool) (sno string, isdir bool, fdlist []string, fderr error) {
	seqno := tool.CreateSeqno()
	clog, cf := logimp.InitLog("client" + string(filepath.Separator) + "c_" + seqno + ".log")
	defer cf.Close()

	ip, iperr := tool.HostToIP(c.RemoteIP)
	if iperr != nil {
		logimp.Warn(clog, "query host[%s] ip error %v\n", c.RemoteIP, iperr)
		return seqno, false, nil, iperr
	}
	logimp.Info(clog, "query host[%s] ip[%s]\n", c.RemoteIP, ip)
	addr := fmt.Sprintf("%s:%s", ip, c.RemotePort)
	logimp.Info(clog, "建立连接 connect to %s\n", addr)
	conn, r, w, errconn := netcom.CreateConn(mylog, addr)
	if errconn != nil {
		return seqno, false, nil, errconn
	}
	defer conn.Close()

	logimp.Info(clog, "建立连接并发送数据\n")
	dir, s, fderr := netcom.TransQueryFileDir(clog, c, seqno, abspath, conn, r, w)
	if fderr != nil {
		logimp.Warn(clog, "resp frame error=%#v\n", fderr)
		return seqno, false, nil, fderr
	}
	return seqno, dir, s, nil
}

func TransOneGut(c cfg.Config, abspath string, srcpath string, destconfigpath string, overwrite bool) (sno string, err error) {
	seqno := tool.CreateSeqno()
	clog, cf := logimp.InitLog("client" + string(filepath.Separator) + "c_" + seqno + ".log")
	defer cf.Close()

	ip, iperr := tool.HostToIP(c.RemoteIP)
	if iperr != nil {
		logimp.Warn(clog, "query host[%s] ip error %v\n", c.RemoteIP, iperr)
		return seqno, iperr
	}
	logimp.Info(clog, "query host[%s] ip[%s]\n", c.RemoteIP, ip)
	addr := fmt.Sprintf("%s:%s", ip, c.RemotePort)
	logimp.Info(clog, "建立连接 connect to %s\n", addr)
	conn, r, w, errconn := netcom.CreateConn(mylog, addr)
	if errconn != nil {
		return seqno, errconn
	}
	defer conn.Close()

	logimp.Info(clog, "建立连接并发送数据\n")

	return seqno, netcom.TransReqBegin(clog, c, seqno, abspath, conn, r, w)
}

func GetTaskList(dt string) ([]string, error) {
	dirpath := "tasks"
	fs, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	substr := "T_" + dt + "_"
	list := make([]string, 0)
	for _, f := range fs {
		filename := f.Name()
		if strings.Contains(filename, substr) {
			list = append(list, filename)
		}
	}
	return list, nil
}
