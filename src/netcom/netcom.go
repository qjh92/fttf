package netcom

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"fttf/src/cfg"
	"fttf/src/logimp"
	"fttf/src/tool"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//command 枚举
const (
	TRANS_BEGIN = "TRANS_BEGIN" //开始传输
	TRANS_ING   = "TRANS_ING"   //传输中
	TRANS_END   = "TRANS_END"   //传输结束

	TRANS_REQ_BEGIN = "TRANS_REQ_BEGIN" //请求对方首先开始Begin

	TRANS_Query_FILEDIR = "TRANS_Query_FILEDIR" //请求对方的路径对应的是文件还是文件夹，如果是文件夹，服务端调用tool.ListDir,将结果转换为竖线分割的字符串，
	//存入data中，返回
)

var READTIMEOUT = 20 * time.Second
var CONNECTTIMEOUT = 5 * time.Second

type Frame struct {
	Seqno          string      //流水号，全局唯一
	Command        string      //指令
	RspCode        int         //响应码
	RspMsg         interface{} //响应信息
	OverWrite      bool
	AbsPath        string //源文件的绝对路径 /home/a/b.txt,在TRANS_REQ_BEGIN\TRANS_Query_FILEDIR指令中需要填充
	DestConfigPath string //目标端，规则中配置的路径,put规则中为remotepath，get规则中为localpath
	FileMD5        string //文件的md5
	Path           string //源文件相对于源配置路径的相对路径，例如源文件配置路径是/home,发送文件路径是/home/a/b.txt,
	// 计算后的path为a/b.txt,与DestConfigPath拼接以后就是目标系统的存放绝对路径
	IsDir       bool //是否文件夹
	DataFrameID int  //数据帧序号，发送数据报文的时候，每次加1
	Data        []byte
}

func ReadPkgLen(conn net.Conn, slog *log.Logger) (int, error) {

	conn.SetReadDeadline(time.Now().Add(READTIMEOUT))

	buf, err := ReadByte(conn, 7) // 读取7位长度信息
	if err != nil {
		if err == io.EOF {
			logimp.Info(slog, "client close!\n")
			return 0, err
		}
		return -1, err
	}
	str := string(buf[:])
	framelen, ferr := strconv.Atoi(str) //转换为长度信息
	if ferr != nil {
		logimp.Warn(slog, "convert to int32 head length failed, %v:", err)
		return -1, ferr
	}
	return framelen, nil
}

func ReadPkg(conn net.Conn, pkglen int, slog *log.Logger) (Frame, error) {
	var f = Frame{}
	conn.SetReadDeadline(time.Now().Add(READTIMEOUT))

	buf, err := ReadByte(conn, pkglen)
	if err != nil {
		if err == io.EOF {
			logimp.Info(slog, "client close!\n")
			return f, err
		}
		return f, err
	}

	f, err = ByteToStruct(buf)
	if err != nil {
		logimp.Warn(slog, "convert byte to struct error,frame=%#v,error=%#v\n", f, err)
	}
	return f, err
}

