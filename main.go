package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)
//创建用户结构体类型
type Client struct {
	C chan string
	Name string
	Addr string
}

//创建全局map，存储在线用户
var onlineMap map[string]Client

//创建全局channel，传递用户消息
var message =make(chan string)

func WriteMsgToClient(clt Client, conn net.Conn)  {
	//监听用户自带Channel上是否有消息。
	for msg := range clt.C{
		conn.Write([]byte(msg+"\n"))
	}
}

func MakeMsg(clt Client,msg string) (buf string) {
	buf ="[" + clt.Addr + "]" +clt.Name + ":  " +msg
	return buf
}

func HandlerConnect(conn net.Conn)  {
	defer conn.Close()
	//创建一个channel，判断用户是否活跃。
	hasDate := make(chan bool)

	//获取用户网络地址
	netAddr := conn.RemoteAddr().String()
	//创建新连接用户,默认用户名是ip+port
	clt := Client{make(chan string),netAddr,netAddr}

	//将新连接用户添加到在线用户map中,key:ip+port value:client
	onlineMap[clt.Addr] = clt

	//创建专门用来给当前用户发信息的go程
	go WriteMsgToClient(clt,conn)

	//发送用户上线信息到全局channel中
	message <- MakeMsg(clt,"login~")

	//创建一个channel，用来判断用户是否退出
	isQuit := make(chan bool)

	//创建一个匿名go程，专门处理用户发送的消息
	go func() {
		buf := make([]byte ,4096)
		for  {
			n , err := conn.Read(buf)
			if n==0 {
				isQuit <- true
				fmt.Printf("检测到客户端:%s退出\n",clt.Name)
				return
			}
			if err != nil {
				fmt.Println("conn,Read err",err)
				return
			}
			msg := string(buf[:n-1])			//在命令窗口输入会加个\n，故n-1

			if msg == "who"&& len(msg)==3 {			//提取用户在线列表,who命令
				conn.Write([]byte("online user list:\n"))
				//遍历当前map，获取在线用户
				for _,user :=range onlineMap{
					userInfo := user.Addr+":"+user.Name+"\n"
					conn.Write([]byte(userInfo))
				}
			}else if len(msg) >=8 && msg[:6]=="rename" {	//改名，rename|命令
				newName := strings.Split(msg,"|")[1]	//msg[8:]
				clt.Name=newName							//修改结构体
				onlineMap[clt.Addr] =clt					//更新在线用户列表
				conn.Write([]byte("rename successfully\n"))

			} else {
				//将读到的用户消息广播给所有用户，写入到message中
				message <- MakeMsg(clt,msg)
			}
			hasDate <- true
		}
	}()
	//保证不退出
	for  {
		select {
			case <-isQuit:
				delete(onlineMap,clt.Addr)				//将用户从在线用户列表移除
				message <- MakeMsg(clt,"logout")	//写入用户退出消息到全局channel
				return //结束go程
			case <-hasDate:
				//什么都不做。目的是重置下面case计时器。
			case <-time.After(time.Second*300):			//超时强踢
				delete(onlineMap,clt.Addr)
				message <- MakeMsg(clt,"logout")
				return
		}
	}
}

func Manager()  {
	//初始化 onlineMap
	onlineMap = make(map[string]Client)

	//监听全局channel中是否有数据,有 数据存储到msg，无 数据阻塞
	for  {
		msg := <-message

		//循环发送信息给在线用户
		for _,clt := range onlineMap{
			clt.C <- msg
		}
	}

}

func main() {
	//创建监听套接字
	listener , err := net.Listen("tcp","127.0.0.1:8008")
	if err != nil {
		fmt.Println("listen err",err)
		return
	}
	defer listener.Close()

	//创建管理者go程
	go Manager()


	//循环监听客户端连接请求
	for  {
		conn , err := listener.Accept()
		if err != nil {
			fmt.Println("listener.Accept err",err)
			return
		}
		//启动go程处理客户端数据请求
		go HandlerConnect(conn)
	}
}
