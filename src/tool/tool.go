package tool

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//对路径进行格式化，目标是返回去掉路径最后面的分隔符的新格式
func FormatPath(p string) string {
	if len(p) > 1 {
		for p[len(p)-1:len(p)] == "/" || p[len(p)-1:len(p)] == "\\" || p[0:1] == "." {
			p = strings.TrimRight(p, "\\")
			p = strings.TrimRight(p, "/")
			p = strings.TrimLeft(p, ".")
			if len(p) < 1 {
				break
			}
		}
	}
	return p
}

func MyFunc() {
	/*s :="abc"
	s2 :="def"*/
	r, e := rand.Int(rand.Reader, big.NewInt(999))
	if e == nil {
		fmt.Print(r)
	}

}

func CreateSeqno() string {
	s := time.Now().Format("20060102_150405")
	s2 := CreateUUID()
	return fmt.Sprintf("%s_%s", s, s2)
}

func CreateUUID() string {
	// V4 基于随机数
	u4 := uuid.New()
	str := u4.String()                      // a0d99f20-1dd1-459b-b516-dfeca4005203
	return strings.ReplaceAll(str, "-", "") // a0d99f201dd1459bb516dfeca4005203
}

func StrMd5(str string) (retMd5 string) {
	h := md5.New()
	h.Write([]byte(str))
	return strings.ToLower(hex.EncodeToString(h.Sum(nil)))
}
func FileMD5(file string) string {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return ""
	}
	return StrMd5(string(data))
}

func PathProc(frelpath string) {
	s, e := filepath.Rel("c:\\c\\", "c:\\a\\b")
	fmt.Printf("%s,%v\n", s, e)
}

func PathSeparatorConvert(oldpath string) string {
	if filepath.Separator == '\\' { //当前os是windows，把odlpath中的‘/’换掉
		return strings.ReplaceAll(oldpath, "/", "\\")
	}

	if filepath.Separator == '/' { //当前os是linux，把odlpath中的‘\’换掉
		return strings.ReplaceAll(oldpath, "\\", "/")
	}

	return ""
}

func PathExist(abspath string) bool {
	_, errs := os.Stat(abspath)
	if errs != nil {
		return false
	}
	if os.IsNotExist(errs) {
		return false
	}
	return true
}

func HostToIP(hostname string) (string, error) {
	addr, err := net.ResolveIPAddr("ip", hostname) //域名解析ip，如果是ip就不在解析
	if err != nil {
		return hostname, err
	}
	return addr.String(), nil
}

//ListDir递归列出所有文件，包括子文件夹，同时子文件夹数量不能大于10000，否则程序资源开销太大，返回太慢
var gListdirDircount = 0

func ListDir(dirpath string) (list []string, dircount int, err error) {
	gListdirDircount = 0
	list, errlist := subListDir(dirpath)
	return list, gListdirDircount, errlist
}

func ListDirWithString(dirpath string) (l string, dircount int, err error) {
	var str string
	list, c, errlist := ListDir(dirpath)
	if errlist == nil {

		for _, l := range list {
			str += l + "|"
		}
		return strings.TrimRight(str, "|"), c, nil
	}

	return "", c, errlist
}

func subListDir(dirpath string) ([]string, error) {
	fs, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}

	list := make([]string, 0, len(fs))
	list = append(list, dirpath)

	for _, f := range fs {
		filename := f.Name()
		if f.IsDir() {
			gListdirDircount++
			if gListdirDircount > 10000 {
				return nil, errors.New("sub directory count > 10000,abort return")
			}
			listsub, listsuberr := subListDir(dirpath + string(filepath.Separator) + filename)
			if listsuberr != nil {
				return nil, listsuberr
			}
			if listsub != nil {
				list = append(list, listsub...)

			}

		} else {
			list = append(list, dirpath+string(filepath.Separator)+filename)
		}
	}
	return list, nil
}

func TempTest() {
	s := time.Now().Format("2006-01-02 15:04:05")
	fmt.Println(s)
}

func GetLongDT() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func GetShortDT() string {
	return time.Now().Format("20060102150405")
}

func PageCompute(ss []string, min_key string, max_key string, up_down string) (sindex int, eindex int) {
	var index = 0
	var startindex = 0
	var endindex = 9

	if up_down == "down" {
		if min_key != "" {
			for index = 0; index < len(ss); index++ {
				if ss[index] == min_key {
					if index == len(ss)-1 {
						endindex = index
						startindex = endindex - 9
					} else {
						startindex = index + 1
						endindex = startindex + 9
					}
					break
				}
			}
		}
	} else {
		if max_key != "" {
			for index = 0; index < len(ss); index++ {
				if ss[index] == max_key {
					if index == 0 {
						startindex = 0
						endindex = startindex + 9
					} else {
						endindex = index - 1
						startindex = endindex - 9
					}

					break
				}
			}
		}
	}

	if endindex >= len(ss) {
		endindex = len(ss) - 1
	}

	if startindex < 0 {
		startindex = 0
	}

	return startindex, endindex
}
