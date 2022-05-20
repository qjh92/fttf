package cfg

import (
	"encoding/json"
	"fttf/src/logimp"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var mylog *log.Logger

var mxlock sync.Mutex // 互斥锁

type Config struct {
	RuleName   string //规则名称，全局唯一
	PGmod      string //"put" or "get"
	LocalPath  string //本地路径
	RemoteIP   string //远端ip
	RemotePort string //远端端口
	RemotePath string //远端路径
	OverWrite  bool   //是否可以覆盖
	Describle  string //描述信息
	DateTime   string //日期串 yyyyMMddHHmmss

}

func Init(lg *log.Logger) {
	mylog = lg
	_, errs := os.Stat("configs")
	if os.IsNotExist(errs) {
		// 创建文件夹
		os.MkdirAll("configs", os.ModePerm)
	}
}

func SaveConfig(c Config) (ok bool, er error) {
	data, err := json.Marshal(c)
	if err != nil {
		logimp.Warn(mylog, "struct 序列化失败。%v", err)
		return false, err
	}

	mxlock.Lock()
	defer mxlock.Unlock()

	var filename = c.RuleName
	fw, err := os.OpenFile("configs"+string(filepath.Separator)+filename+".ftc", os.O_WRONLY|os.O_CREATE, 0644)
	defer fw.Close()

	if err != nil {
		logimp.Warn(mylog, "创建配置文件%s失败。%v", filename, err)
		return false, err
	}
	fw.Write(data)

	return true, nil

}

func ReadAllConfig() map[string]*Config {
	fs, err := ioutil.ReadDir("configs")
	if err != nil {
		logimp.Warn(mylog, "读取configs目录失败。%v", err)
		return nil
	}

	configmap := make(map[string]*Config, len(fs))

	for _, f := range fs {
		if f.IsDir() {
			continue
		}
		filename := f.Name()
		keyname := strings.ReplaceAll(filename, ".ftc", "")
		rc := ReadOneConfig(filename)
		if rc != nil {
			configmap[keyname] = rc
		}

	}

	logimp.Info(mylog, "%v", configmap)
	return configmap

}

func ReadOneConfig(filename string) (c *Config) {
	//判断配置文件是否存在
	_, errs := os.Stat("configs" + string(filepath.Separator) + filename)
	if os.IsNotExist(errs) {
		return nil
	}

	data, err := ioutil.ReadFile("configs" + string(filepath.Separator) + filename)
	if err != nil {
		logimp.Warn(mylog, "读取配置文件%s失败。%v", filename, err)
		return nil
	}

	c = &Config{}
	err = json.Unmarshal(data, c)
	if err != nil {
		logimp.Warn(mylog, "读取配置文件%s成功，反序列化失败。%v", filename, err)
		return nil
	}
	//if c.RuleName!=strings.ReplaceAll(filename,".ftc","") {
	//	logimp.Warn(mylog,"读取配置文件%s成功，反序列化后的RuleName和配置文件名(不包括扩展名)不一致,略过。",filename)
	//	return nil
	//}
	logimp.Info(mylog, "读取配置文件%s,反序列化成功", filename)

	return c

}

func DeleteConfig(m map[string]*Config, rn string) (bool, error) {
	mxlock.Lock()
	defer mxlock.Unlock()

	e := os.Remove("configs" + string(filepath.Separator) + rn + ".ftc")
	if e != nil {
		return false, e
	}

	_, err := m[rn]
	if err {
		delete(m, rn)
	}

	return true, nil
}