func ReadByte(reader io.Reader, len int) ([]byte, error) {
	var err error
	var buf = make([]byte, len)

	if _, err = io.ReadFull(reader, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func StructToByte(f Frame) ([]byte, error) {
	//JSON序列化：结构体-->JSON格式的字符串
	data, err := json.Marshal(f)
	return data, err
}

func ByteToStruct(b []byte) (Frame, error) {
	f := &Frame{}
	err := json.Unmarshal([]byte(b), f)
	return *f, err
}

func CreateConn(lg *log.Logger, addr string) (net.Conn, *bufio.Reader, *bufio.Writer, error) {
	conn, err := net.DialTimeout("tcp", addr, CONNECTTIMEOUT) //创建套接字,连接服务器,设置超时时间
	if err != nil {
		logimp.Warn(lg, "connect error! addr=%s,error=%v\n", addr, err)
		return nil, nil, nil, err
	}

	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)
	return conn, r, w, nil
}

func SendFrame(lg *log.Logger, f Frame, conn net.Conn, w *bufio.Writer) error {

	b, errb := StructToByte(f)
	if errb != nil {
		logimp.Warn(lg, "StructToByte error!frame=%v,error=%v\n", f, errb)
		return errb
	}

	blen := fmt.Sprintf("%07d", len(b))
	_, err := w.Write([]byte(blen))

	if err != nil {
		logimp.Warn(lg, "send pkg head error!%v\n", err)
		return err
	}
	_, err = w.Write(b)

	if err != nil {
		logimp.Warn(lg, "send pkg body error!%v\n", err)
		return err
	}

	err = w.Flush()
	if err != nil {
		logimp.Warn(lg, "send flush error!%v\n", err)
		return err
	}

	return nil
}

func RecvFrame(lg *log.Logger, conn net.Conn, r *bufio.Reader) (Frame, error) {
	var rspframe Frame
	bt, erpl := ReadPkgLen(conn, lg)
	if bt < 0 {
		logimp.Warn(lg, "read pkg head error %#v,%#v\n", bt, erpl)
		return rspframe, erpl
	}

	if erpl == io.EOF {
		return rspframe, erpl
	}

	rspframe, rsperr := ReadPkg(conn, bt, lg)
	if rsperr != nil {
		logimp.Warn(lg, "read pkg body error %#v,%#v\n", rspframe, rsperr)
		return rspframe, erpl
	}

	return rspframe, nil
}

func TransReq(lg *log.Logger, seqno string, abspath string, srcpath string, destconfigpath string, overwrite bool, conn net.Conn, r *bufio.Reader, w *bufio.Writer) error {

	logimp.Info(lg, "get fire or dir stat,path=[%s]\n", abspath)
	fw, errs := os.Stat(abspath)
	if errs != nil {
		logimp.Warn(lg, "file stat error,fileallpath=%s,%v\n", abspath, errs)
		return errs
	}
	if os.IsNotExist(errs) {
		logimp.Warn(lg, "file stat error,fileallpath=%s,%v\n", abspath, errs)
		return errs
	}

	logimp.Info(lg, "get relpath from abspath[%s] and srcpath[%s]\n", abspath, srcpath)
	relpath, re := filepath.Rel(srcpath, abspath)
	if re != nil {
		logimp.Warn(lg, "get rel path error!,srcpath=%s,abspath=%s,error=%v\n", srcpath, abspath, re)
		return re
	}
	logimp.Info(lg, "relpath=[%s]\n", relpath)

	//发送传输请求begin
	var frame = Frame{
		Seqno:          seqno,
		Command:        TRANS_BEGIN,
		RspCode:        0,
		RspMsg:         nil,
		DestConfigPath: destconfigpath,
		FileMD5:        "",
		OverWrite:      overwrite,
		Path:           relpath,
		IsDir:          fw.IsDir(),
		DataFrameID:    0,
		Data:           nil,
	}

	if !fw.IsDir() {
		frame.FileMD5 = tool.FileMD5(abspath)
		logimp.Info(lg, "get file md5=%s\n", frame.FileMD5)
	}

	logimp.Info(lg, "send req frame=[%#v]\n", frame)
	errsend := SendFrame(lg, frame, conn, w)
	if errsend != nil {
		return errsend
	}
	rspframe, rsperr := RecvFrame(lg, conn, r)
	if rsperr != nil {
		return rsperr
	}
	logimp.Info(lg, "recv rsp frame=[%#v]\n", rspframe)

	//处理返回的rspframe
	if rspframe.Command == TRANS_END { //结束传输
		if rspframe.RspCode == cfg.ERROR { //有错误码
			logimp.Warn(lg, "recv Command=TRANS_END and RspCode=ERROR,%v\n", rspframe.RspMsg)
			return errors.New(fmt.Sprintf("%s", rspframe.RspMsg))
		} else { //本次传输完成
			logimp.Info(lg, "%s\n", rspframe.RspMsg)
			return nil
		}
	}

	if rspframe.Command == TRANS_BEGIN {
		frame.Command = TRANS_ING
		logimp.Info(lg, "recv Command=TRANS_BEGIN and set req command TRANS_ING,begin send data\n")

		fr, errfr := os.Open(abspath)
		if errfr != nil {
			logimp.Warn(lg, "open file error,trans over,error=%v\n", errfr)
			return errfr
		}
		defer fr.Close()
		logimp.Info(lg, "open file[%s] ok,ready to read\n", abspath)
		var tmp = make([]byte, 1024*1024*5)
		var rdnum = 0
		for {
			rdnum++
			logimp.Info(lg, "file ready to read num[%d]\n", rdnum)
			n, err := fr.Read(tmp)
			if err == io.EOF {
				logimp.Info(lg, "end of file,num[%d]\n", rdnum)
				frame.Data = nil
				frame.Command = TRANS_END
				err = SendFrame(lg, frame, conn, w)
				if err != nil {
					logimp.Warn(lg, "read file complete,send TRANS_END error,trans over,error=%v\n", err)
					return err
				}
				logimp.Info(lg, "read file complete,send TRANS_END ok,recv rspframe\n")
				rspendframe, errrspend := RecvFrame(lg, conn, r)
				if errrspend != nil { //接收终止传输异常
					logimp.Warn(lg, "recv rspframe error,%v\n", errrspend)
					return errrspend
				} else {
					if rspendframe.RspCode != cfg.OK { //接收正常，内部有错误码
						logimp.Warn(lg, "rspframe rspcode=error,%#v\n", rspendframe)
						return errors.New(fmt.Sprintf("%s", rspendframe.RspMsg))
					}
				}

				logimp.Info(lg, "file read all complete,send TRANS_END ok!!!,recv rspframe ok,trans over;%s\n", rspendframe.RspMsg)
				return nil
			}
			if err != nil {
				logimp.Warn(lg, "file read num[%d] error,trans over,error=%v\n", rdnum, err)
				return err
			}
			frame.Data = tmp[:n]
			frame.DataFrameID += 1
			err = SendFrame(lg, frame, conn, w)

			if err != nil {
				logimp.Warn(lg, "send read num[%d] data error,trans over,error=%v\n", rdnum, err)
				return err
			}

			logimp.Info(lg, "read file num[%d] and send dataframid[%d] ok\n", rdnum, frame.DataFrameID)

		}

	}

	return nil
}

func TransProcReqRsp(lg *log.Logger, conn net.Conn, r *bufio.Reader, w *bufio.Writer, newlog bool) error {
	var tslog *log.Logger
	var tsf *os.File
	var wfile *os.File
	var werror error
	var seqno = ""
	var path string

	defer wfile.Close()

	for {
		reqframe, rsperr := RecvFrame(lg, conn, r)
		if rsperr != nil {
			if rsperr == io.EOF { //client closed
				return nil
			}
			return rsperr
		}

		if seqno == "" {
			seqno = reqframe.Seqno
			if len(seqno) < 1 {
				logimp.Warn(lg, "seqno='' error!\n")
				return errors.New("seqno='',error")
			}

			if newlog {
				tslog, tsf = logimp.InitLog("server" + string(filepath.Separator) + "s_" + seqno + ".log")
				defer tsf.Close()
				lg = tslog
				logimp.Info(tslog, "recv frame=%#v\n", reqframe)
			} else {
				tslog = lg
			}
		}

		var rspframe = Frame{
			Seqno:     reqframe.Seqno,
			Command:   reqframe.Command,
			IsDir:     reqframe.IsDir,
			OverWrite: reqframe.OverWrite,
			RspCode:   cfg.OK,
		}

		//开始传输指令，如果是文件夹，判断文件夹是否存在，没有则创建
		//如果是文件，判断文件是否存在，md5是否一致，如果不一致，则判断是否可以覆盖，可以覆盖则删除旧文件
		if reqframe.Command == TRANS_BEGIN {
			logimp.Info(tslog, "proc TRANS_BEGIN\n")

			if !tool.PathExist(reqframe.DestConfigPath) {
				logimp.Warn(tslog, "rspframe.DestConfigPath[%s] not exist!\n", reqframe.DestConfigPath)
				return errors.New("rspframe DestConfigPath not exist!")
			}

			path = fmt.Sprintf("%s%c%s", reqframe.DestConfigPath, filepath.Separator, tool.PathSeparatorConvert(reqframe.Path))
			logimp.Info(tslog, "destconfigpath=[%s],relpath=[%s],dest abs path=%s\n", reqframe.DestConfigPath, reqframe.Path, path)

			if reqframe.IsDir { //文件夹
				if !tool.PathExist(path) {
					errmk := os.MkdirAll(path, os.ModePerm)
					if errmk != nil {
						rspframe.RspCode = cfg.ERROR
						rspframe.RspMsg = errmk
						logimp.Warn(tslog, "MkDirAll error,rsp TRANS_END and cfg.ERROR,STOP\n")
					}
					rspframe.Command = TRANS_END
					rspframe.RspMsg = "create dir ok,rsp TRANS_END and cfg.OK,direct over"
					logimp.Info(tslog, "%s\n", rspframe.RspMsg)
				} else { //文件夹已经存在
					rspframe.Command = TRANS_END
					rspframe.RspMsg = "dir exist,no need create,direct over"
					logimp.Info(tslog, "dir exist,no need create,rsp TRANS_END and cfg.OK,direct over\n")
				}
			} else { //文件
				if tool.PathExist(path) { //存在
					logimp.Info(tslog, "file exist,computer file md5\n")
					oldfilemd5 := tool.FileMD5(path)
					if oldfilemd5 == reqframe.FileMD5 { //文件比对一致，结束传输
						logimp.Info(tslog, "file md5 same [%s]\n", oldfilemd5)
						rspframe.Command = TRANS_END
						rspframe.RspMsg = "dest file exist and md5 same,rsp TRANS_END and cfg.OK,direct over"
						logimp.Info(tslog, "%s\n", rspframe.RspMsg)
					} else { //文件存在但是比对不一致
						logimp.Info(tslog, "file md5 not same oldfile md5[%s],new file md5[%s]\n", oldfilemd5, reqframe.FileMD5)
						if reqframe.OverWrite { //可以覆盖，则可以删除旧文件
							if rmerr := os.RemoveAll(path); rmerr != nil {
								rspframe.Command = TRANS_END
								rspframe.RspCode = cfg.ERROR
								rspframe.RspMsg = fmt.Sprintf("dest file exist,md5 not same,overwrite true;delete old file error[%v]", rmerr)
								logimp.Warn(tslog, "%s\n", rspframe.RspMsg)
							} else {
								logimp.Info(tslog, "dest file exist,md5 not same,overwrite true;delete old file ok,rsp TRANS_BEGIN and cfg.OK\n")
							}

						} else {
							rspframe.Command = TRANS_END
							rspframe.RspCode = cfg.ERROR
							rspframe.RspMsg = fmt.Sprintf("dest file[%s] exist,md5 not same,overwrite false,rsp TRANS_END and cfg.ERROR\n")
							logimp.Warn(tslog, "dest file exist,md5 not same,overwrite false,rsp TRANS_END and cfg.ERROR\n")
						}
					}
				} else { //不存在
					logimp.Info(tslog, "dest file not exist;rsp TRANS_BEGIN and cfg.OK,can send data\n")
				}
			}
			SendFrame(tslog, rspframe, conn, w)
		}
		if reqframe.Command == TRANS_ING {
			if wfile == nil { //没有创建文件，则开始创建
				os.MkdirAll(filepath.Dir(path), os.ModePerm) //保证目标文件的相对目录文件夹是存在的
				wfile, werror = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
				if werror != nil {
					rspframe.Command = TRANS_END
					rspframe.RspCode = cfg.ERROR
					rspframe.RspMsg = fmt.Sprintf("create and open file[%s] error,%v\n", path, werror)
					logimp.Warn(tslog, "%s\n", rspframe.RspMsg)
					SendFrame(tslog, rspframe, conn, w)
					return werror
				} else {
					logimp.Info(tslog, "dest file not exist;create new file ok\n")
				}
			}

			logimp.Info(tslog, "recv file data ok,ready to save,dataframid[%d]\n", reqframe.DataFrameID)
			wn, we := wfile.Write(reqframe.Data)
			if we != nil {
				logimp.Warn(tslog, "save data error,dataframid[%d],error=[%v]\n", reqframe.DataFrameID, we)
			}
			logimp.Info(tslog, "save data ok,dataframid[%d],date lenth=%d\n", reqframe.DataFrameID, wn)
		}

		if reqframe.Command == TRANS_END {
			wfile.Close()
			destmd5 := tool.FileMD5(path)
			if destmd5 != reqframe.FileMD5 {
				rspframe.RspCode = cfg.ERROR
				rspframe.RspMsg = fmt.Sprintf("file md5 not same [src md5=%s,dest md5=%s]", reqframe.FileMD5, destmd5)
				SendFrame(tslog, rspframe, conn, w)
				logimp.Info(tslog, "%s\n", rspframe.RspMsg)
				return errors.New(fmt.Sprintf("%s", rspframe.RspMsg))
			} else {
				rspframe.RspCode = cfg.OK
				rspframe.RspMsg = fmt.Sprintf("src md5[%s] == dest md5[%s]", reqframe.FileMD5, destmd5)
				SendFrame(tslog, rspframe, conn, w)
				logimp.Info(tslog, "recv TRANS_END and check file md5 same,trans ok!!!;src md5[%s] == dest md5[%s]", reqframe.FileMD5, destmd5)
				return nil
			}

		}

		if reqframe.Command == TRANS_REQ_BEGIN {
			c := &cfg.Config{}
			ceror := json.Unmarshal(reqframe.Data, c)
			if ceror != nil {
				rspframe.RspCode = cfg.ERROR
				rspframe.RspMsg = fmt.Sprintf("%v", ceror)
				SendFrame(tslog, rspframe, conn, w)
				logimp.Info(tslog, "%s\n", rspframe.RspMsg)
				return ceror
			}

			return TransReq(tslog, seqno, reqframe.AbsPath, c.RemotePath, c.LocalPath, c.OverWrite, conn, r, w)

		}

		if reqframe.Command == TRANS_Query_FILEDIR {
			fd, fierr := os.Stat(reqframe.AbsPath)
			if fierr != nil {
				rspframe.RspCode = cfg.ERROR
				rspframe.RspMsg = fmt.Sprintf("%v", fierr)
				SendFrame(tslog, rspframe, conn, w)
				logimp.Warn(tslog, "get os.stat error! abspath=%s,error=%v\n", reqframe.AbsPath, fierr)
				return fierr
			}
			if fd.IsDir() {
				rspframe.IsDir = true
				l, _, lderr := tool.ListDirWithString(reqframe.AbsPath)
				if lderr != nil {
					rspframe.RspCode = cfg.ERROR
					rspframe.RspMsg = fmt.Sprintf("%v", lderr)
					SendFrame(tslog, rspframe, conn, w)
					logimp.Warn(tslog, "ListDir error! abspath=%s,error=%v\n", reqframe.AbsPath, lderr)
					return lderr
				}

				rspframe.Data = []byte(l)
			} else {
				rspframe.IsDir = false
			}
			SendFrame(tslog, rspframe, conn, w)
			logimp.Info(lg, "send frame=%#v\n", rspframe)
		}
	}

	return nil
}

func TransReqBegin(lg *log.Logger, c cfg.Config, seqno string, abspath string, conn net.Conn, r *bufio.Reader, w *bufio.Writer) error {

	cdata, cdataerr := json.Marshal(c)
	if cdataerr != nil {
		logimp.Warn(lg, "config convert to byte error,%v\n", cdataerr)
		return cdataerr
	}

	//发送TRANS_REQ_BEGIN=3 请求对方首先开始Begin
	var frame = Frame{
		Seqno:          seqno,
		Command:        TRANS_REQ_BEGIN,
		RspCode:        0,
		RspMsg:         nil,
		OverWrite:      false,
		AbsPath:        abspath,
		DestConfigPath: "",
		FileMD5:        "",
		Path:           "",
		IsDir:          false,
		DataFrameID:    0,
		Data:           cdata,
	}

	logimp.Info(lg, "send req frame=[%#v]\n", frame)
	errsend := SendFrame(lg, frame, conn, w)
	if errsend != nil {
		return errsend
	}

	err := TransProcReqRsp(lg, conn, r, w, false)

	return err

}

func TransQueryFileDir(lg *log.Logger, c cfg.Config, seqno string, abspath string, conn net.Conn, r *bufio.Reader, w *bufio.Writer) (isdir bool, fdlist []string, fderr error) {

	cdata, cdataerr := json.Marshal(c)
	if cdataerr != nil {
		logimp.Warn(lg, "config convert to byte error,%v\n", cdataerr)
		return false, nil, cdataerr
	}

	//发送TRANS_REQ_BEGIN=4 获取abspath对应的文件目录列表
	var frame = Frame{
		Seqno:          seqno,
		Command:        TRANS_Query_FILEDIR,
		RspCode:        0,
		RspMsg:         nil,
		OverWrite:      false,
		AbsPath:        abspath,
		DestConfigPath: "",
		FileMD5:        "",
		Path:           "",
		IsDir:          false,
		DataFrameID:    0,
		Data:           cdata,
	}

	logimp.Info(lg, "send req frame=[%#v]\n", frame)
	errsend := SendFrame(lg, frame, conn, w)
	if errsend != nil {
		return false, nil, errsend
	}

	rspf, errrspf := RecvFrame(lg, conn, r)
	if errrspf != nil {
		logimp.Warn(lg, "recv frame error[%#v]\n", errrspf)
		return false, nil, errrspf
	}

	logimp.Info(lg, "recv frame=%#v\n", rspf)
	if rspf.RspCode != cfg.OK {
		errmsg := fmt.Sprintf("%#v", rspf.RspMsg)
		logimp.Warn(lg, "recv frame RspCode error[%s]\n", errmsg)
		return false, nil, errors.New(errmsg)
	}

	if rspf.IsDir {
		return true, strings.Split(string(rspf.Data), "|"), nil
	} else {
		return false, nil, nil
	}

}
