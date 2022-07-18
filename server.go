package main

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	Ip   string
	Port int

	// 在线用户的Map
	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	// 消息广播的channel
	Message chan string
}

// NewServer 创建一个server的接口
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}
	return server
}

// ListenMessage 监听Message广播消息channel的goroutine, 一旦有消息就发送给全部在线的User
func (this *Server) ListenMessage() {
	for {
		msg := <-this.Message

		// 将msg发送给全部的在线User
		this.mapLock.Lock()
		for _, cli := range this.OnlineMap {
			cli.C <- msg
		}
		this.mapLock.Unlock()
	}
}

// BroadCast 广播消息
func (this *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	this.Message <- sendMsg
}

func (this *Server) Handler(conn net.Conn) {
	// 当前连接业务
	//fmt.Println("connecting success")
	user := NewUser(conn, this)

	user.Online()

	// 监听用户是否活跃的Channel
	isLive := make(chan bool)
	//接收客户端的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				user.Offline()
				return
			}
			if err != nil && err != io.EOF {
				fmt.Println("conn read err", err)
				return
			}
			// 提取用户消息
			msg := string(buf[:n-1])

			// 用户针对msg进行消息处理
			user.DoMessage(msg)

			// 用户的任意消息, 代表当前用户是一个活跃的
			isLive <- true
		}
	}()

	// 当前handler阻塞
	for {
		select {
		// 如果当前用户是活跃的, 应该重制定时器
		// 不用做任何事情，只为了激活select, 更新下面的定时器
		case <-isLive:
		case <-time.After(time.Second * 60 * 5):
			// 已经超时
			// 将当前的User强制关闭
			user.SendMsg("你被踢了")

			// 销毁资源
			close(user.C)

			// 关闭连接
			conn.Close()

			// 退出当前handler
			return
		}
	}
}

func (this *Server) Start() {
	// socket listen
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port))
	if err != nil {
		fmt.Println("net.Listen error", err)
		return
	}
	// close listen socket
	defer listener.Close()

	//启动监听Message的goroutine
	go this.ListenMessage()

	for {
		// accept
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener accept error", err)
			continue
		}

		// do handler
		go this.Handler(conn)
	}

}
