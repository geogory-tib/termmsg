package types

import (
	"bytes"
	"log"
	"net"
	"sync"
	"termsg/commands"
	"termsg/headers"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type Connection_manager struct {
	Users map[string]net.Conn
	Mutex sync.RWMutex
}

type Server struct {
	Active_Users Connection_manager
	Usernames    map[net.Addr]string
	Ip           string
	Port         string
	Listener     net.Listener
}

func (server *Server) Server_main() {
start:
	for {
		conn, err := server.Listener.Accept()
		if err != nil {
			log.Println(err)
		}
		_, err = ws.Upgrade(conn) // handshake is uneeded
		if err != nil {
			log.Println(err)
			goto start
		}
		server.check_if_connection_is_user(conn)
		go server.handle_client_connection(conn)
	}
}

func (server *Server) check_if_connection_is_user(conn net.Conn) {
	name, is_a_user := server.Usernames[conn.RemoteAddr()]
	if is_a_user {
		server.Active_Users.Mutex.Lock()
		defer server.Active_Users.Mutex.Unlock()
		server.Active_Users.Users[name] = conn
	} else {
		go server.create_user(conn)
	}
}

func (server *Server) create_user(conn net.Conn) {
	buffer := make([]byte, 0, 100)
	err := wsutil.WriteServerMessage(conn, ws.OpText, buffer)
	if err != nil {
		log.Println(err)
		return
	}
	response, err := wsutil.ReadClientText(conn)
	if err != nil {
		log.Println(err)
		return
	}
	server.Usernames[conn.RemoteAddr()] = string(response)
	server.Active_Users.Mutex.Lock()
	defer server.Active_Users.Mutex.Unlock()
	server.Active_Users.Users[string(response)] = conn
}
func (server *Server) handle_client_connection(conn net.Conn) {
	err := wsutil.WriteServerText(conn, []byte("Connected"))
	defer conn.Close()
	if err != nil {
		log.Println(err)
	}
	msg_buffer := make([]wsutil.Message, 0, 10)
	for {

		msg_buffer, err = wsutil.ReadClientMessage(conn, msg_buffer)
		if err != nil {
			log.Println("Error  reading from client on "+conn.RemoteAddr().String()+" Error: ", err)
			return
		}
		if msg_buffer[len(msg_buffer)-1].OpCode == ws.OpClose {
			username, _ := server.Usernames[conn.RemoteAddr()]
			server.Active_Users.Mutex.Lock()
			delete(server.Active_Users.Users, username)
			return
		}
		split_msg := bytes.Split(msg_buffer[len(msg_buffer)-1].Payload, []byte("\n\r"))

		if bytes.Equal(split_msg[0], []byte(headers.Command_Header)) {
			command := split_msg[1]
			server.handle_server_commands(command, conn)
		}
		if bytes.Equal(split_msg[0], []byte(headers.SEND_HEADER)) {
			server.handle_send_request(split_msg[1:], conn)
		}

	}
}
func (server *Server) handle_server_commands(command_buffer []byte, conn net.Conn) {
	command := string(command_buffer)
	switch command {
	case commands.SHOW_ACTIVE_USERS:
		users := make([]byte, 0, 25)
		users = append(users, []byte(headers.Command_Reply)...)
		server.Active_Users.Mutex.RLock()
		defer server.Active_Users.Mutex.RUnlock()
		for name := range server.Active_Users.Users {
			users = append(users, []byte(name)...)
		}
		err := wsutil.WriteServerText(conn, users)
		if err != nil {
			log.Println("Error sending command reply on "+conn.RemoteAddr().String()+" Error: ", err)
			return
		}
	}
}

func (server *Server) handle_send_request(msg_request [][]byte, conn net.Conn) {
	server.Active_Users.Mutex.RLock()
	defer server.Active_Users.Mutex.RUnlock()
	conn_to_send_to, user_is_acitive := server.Active_Users.Users[string(msg_request[0])]
	if user_is_acitive {
		message_to_send := make([]byte, 0, 500)
		sender_name := server.Usernames[conn.RemoteAddr()]
		message_to_send = append(message_to_send, []byte(headers.FROM_HEADER)...)
		message_to_send = append(message_to_send, []byte(sender_name+"\n\r")...)
		message_to_send = append(message_to_send, msg_request[1]...)
		err := wsutil.WriteServerText(conn_to_send_to, message_to_send)
		if err != nil {
			log.Println("Error writing to recpient", err)
			wsutil.WriteClientBinary(conn, []byte(headers.SEND_FAIL))
			return
		}
		wsutil.WriteClientText(conn, []byte(headers.SEND_SUCCESS))
	} else {
		reply_message := make([]byte, 0, 100)
		reply_message = append(reply_message, []byte(headers.USER_NOT_ONLINE)...)
		reply_message = append(reply_message, msg_request[0]...)
		wsutil.WriteServerText(conn, reply_message)
	}
}
