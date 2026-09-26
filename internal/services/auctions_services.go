package services

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type MessageKind int

const (
	PlaceBid MessageKind = iota

	SuccessfullyPlacedBid
	FailedToPlaceBid
	NewBidPlaced
	AuctionFinished
	InvalidJSON
)

type Message struct {
	Message string      `json:"message,omitempty"`
	Amount  int         `json:"amount,omitempty"`
	Kind    MessageKind `json:"kind"`
	UserID  uuid.UUID   `json:"user_id,omitempty`
}

type AuctionLobby struct {
	sync.Mutex
	Rooms map[uuid.UUID]*AuctionRoom
}

type AuctionRoom struct {
	Id         uuid.UUID
	Context    context.Context
	Broadcast  chan Message
	Register   chan *Client
	Unregister chan *Client
	Clients    map[uuid.UUID]*Client

	BidsService *BidsService
}

func (r *AuctionRoom) registerClient(c *Client) {
	slog.Info("New user Connected", "Client", c)
	r.Clients[c.UserId] = c
}

func (r *AuctionRoom) unregisterClient(c *Client) {
	slog.Info("User diconnected", "Client", c)
	delete(r.Clients, c.UserId)
}

func (r *AuctionRoom) broadcastMessage(m Message) {
	slog.Info("New message recieved", "RoomID", r.Id, "message", m.Message, "user_id", m.UserID)

	switch m.Kind {
	case PlaceBid:
		bid, err := r.BidsService.Placebid(r.Context, r.Id, m.UserID, int32(m.Amount))
		if err != nil {
			if errors.Is(err, ErrBidIsTooLow) {
				if client, ok := r.Clients[m.UserID]; ok {
					client.Send <- Message{Kind: FailedToPlaceBid, Message: ErrBidIsTooLow.Error()}
				}
			}
		}

		if client, ok := r.Clients[m.UserID]; ok {
			client.Send <- Message{Kind: SuccessfullyPlacedBid, Message: "Your bid was Successfully placed"}
		}

		for id, client := range r.Clients {
			newBidMessage := Message{Kind: NewBidPlaced, Message: "A new bid was placed", Amount: int(bid.BidAmountInCents)}
			if id == m.UserID {
				continue
			}
			client.Send <- newBidMessage
		}
	case InvalidJSON:
		client, ok := r.Clients[m.UserID]
		if !ok {
			slog.Info("Client not found in hashmap", "user_id", m.UserID)
		}

		client.Send <- m
	}
}

func (r *AuctionRoom) Run() {
	defer func() {
		close(r.Broadcast)
		close(r.Register)
		close(r.Unregister)
	}()

	for {
		select {
		case client := <-r.Register:
			r.registerClient(client)
		case client := <-r.Unregister:
			r.unregisterClient(client)
		case message := <-r.Broadcast:
			r.broadcastMessage(message)
		case <-r.Context.Done():
			slog.Info("Auction has ended.", "auctionID", r.Id)
			for _, client := range r.Clients {
				client.Send <- Message{Kind: AuctionFinished, Message: "auction has been finished"}
			}
			return
		}
	}
}

func NewAuctionRoom(ctx context.Context, id uuid.UUID, BidsService BidsService) *AuctionRoom {
	return &AuctionRoom{
		Id:          id,
		Broadcast:   make(chan Message),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Clients:     make(map[uuid.UUID]*Client),
		Context:     ctx,
		BidsService: &BidsService,
	}
}

type Client struct {
	Room   *AuctionRoom
	Conn   *websocket.Conn
	Send   chan Message
	UserId uuid.UUID
}

func NewClient(room *AuctionRoom, conn *websocket.Conn, userId uuid.UUID) *Client {
	return &Client{
		Room:   room,
		Conn:   conn,
		Send:   make(chan Message, 512),
		UserId: userId,
	}
}

const (
	maxMessageSize = 512
	readDeadLine   = 60 * time.Second
)

func (c *Client) ReadEventLoop() {
	defer func() {
		c.Room.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(readDeadLine))

	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(readDeadLine))
		return nil
	})

	for {
		var m Message
		m.UserID = c.UserId
		err := c.Conn.ReadJSON(&m)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("Unexpected Close error", "error", err)
				return
			}
			c.Room.Broadcast <- Message{
				Kind:    InvalidJSON,
				Message: "this message should be a valid json",
				UserID:  m.UserID,
			}
		}

		c.Room.Broadcast <- m
	}
}
