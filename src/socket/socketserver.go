package socket

import (
	"bufio"
	"fttf/src/cfg"
	"fttf/src/logimp"
	"fttf/src/netcom"
	"log"
	"net"
	"strconv"
)

var mainlog *log.Logger

var configmap map[string]*cfg.Config

func Init(a *log.Logger, b map[string]*cfg.Config) {
	mainlog = a
	configmap = b

}

func RunSocketServer(sport int) {

	logimp.Info(mainlog, "启动socketserver......sport=%d\n", sport)

	listen, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(sport))
	if err != nil {
		logimp.ErrorQuit(mainlog, "启动socketserver失败,sport=%d,error=%v \n", sport, err)
		return
	}

	for {
		conn, err := listen.Accept() // 建立连接
		if err != nil {
			logimp.Warn(mainlog, "accept failed, %v:", err)
			continue
		}

		logimp.Info(mainlog, "accept connect %s \n", conn.RemoteAddr().String())
		go process(conn) // 启动一个goroutine处理连接

	}
}

// 处理函数
func process(conn net.Conn) {
	defer conn.Close() // 关闭连接
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	netcom.TransProcReqRsp(mainlog, conn, r, w, true)

}
