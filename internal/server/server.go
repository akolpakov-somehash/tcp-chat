package server

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type Server struct {
	host, port  string
	clients     map[string]*Client // map usernames to clients
	mu          sync.RWMutex       // protect clients map
	broadcastCh chan Message       // channel for broadcasting
	joinCh      chan net.Conn      // channel for new connections
	leaveCh     chan net.Conn      // channel for disconnections
}

type Message struct {
	sender    string
	content   string
	timestamp time.Time
}

type Client struct {
	conn     net.Conn
	username string
	server   *Server
}

type Config struct {
	Host, Port string
}

func NewServer(config *Config) *Server {
	return &Server{
		host:    config.Host,
		port:    config.Port,
		clients: make(map[string]*Client),
	}
}

func (s *Server) Run(ctx context.Context) {
	log.Println("Starting server...")
	defer log.Println("Server closed.")

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", s.host, s.port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Server started: %s:%s", s.host, s.port)
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("New connection.")
		client := &Client{
			conn:   conn,
			server: s,
		}
		go client.HandleRequest()
	}
}

func (s *Server) broadcast(msg Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		if client.username != msg.sender {
			client.send(fmt.Sprintf("[%s] %s: %s",
				msg.timestamp.Format("15:04:05"),
				msg.sender,
				msg.content))
		}
	}
}

func (s *Server) addClient(client *Client) error {
	s.mu.Lock()

	if _, ok := s.clients[client.username]; ok {
		return fmt.Errorf("user with username %s already there", client.username)
	}
	s.clients[client.username] = client
	s.mu.Unlock()
	s.broadcast(Message{
		sender:    "Server",
		content:   fmt.Sprintf("%s joined the chat", client.username),
		timestamp: time.Now(),
	})
	return nil
}

func (s *Server) removeClient(client *Client) {
	s.mu.Lock()
	delete(s.clients, client.username)
	s.mu.Unlock()
	s.broadcast(Message{
		sender:    "Server",
		content:   fmt.Sprintf("%s left the chat", client.username),
		timestamp: time.Now(),
	})
}

///////Client

func (c *Client) HandleRequest() error {
	defer c.conn.Close()
	defer c.server.removeClient(c)

	// Get username
	username, err := c.getUserName()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}
	c.username = username
	log.Printf("%s joined", username)

	// Add client to server
	if err := c.server.addClient(c); err != nil {
		return fmt.Errorf("failed to add client: %w", err)
	}
	log.Printf("%s added", username)

	// Welcome message
	if err := c.send(fmt.Sprintf("Welcome %s! Type /help for commands", username)); err != nil {
		return err
	}

	// Message loop
	reader := bufio.NewReader(c.conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}

		if strings.HasPrefix(message, "/") {
			if err := c.handleCommand(message[1:]); err != nil {
				return err
			}
			continue
		}

		// Broadcast message
		c.server.broadcast(Message{
			sender:    c.username,
			content:   message,
			timestamp: time.Now(),
		})
	}
}

func (c *Client) getUserName() (string, error) {
	c.send("Please provide a username:")
	reader := bufio.NewReader(c.conn)
	name, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("name could not be empty")
	}
	return name, nil

}

func (c *Client) send(msg string) error {
	_, err := c.conn.Write([]byte(msg + "\n"))
	log.Printf("message %s sent to %s", msg, c.username)

	return err
}

func (c *Client) disconnect() error {
	err := c.send("Bye!")
	if err != nil {
		return err
	}
	err = c.conn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) listUsers() error {

	for u := range c.server.clients {
		err := c.send(u)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) handleCommand(cmd string) error {
	helpMessage := "TBD"
	switch strings.ToLower(cmd) {
	case "bye":
		return c.disconnect()
	case "help":
		return c.send(helpMessage)
	case "users":
		return c.listUsers()
	default:
		return nil
	}
}
